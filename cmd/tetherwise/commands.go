package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/manthan-parmar-1998/tetherwise/internal/check"
	"github.com/manthan-parmar-1998/tetherwise/internal/metrics/cloudwatch"
	"github.com/manthan-parmar-1998/tetherwise/internal/model"
	"github.com/manthan-parmar-1998/tetherwise/internal/report"
	"github.com/manthan-parmar-1998/tetherwise/internal/safety"
	k8ssource "github.com/manthan-parmar-1998/tetherwise/internal/source/kubernetes"
)

func (a *app) newGraphCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Build the dependency graph",
		Long: `Build a dependency graph from Terraform state and plan, Kubernetes config,
and an optional hand-written edges file.

Write a versioned snapshot JSON document to -o, or to stdout.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.openAudit(); err != nil {
				return err
			}
			defer a.closeAudit()
			tfStates, _ := cmd.Flags().GetStringArray("tf-state")
			tfPlans, _ := cmd.Flags().GetStringArray("tf-plan")
			k8sPaths, _ := cmd.Flags().GetStringArray("k8s")
			edgesPath, _ := cmd.Flags().GetString("edges")
			live, _ := cmd.Flags().GetBool("k8s-live")
			outPath, _ := cmd.Flags().GetString("output")
			if len(tfStates) == 0 && len(tfPlans) == 0 && len(k8sPaths) == 0 && edgesPath == "" && !live {
				return errors.New("provide at least one of --tf-state, --tf-plan, --k8s, --edges, or --k8s-live")
			}
			if err := a.prove(cmd, live, false); err != nil {
				return err
			}
			g, _, _, err := a.loadGraph(cmd.Context(), tfStates, tfPlans, k8sPaths, edgesPath, "", live)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if outPath != "" {
				f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // G304: operator-selected output path
				if err != nil {
					return fmt.Errorf("open snapshot: %w", err)
				}
				defer func() { _ = f.Close() }()
				w = f
			}
			if err := g.WriteJSON(w); err != nil {
				return err
			}
			return nil
		},
	}
	addSourceFlags(cmd)
	cmd.Flags().StringP("output", "o", "", "write the snapshot to this path instead of stdout")
	return cmd
}

func (a *app) newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check PLAN",
		Short: "Report exposure for a change",
		Long: `Report who else is exposed by a Terraform plan, with a confidence level
per dependency and shared-resource headroom.

PLAN is a terraform show -json plan. Pass --snapshot to reuse a graph, or
the same source flags as "tetherwise graph" to build one. --metrics is
"cloudwatch" or "fixture:<path>".`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.openAudit(); err != nil {
				return err
			}
			defer a.closeAudit()
			planPath := args[0]
			tfStates, _ := cmd.Flags().GetStringArray("tf-state")
			tfPlans, _ := cmd.Flags().GetStringArray("tf-plan")
			k8sPaths, _ := cmd.Flags().GetStringArray("k8s")
			edgesPath, _ := cmd.Flags().GetString("edges")
			snapshot, _ := cmd.Flags().GetString("snapshot")
			live, _ := cmd.Flags().GetBool("k8s-live")
			metricsMode, _ := cmd.Flags().GetString("metrics")
			format, _ := cmd.Flags().GetString("format")
			outPath, _ := cmd.Flags().GetString("output")
			failOn, _ := cmd.Flags().GetString("fail-on")
			if failOn != "" && failOn != "high" {
				return errors.New(`--fail-on must be "high"`)
			}
			switch format {
			case "markdown", "json":
			default:
				return errors.New(`--format must be markdown or json`)
			}
			wantAWS := metricsMode == "cloudwatch"
			if err := a.prove(cmd, live, wantAWS); err != nil {
				return err
			}
			plans := append([]string{planPath}, tfPlans...)
			g, changes, sources, err := a.loadGraph(cmd.Context(), tfStates, plans, k8sPaths, edgesPath, snapshot, live)
			if err != nil {
				return err
			}
			if !contains(sources, "terraform") {
				sources = append(sources, "terraform")
			}
			limits, metricSource, err := a.readMetrics(cmd.Context(), metricsMode, g.Resources())
			if err != nil {
				return err
			}
			if metricSource != "" {
				sources = append(sources, metricSource)
			}
			result, err := check.Run(g, dedupeChanges(changes), limits, sources, check.Options{})
			if err != nil {
				return err
			}
			var buf []byte
			if format == "json" {
				buf, err = report.JSON(result)
			} else {
				buf = []byte(report.Markdown(result))
			}
			if err != nil {
				return err
			}
			if outPath == "" {
				if _, err := cmd.OutOrStdout().Write(buf); err != nil {
					return err
				}
			} else if err := os.WriteFile(outPath, buf, 0o600); err != nil {
				return fmt.Errorf("write report: %w", err)
			}
			if failOn == "high" && result.Verdict == check.VerdictHigh {
				a.exitCode = 2
			}
			return nil
		},
	}
	addSourceFlags(cmd)
	cmd.Flags().String("snapshot", "", "graph snapshot to load instead of rebuilding from sources")
	cmd.Flags().String("metrics", "", "cloudwatch or fixture:<path>")
	cmd.Flags().String("format", "markdown", "markdown or json")
	cmd.Flags().StringP("output", "o", "", "write the report to this path instead of stdout")
	cmd.Flags().String("fail-on", "", "exit 2 when the verdict is high-exposure (value: high)")
	return cmd
}

func addSourceFlags(cmd *cobra.Command) {
	cmd.Flags().StringArray("tf-state", nil, "terraform show -json state file (repeatable)")
	cmd.Flags().StringArray("tf-plan", nil, "terraform show -json plan file (repeatable)")
	cmd.Flags().StringArray("k8s", nil, "kubernetes manifest file or directory (repeatable)")
	cmd.Flags().String("edges", "", "hand-declared edges file")
	cmd.Flags().Bool("k8s-live", false, "list workloads from a cluster with get and list only")
	cmd.Flags().String("kubeconfig", "", "kubeconfig used with --k8s-live")
	cmd.Flags().String("k8s-context", "", "kube context used with --k8s-live")
}

func (a *app) prove(cmd *cobra.Command, live, wantAWS bool) error {
	ctx, cancel := context.WithTimeout(commandContext(cmd), 20*time.Second)
	defer cancel()
	opt := safety.Options{IKnow: a.v.GetBool("i-know"), Audit: a.audit}
	if wantAWS || safety.CredentialsConfigured() {
		identity, simulator, err := safety.AWSClients(ctx)
		if err != nil {
			if opt.IKnow {
				warn(cmd.ErrOrStderr(), "warning: AWS client setup failed; continuing because --i-know was set")
			} else {
				return fmt.Errorf("refusing to run: %w", err)
			}
		} else {
			opt.Identity = identity
			opt.Simulator = simulator
		}
	}
	if live {
		kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
		contextName, _ := cmd.Flags().GetString("k8s-context")
		client, err := k8ssource.NewClient(kubeconfig, contextName)
		if err != nil {
			return err
		}
		a.kube = client
		opt.Reviewer = safety.KubeReviewer{Client: client}
	}
	proof, err := safety.Prove(ctx, opt)
	for _, warning := range proof.Warnings {
		warn(cmd.ErrOrStderr(), "warning: %s", warning)
	}
	return err
}

func (a *app) readMetrics(ctx context.Context, mode string, resources []model.Resource) ([]model.Limit, string, error) {
	if mode == "" {
		return nil, "", nil
	}
	ctx = contextOrBackground(ctx)
	switch {
	case mode == "cloudwatch":
		client, err := cloudwatch.NewFromConfig(ctx, a.thresholds(), a.audit)
		if err != nil {
			return nil, "", err
		}
		limits, err := client.Headroom(ctx, resources)
		return limits, "cloudwatch", err
	case strings.HasPrefix(mode, "fixture:"):
		path := strings.TrimPrefix(mode, "fixture:")
		if path == "" {
			return nil, "", errors.New("fixture path is empty")
		}
		fixture, err := cloudwatch.LoadFixture(path, a.thresholds())
		if err != nil {
			return nil, "", err
		}
		limits, err := fixture.Headroom(ctx, resources)
		return limits, "fixture", err
	default:
		return nil, "", fmt.Errorf("unknown --metrics %q (want cloudwatch or fixture:<path>)", mode)
	}
}

func commandContext(cmd *cobra.Command) context.Context {
	if cmd != nil && cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func warn(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format+"\n", args...)
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func dedupeChanges(in []model.Change) []model.Change {
	seen := map[string]struct{}{}
	var out []model.Change
	for _, change := range in {
		key := change.Resource + "\x00" + string(change.Action) + "\x00" + strings.Join(change.ChangedAttrs, ",")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, change)
	}
	model.SortChanges(out)
	return out
}

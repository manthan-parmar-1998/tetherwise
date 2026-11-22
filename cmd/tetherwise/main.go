// Command tetherwise is a read-only, pre-merge CLI.
// It answers "who else breaks?" for an infrastructure change.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/manthan-parmar-1998/tetherwise/internal/audit"
	"github.com/manthan-parmar-1998/tetherwise/internal/metrics/cloudwatch"
	"k8s.io/client-go/kubernetes"
)

// version, commit, and date are injected at link time:
//
//	-X main.version= -X main.commit= -X main.date=
//
// Local builds set them from the Makefile. Goreleaser sets them on release.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type app struct {
	v        *viper.Viper
	audit    *audit.Logger
	kube     kubernetes.Interface
	exitCode int
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	a := newApp()
	cmd := a.command()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return 1
	}
	return a.exitCode
}

func newApp() *app {
	return &app{v: viper.New()}
}

func (a *app) command() *cobra.Command {
	var cfgFile string
	root := &cobra.Command{
		Use:   "tetherwise",
		Short: "Read-only pre-merge check for who else breaks",
		Long: `tetherwise answers "who else breaks?" before an infrastructure change merges.

It builds a dependency graph from Terraform state and plan, Kubernetes config,
and an optional hand-written edges file, then reports exposure and how close
shared resources are to their limits. It never mutates cloud resources.`,
		SilenceUsage: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			return loadConfig(a.v, cfgFile)
		},
	}
	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ./tetherwise.yaml or $HOME/.config/tetherwise/tetherwise.yaml)")
	root.PersistentFlags().Bool("i-know", false, "continue even when the caller is allowed to mutate; for disposable environments only")
	root.PersistentFlags().String("audit", "", "append-only JSONL audit log for external reads")
	_ = a.v.BindPFlag("i-know", root.PersistentFlags().Lookup("i-know"))
	_ = a.v.BindPFlag("audit", root.PersistentFlags().Lookup("audit"))
	root.CompletionOptions.HiddenDefaultCmd = true

	root.AddCommand(a.newGraphCmd(), a.newCheckCmd(), newVersionCmd())
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "version",
		Short:        "Print build version",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "tetherwise %s (commit %s, built %s)\n", version, commit, date)
			return err
		},
	}
}

func (a *app) openAudit() error {
	if a.audit != nil {
		return nil
	}
	path := a.v.GetString("audit")
	if path == "" {
		return nil
	}
	logger, err := audit.Open(path, true)
	if err != nil {
		return err
	}
	a.audit = logger
	return nil
}

func (a *app) closeAudit() {
	if a.audit == nil {
		return
	}
	_ = a.audit.Close()
	a.audit = nil
}

func (a *app) thresholds() cloudwatch.Thresholds {
	var t cloudwatch.Thresholds
	_ = a.v.UnmarshalKey("thresholds", &t)
	return t.WithDefaults()
}

// loadConfig reads an optional YAML file and the environment.
// Environment variables use the TETHERWISE_ prefix.
func loadConfig(v *viper.Viper, cfgFile string) error {
	v.SetEnvPrefix("TETHERWISE")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	v.AutomaticEnv()

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("tetherwise")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".config", "tetherwise"))
		}
	}

	if err := v.ReadInConfig(); err != nil {
		if cfgFile == "" {
			var missing viper.ConfigFileNotFoundError
			if errors.As(err, &missing) {
				return nil
			}
		}
		return fmt.Errorf("read config: %w", err)
	}
	return nil
}

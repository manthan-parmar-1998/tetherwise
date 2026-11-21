// Command tetherwise is a read-only, pre-merge CLI.
// It will answer "who else breaks?" for an infrastructure change.
// Building the graph, running checks, reading metrics, and writing the
// audit log are not implemented yet.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

// errNotImplemented is returned by commands whose behavior is not written yet.
var errNotImplemented = errors.New("not implemented yet")

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	v := viper.New()
	var cfgFile string

	root := &cobra.Command{
		Use:   "tetherwise",
		Short: "Read-only pre-merge check for who else breaks",
		Long: `tetherwise answers "who else breaks?" before an infrastructure change merges.

It will build a dependency graph from Terraform state and plan, Kubernetes
config, and an optional hand-written edges file, then report exposure and
how close shared resources are to their limits. It never mutates cloud resources.

Graph building, checks, the safety gate, metrics, and the audit log are not
implemented yet.`,
		SilenceUsage: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			return loadConfig(v, cfgFile)
		},
	}
	root.PersistentFlags().StringVar(
		&cfgFile,
		"config",
		"",
		"config file (default ./tetherwise.yaml or $HOME/.config/tetherwise/tetherwise.yaml)",
	)
	root.CompletionOptions.HiddenDefaultCmd = true

	root.AddCommand(
		newPendingCmd(
			"graph",
			"Build the dependency graph",
			`Build a dependency graph from Terraform state and plan, Kubernetes config,
and an optional hand-written edges file.

Not implemented yet.`,
		),
		newPendingCmd(
			"check",
			"Report exposure for a change",
			`Report who else is exposed by a change, with a confidence level per
dependency and shared-resource headroom from CloudWatch.

Not implemented yet.`,
		),
		newVersionCmd(),
	)
	return root
}

func newPendingCmd(use, short, long string) *cobra.Command {
	return &cobra.Command{
		Use:          use,
		Short:        short,
		Long:         long,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(*cobra.Command, []string) error {
			return errNotImplemented
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "version",
		Short:        "Print build version",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"tetherwise %s (commit %s, built %s)\n",
				version,
				commit,
				date,
			)
			return err
		},
	}
}

// loadConfig reads an optional YAML file and the environment.
// Keys are not interpreted yet; Manthan will define them with the commands.
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

package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/hypertrial/intentci/internal/version"
)

// Execute runs the root command using process args/stdio.
func Execute() error {
	return ExecuteWith(os.Args[1:], os.Stdout, os.Stderr)
}

// ExecuteWith runs the root command with explicit args and writers.
func ExecuteWith(args []string, stdout, stderr io.Writer) error {
	root := newRoot()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	return root.Execute()
}

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "intentci",
		Short:         "CI for product intent",
		Long:          "IntentCI verifies code changes against approved product requirements.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(newInitCmd())
	root.AddCommand(newValidateCmd())
	root.AddCommand(newCheckCmd())
	root.AddCommand(newVerifyCmd())
	root.AddCommand(newChangeCmd())
	root.AddCommand(newExplainCmd())
	root.AddCommand(newHookCmd())
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print IntentCI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.String())
		},
	})
	return root
}

// Main is the process entry used by cmd/intentci.
func Main() int {
	return RunMain(os.Args[1:], os.Stdout, os.Stderr)
}

// RunMain executes IntentCI and returns an exit code.
func RunMain(args []string, stdout, stderr io.Writer) int {
	err := ExecuteWith(args, stdout, stderr)
	if err == nil {
		return 0
	}
	code := CodeOf(err)
	if msg := err.Error(); msg != "" {
		fmt.Fprintln(stderr, msg)
	}
	return code
}

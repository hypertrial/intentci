package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hypertrial/intentci/internal/version"
)

// Execute runs the root command.
func Execute() error {
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
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print IntentCI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.String())
		},
	})
	return root.Execute()
}

// Main is the process entry used by cmd/intentci.
func Main() int {
	err := Execute()
	if err == nil {
		return 0
	}
	code := CodeOf(err)
	if msg := err.Error(); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
	}
	return code
}

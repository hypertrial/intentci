package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/hooks"
)

func newHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage the IntentCI Git pre-push hook",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Install or refresh the managed pre-push hook",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := getwd()
			if err != nil {
				return err
			}
			root, err := config.FindRoot(cwd)
			if err != nil {
				// Hooks install at git root even without .intentci; fall back to cwd.
				root = cwd
			}
			path, err := hooks.Install(root)
			if err != nil {
				return exitErr(21, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Installed IntentCI pre-push hook: %s\n", path)
			fmt.Fprintln(cmd.OutOrStdout(), "Note: Git hooks can be bypassed with git push --no-verify.")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "uninstall",
		Short: "Remove the managed IntentCI pre-push hook section",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := getwd()
			if err != nil {
				return err
			}
			root, err := config.FindRoot(cwd)
			if err != nil {
				root = cwd
			}
			path, err := hooks.Uninstall(root)
			if err != nil {
				return exitErr(21, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed IntentCI pre-push hook section from: %s\n", path)
			return nil
		},
	})
	return cmd
}

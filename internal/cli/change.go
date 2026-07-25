package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hypertrial/intentci/internal/changespec"
	"github.com/hypertrial/intentci/internal/config"
)

func newChangeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "change",
		Short: "Manage Change Specs",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "create <id>",
		Short: "Create a draft Change Spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := getwd()
			if err != nil {
				return err
			}
			root, err := config.FindRoot(cwd)
			if err != nil {
				return exitErr(21, err)
			}
			path, err := changespec.Create(root, args[0])
			if err != nil {
				return exitErr(20, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", path)
			fmt.Fprintln(cmd.OutOrStdout(), "Edit the Change Spec and set status to approved when ready.")
			return nil
		},
	})
	return cmd
}

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hypertrial/intentci/internal/initcmd"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize IntentCI in the current repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := getwd()
			if err != nil {
				return err
			}
			res, err := initcmd.Run(cwd)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized IntentCI in %s\n", res.Root)
			for _, p := range res.Created {
				fmt.Fprintf(cmd.OutOrStdout(), "  created %s\n", p)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Edit .intentci/contract.yaml and promote draft requirements to approved.")
			return nil
		},
	}
}

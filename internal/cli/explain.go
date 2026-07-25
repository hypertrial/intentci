package cli

import (
	"github.com/spf13/cobra"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/explain"
)

func newExplainCmd() *cobra.Command {
	var changeID string
	var base string
	cmd := &cobra.Command{
		Use:   "explain <requirement-id>",
		Short: "Explain a requirement or acceptance criterion",
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
			if err := explain.Run(explain.Options{
				Root:     root,
				ID:       args[0],
				ChangeID: changeID,
				Base:     base,
				Out:      cmd.OutOrStdout(),
			}); err != nil {
				return exitErr(20, err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&changeID, "change", "", "Change Spec id (required for AC-* ids)")
	cmd.Flags().StringVar(&base, "base", "", "Git base ref")
	return cmd
}

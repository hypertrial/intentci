package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/contract"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the Product Contract",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			root, err := config.FindRoot(cwd)
			if err != nil {
				return exitErr(21, err)
			}
			c, _, err := contract.LoadFromRoot(root)
			if err != nil {
				return exitErr(20, err)
			}
			if err := contract.Validate(c); err != nil {
				return exitErr(20, err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Product Contract is valid")
			return nil
		},
	}
}

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hypertrial/intentci/internal/changespec"
	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/contract"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the Product Contract and Change Specs",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := getwd()
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
			dir := changespec.Dir(root)
			entries, err := os.ReadDir(dir)
			if err != nil && !os.IsNotExist(err) {
				return exitErr(20, err)
			}
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
					continue
				}
				id := strings.TrimSuffix(e.Name(), ".yaml")
				spec, _, err := changespec.Load(root, id)
				if err != nil {
					return exitErr(20, fmt.Errorf("%s: %w", filepath.Join(dir, e.Name()), err))
				}
				if err := changespec.Validate(spec, c); err != nil {
					return exitErr(20, fmt.Errorf("%s: %w", id, err))
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Product Contract and Change Specs are valid")
			return nil
		},
	}
}

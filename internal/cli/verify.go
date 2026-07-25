package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/report"
	"github.com/hypertrial/intentci/internal/verify"
)

type runFlags struct {
	base   string
	all    bool
	format string
	output string
	trust  bool
}

func newCheckCmd() *cobra.Command {
	return newRunCmd("check", "fast", "Run fast-profile verification")
}

func newVerifyCmd() *cobra.Command {
	return newRunCmd("verify", "full", "Run full-profile verification")
}

func newRunCmd(use, profile, short string) *cobra.Command {
	f := &runFlags{}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			root, err := config.FindRoot(cwd)
			if err != nil {
				return exitErr(21, err)
			}
			switch f.format {
			case "text", "json":
			default:
				return exitErr(20, fmt.Errorf("unsupported format %q (want text|json)", f.format))
			}
			outcome, err := verify.Run(context.Background(), verify.Options{
				Root:    root,
				Base:    f.base,
				Profile: profile,
				All:     f.all,
				Trust:   f.trust,
				Stdout:  cmd.OutOrStdout(),
				Stderr:  cmd.ErrOrStderr(),
				Stream:  f.format != "json",
			})
			if err != nil {
				code := 30
				if outcome != nil && outcome.ExitCode != 0 {
					code = outcome.ExitCode
				}
				return exitErr(code, err)
			}
			if err := report.Write(f.format, f.output, outcome.Result, cmd.OutOrStdout()); err != nil {
				return exitErr(30, err)
			}
			if outcome.ExitCode != 0 {
				// Report already written; exit with status code and no extra message.
				return &ExitError{Code: outcome.ExitCode}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&f.base, "base", "", "Git base ref (default: policy.default_base)")
	cmd.Flags().BoolVar(&f.all, "all", false, "Verify all approved blocking requirements")
	cmd.Flags().StringVar(&f.format, "format", "text", "Output format: text|json")
	cmd.Flags().StringVar(&f.output, "output", "", "Write report to a file")
	cmd.Flags().BoolVar(&f.trust, "trust", false, "Trust this repository for local command execution")
	return cmd
}

// CodeOf extracts an exit code from an error chain.
func CodeOf(err error) int {
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return 1
}

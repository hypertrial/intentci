package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/version"
)

// ExecuteWith runs the root command.
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
		Short:         "Intent compiler and evidence-based verification for agent-generated code",
		Long:          "IntentCI compiles Markdown requirements into obligations, runs providers, and produces evidence-backed verdicts.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(newInitCmd())
	root.AddCommand(newCompileCmd())
	root.AddCommand(newVerifyCmd())
	root.AddCommand(newExplainCmd())
	root.AddCommand(newRepairCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newSchemaCmd())
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print IntentCI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.String())
		},
	})
	return root
}

// RunMain executes IntentCI and returns an exit code.
func RunMain(args []string, stdout, stderr io.Writer) int {
	err := ExecuteWith(args, stdout, stderr)
	if err == nil {
		return exitcode.Pass
	}
	if ee, ok := err.(*ExitError); ok {
		if ee.Msg != "" {
			fmt.Fprintln(stderr, ee.Msg)
		}
		return ee.Code
	}
	fmt.Fprintln(stderr, err.Error())
	return exitcode.Internal
}

// ExitError carries a process exit code.
type ExitError struct {
	Code int
	Msg  string
}

func (e *ExitError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return fmt.Sprintf("exit %d", e.Code)
}

func exitErr(code int, format string, args ...any) error {
	return &ExitError{Code: code, Msg: fmt.Sprintf(format, args...)}
}

var getwd = os.Getwd

// goos is overridable in tests for doctor platform checks.
var goos = runtime.GOOS

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hypertrial/intentci/internal/compiler"
	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/initcmd"
	"github.com/hypertrial/intentci/internal/repair"
	"github.com/hypertrial/intentci/internal/report"
	"github.com/hypertrial/intentci/internal/verify"
	appschema "github.com/hypertrial/intentci/pkg/schema"
)

func newInitCmd() *cobra.Command {
	var force, noExample bool
	var language, ci string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize IntentCI in the current repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := getwd()
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			if err := initcmd.Run(initcmd.Options{
				Root: root, Force: force, Language: language,
				CIGithub: strings.EqualFold(ci, "github"), NoExample: noExample,
			}); err != nil {
				return exitErr(exitcode.Usage, "%v", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Initialized", config.Dir(root))
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config")
	cmd.Flags().StringVar(&language, "language", "", "Example language (go|python|typescript|rust)")
	cmd.Flags().StringVar(&ci, "ci", "", "CI template (github)")
	cmd.Flags().BoolVar(&noExample, "no-example", false, "Skip example requirement")
	return cmd
}

func newCompileCmd() *cobra.Command {
	var strict bool
	var requirement, format, output string
	cmd := &cobra.Command{
		Use:   "compile",
		Short: "Compile Markdown requirements into canonical Intent IR",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := getwd()
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			res, err := compiler.Compile(compiler.Options{
				Root: root, RequirementID: requirement, Strict: strict,
			})
			for _, d := range res.Diagnostics {
				fmt.Fprintln(cmd.ErrOrStderr(), d.Error())
			}
			if err != nil {
				return exitErr(exitcode.CompileFailed, "%v", err)
			}
			store, err := evidence.NewStore(root, mustConfig(root).Evidence.Directory)
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			runID := evidence.NewRunID()
			outPath := output
			if outPath == "" {
				outPath = filepath.Join(store.Dir(runID), "compiled-intent.json")
			}
			if err := compiler.WriteIR(res.Document, outPath); err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			_ = os.WriteFile(filepath.Join(store.Root, "latest-compile"), []byte(runID+"\n"), 0o644)
			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res.Document)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Compiled %d requirement(s) → %s\n", len(res.Document.Requirements), outPath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "Fail on any diagnostic")
	cmd.Flags().StringVar(&requirement, "requirement", "", "Compile a single requirement id")
	cmd.Flags().StringVar(&format, "format", "text", "text|json")
	cmd.Flags().StringVar(&output, "output", "", "Output path for IR JSON")
	return cmd
}

func mustConfig(root string) *config.Config {
	cfg, err := config.Load(root)
	if err != nil {
		return config.Default()
	}
	return cfg
}

func newVerifyCmd() *cobra.Command {
	var all, changed, noCache bool
	var base, requirement, obligation, format, output string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Compile, select, execute providers, and emit verdicts",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := getwd()
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			if !all && !changed && requirement == "" {
				changed = true
			}
			out, err := verify.Run(context.Background(), verify.Options{
				Root: root, Base: base, All: all, Changed: changed,
				RequirementID: requirement, ObligationID: obligation,
				NoCache: noCache, Format: format,
			})
			if err != nil {
				code := exitcode.Internal
				if out != nil && out.ExitCode != 0 {
					code = out.ExitCode
				}
				return exitErr(code, "%v", err)
			}
			_ = report.WriteGitHubStepSummary(out.Bundle)
			w := cmd.OutOrStdout()
			var file *os.File
			if output != "" {
				file, err = os.Create(output)
				if err != nil {
					return exitErr(exitcode.Internal, "%v", err)
				}
				defer file.Close()
				w = file
			}
			if err := report.Write(w, format, out.Bundle); err != nil {
				return exitErr(exitcode.Usage, "%v", err)
			}
			if out.ExitCode != exitcode.Pass {
				return &ExitError{Code: out.ExitCode}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Verify all active requirements")
	cmd.Flags().BoolVar(&changed, "changed", false, "Verify requirements affected by the Git diff")
	cmd.Flags().StringVar(&base, "base", "", "Git base ref")
	cmd.Flags().StringVar(&requirement, "requirement", "", "Requirement id")
	cmd.Flags().StringVar(&obligation, "obligation", "", "Obligation id")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Disable provider cache")
	cmd.Flags().StringVar(&format, "format", "text", "text|json|junit")
	cmd.Flags().StringVar(&output, "output", "", "Write report to path")
	return cmd
}

func newExplainCmd() *cobra.Command {
	var runID string
	var showEvidence, showLogs bool
	var format string
	cmd := &cobra.Command{
		Use:   "explain [id]",
		Short: "Explain a requirement verdict from a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := getwd()
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			cfg := mustConfig(root)
			store, err := evidence.NewStore(root, cfg.Evidence.Directory)
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			var b *evidence.Bundle
			if runID != "" {
				b, err = store.Load(runID)
			} else {
				b, err = store.LoadLatest()
			}
			if err != nil {
				return exitErr(exitcode.Usage, "no run to explain: %v", err)
			}
			_ = showLogs
			if format == "json" {
				for _, r := range b.Run.Requirements {
					if r.ID == args[0] {
						enc := json.NewEncoder(cmd.OutOrStdout())
						enc.SetIndent("", "  ")
						return enc.Encode(r)
					}
				}
				return exitErr(exitcode.Usage, "requirement %q not found", args[0])
			}
			if err := report.Explain(cmd.OutOrStdout(), b, args[0], showEvidence); err != nil {
				return exitErr(exitcode.Usage, "%v", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&runID, "run", "", "Run id")
	cmd.Flags().BoolVar(&showEvidence, "show-evidence", false, "Show evidence details")
	cmd.Flags().BoolVar(&showLogs, "show-logs", false, "Show provider logs")
	cmd.Flags().StringVar(&format, "format", "text", "text|json")
	return cmd
}

func newRepairCmd() *cobra.Command {
	var agentCommand, requirement string
	var maxAttempts int
	var dryRun, changed, allowTest bool
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Run a bounded agent repair loop",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := getwd()
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			cfg, err := config.Load(root)
			if err != nil {
				return exitErr(exitcode.CompileFailed, "%v", err)
			}
			if allowTest {
				cfg.Repair.AllowTestChanges = true
			}
			store, err := evidence.NewStore(root, cfg.Evidence.Directory)
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			_ = requirement
			out, err := repair.Run(context.Background(), repair.Options{
				Root: root, Config: cfg, Store: store, AgentCommand: agentCommand,
				MaxAttempts: maxAttempts, DryRun: dryRun,
				Verify: func(ctx context.Context) (*evidence.Bundle, error) {
					o, err := verify.Run(ctx, verify.Options{
						Root: root, All: !changed, Changed: changed, RequirementID: requirement, NoCache: true,
					})
					if err != nil {
						return nil, err
					}
					return o.Bundle, nil
				},
			})
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			if out.Bundle != nil {
				_ = report.Write(cmd.OutOrStdout(), "text", out.Bundle)
			}
			if out.Stopped != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "repair stopped:", out.Stopped)
			}
			if out.ExitCode != exitcode.Pass {
				return &ExitError{Code: out.ExitCode}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agentCommand, "agent-command", "", "Shell command; {packet} is replaced with packet path")
	cmd.Flags().StringVar(&requirement, "requirement", "", "Limit to requirement id")
	cmd.Flags().IntVar(&maxAttempts, "max-attempts", 0, "Override repair.max_attempts")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Build packets without invoking an agent")
	cmd.Flags().BoolVar(&changed, "changed", false, "Verify changed requirements each attempt")
	cmd.Flags().BoolVar(&allowTest, "allow-test-changes", false, "Allow the agent to modify tests")
	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show repository IntentCI status from the latest run",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := getwd()
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			cfg := mustConfig(root)
			store, err := evidence.NewStore(root, cfg.Evidence.Directory)
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			b, err := store.LoadLatest()
			if err != nil {
				return exitErr(exitcode.Usage, "no runs yet: %v", err)
			}
			active, verified, failed, unproven, uncertain := 0, 0, 0, 0, 0
			for _, r := range b.Run.Requirements {
				active++
				switch r.Verdict {
				case "pass":
					verified++
				case "fail":
					failed++
				case "unproven":
					unproven++
				case "uncertain":
					uncertain++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Requirements: %d active\nVerified:     %d\nFailed:        %d\nUnproven:      %d\nUncertain:     %d\nLast run:      %s\nCommit:        %s\n",
				active, verified, failed, unproven, uncertain, b.CreatedAt.Format(timeRFC3339), short(b.HeadCommit))
			return nil
		},
	}
}

const timeRFC3339 = "2006-01-02T15:04:05Z"

func short(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local dependencies and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := getwd()
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			ok := true
			check := func(name string, err error) {
				if err != nil {
					ok = false
					fmt.Fprintf(cmd.OutOrStdout(), "FAIL %s: %v\n", name, err)
					return
				}
				fmt.Fprintf(cmd.OutOrStdout(), "OK   %s\n", name)
			}
			if !git.IsRepo(root) {
				check("git repository", fmt.Errorf("not a git repository"))
			} else {
				check("git repository", nil)
			}
			cfg, err := config.Load(root)
			check("configuration", err)
			if err == nil {
				store, err := evidence.NewStore(root, cfg.Evidence.Directory)
				if err != nil {
					check("evidence directory", err)
				} else {
					probe := filepath.Join(store.Root, ".write-probe")
					err := os.WriteFile(probe, []byte("ok"), 0o644)
					_ = os.Remove(probe)
					check("evidence directory writable", err)
				}
			}
			switch goos {
			case "linux", "darwin":
				check("platform "+goos+"/"+runtime.GOARCH, nil)
			default:
				check("platform", fmt.Errorf("unsupported %s (use WSL on Windows)", goos))
			}
			if cfg != nil && cfg.Telemetry.Enabled {
				check("telemetry disabled", fmt.Errorf("telemetry.enabled is true"))
			} else {
				check("telemetry disabled", nil)
			}
			if !ok {
				return &ExitError{Code: exitcode.Usage}
			}
			return nil
		},
	}
}

func newSchemaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schema [name]",
		Short: "Print a JSON schema (requirement|evidence|verdict|repair|ir)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var raw []byte
			switch strings.ToLower(args[0]) {
			case "requirement":
				raw = appschema.RequirementJSON
			case "evidence":
				raw = appschema.EvidenceJSON
			case "verdict":
				raw = appschema.VerdictJSON
			case "repair":
				raw = appschema.RepairJSON
			case "ir", "intent":
				raw = appschema.IRJSON
			default:
				return exitErr(exitcode.Usage, "unknown schema %q", args[0])
			}
			_, err := cmd.OutOrStdout().Write(raw)
			return err
		},
	}
}

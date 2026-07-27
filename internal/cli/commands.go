package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	"github.com/hypertrial/intentci/internal/security"
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
			if format != "text" && format != "json" {
				return exitErr(exitcode.Usage, "unsupported format %q", format)
			}
			if cmd.Flags().Changed("requirement") && strings.TrimSpace(requirement) == "" {
				return exitErr(exitcode.Usage, "--requirement must not be empty")
			}
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
			if requirement != "" && len(res.Document.Requirements) == 0 {
				return exitErr(exitcode.Usage, "requirement %q not found", requirement)
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
	var all, changed, noCache, failFast, noGit bool
	var base, head, requirement, obligation, providerID, format, output string
	var maxParallel int
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Compile, select, execute providers, and emit verdicts",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := getwd()
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			if all && changed {
				return exitErr(exitcode.Usage, "--all and --changed are mutually exclusive")
			}
			if format != "text" && format != "json" && format != "junit" {
				return exitErr(exitcode.Usage, "unsupported format %q", format)
			}
			if maxParallel < 0 {
				return exitErr(exitcode.Usage, "--max-parallel must be >= 0")
			}
			for name, value := range map[string]string{
				"requirement": requirement, "obligation": obligation, "provider": providerID,
			} {
				if cmd.Flags().Changed(name) && strings.TrimSpace(value) == "" {
					return exitErr(exitcode.Usage, "--%s must not be empty", name)
				}
			}
			if !all && !changed && requirement == "" {
				changed = true
			}
			out, err := runVerification(cmd.Context(), verify.Options{
				Root: root, Base: base, Head: head, All: all, Changed: changed,
				RequirementID: requirement, ObligationID: obligation,
				ProviderID: providerID, MaxParallel: maxParallel,
				MaxParallelSet: cmd.Flags().Changed("max-parallel"),
				FailFast:       failFast, FailFastSet: cmd.Flags().Changed("fail-fast"),
				NoGit: noGit, NoCache: noCache, Format: format,
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
			if output != "" {
				var rendered bytes.Buffer
				_ = report.Write(&rendered, format, out.Bundle)
				if err := writeReportFile(root, output, rendered.Bytes()); err != nil {
					if security.IsPathViolation(err) {
						return exitErr(exitcode.SecurityBoundary, "%v", err)
					}
					return exitErr(exitcode.Internal, "%v", err)
				}
			} else if err := report.Write(w, format, out.Bundle); err != nil {
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
	cmd.Flags().StringVar(&head, "head", "", "Git head ref")
	cmd.Flags().StringVar(&requirement, "requirement", "", "Requirement id")
	cmd.Flags().StringVar(&obligation, "obligation", "", "Obligation id")
	cmd.Flags().StringVar(&providerID, "provider", "", "Verifier provider id")
	cmd.Flags().IntVar(&maxParallel, "max-parallel", 0, "Override verification.max_parallel")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "Stop scheduling after the first non-pass result")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Disable provider cache")
	cmd.Flags().BoolVar(&noGit, "no-git", false, "Verify without Git provenance (requires --all or --requirement)")
	cmd.Flags().StringVar(&format, "format", "text", "text|json|junit")
	cmd.Flags().StringVar(&output, "output", "", "Write report to path")
	return cmd
}

func writeReportFile(root, relative string, content []byte) error {
	path := relative
	if !filepath.IsAbs(path) {
		var err error
		path, err = security.ResolveInside(root, relative)
		if err != nil {
			return err
		}
	}
	if err := makeReportDirs(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := createReportTemp(filepath.Dir(path), ".intentci-report-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer removeReportFile(name)
	written, err := file.Write(content)
	if err != nil {
		file.Close()
		return err
	}
	if written != len(content) {
		file.Close()
		return io.ErrShortWrite
	}
	if err := file.Close(); err != nil {
		return err
	}
	return renameReportFile(name, path)
}

type reportTempFile interface {
	io.Writer
	Name() string
	Close() error
}

var makeReportDirs = os.MkdirAll
var createReportTemp = func(directory, pattern string) (reportTempFile, error) {
	return os.CreateTemp(directory, pattern)
}
var removeReportFile = os.Remove
var renameReportFile = os.Rename
var runVerification = verify.Run

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
			if format != "text" && format != "json" {
				return exitErr(exitcode.Usage, "unsupported format %q", format)
			}
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
			if format == "json" {
				value, found := explainJSONValue(b, args[0])
				if !found {
					return exitErr(exitcode.Usage, "identifier %q not found", args[0])
				}
				if showLogs {
					value = map[string]any{"result": value, "provider_results": b.ProviderLogs}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(value)
			}
			if err := report.ExplainWithOptions(cmd.OutOrStdout(), b, args[0], report.ExplainOptions{
				ShowEvidence: showEvidence, ShowLogs: showLogs,
			}); err != nil {
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
	var agent, agentCommand, requirement string
	var maxAttempts int
	var dryRun, changed, allowTest bool
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Run a bounded agent repair loop",
		RunE: func(cmd *cobra.Command, args []string) error {
			if agent != "" && agentCommand != "" {
				return exitErr(exitcode.Usage, "--agent and --agent-command are mutually exclusive")
			}
			if maxAttempts < 0 || (cmd.Flags().Changed("max-attempts") && maxAttempts < 1) {
				return exitErr(exitcode.Usage, "--max-attempts must be >= 1")
			}
			if cmd.Flags().Changed("requirement") && strings.TrimSpace(requirement) == "" {
				return exitErr(exitcode.Usage, "--requirement must not be empty")
			}
			if cmd.Flags().Changed("agent") && strings.TrimSpace(agent) == "" {
				return exitErr(exitcode.Usage, "--agent must not be empty")
			}
			if cmd.Flags().Changed("agent-command") && strings.TrimSpace(agentCommand) == "" {
				return exitErr(exitcode.Usage, "--agent-command must not be empty")
			}
			if agent != "" {
				if !validAdapterName(agent) {
					return exitErr(exitcode.Usage, "invalid agent name %q", agent)
				}
				path, err := exec.LookPath("intentci-agent-" + agent)
				if err != nil {
					return exitErr(exitcode.Usage, "agent %q not found on PATH", agent)
				}
				agentCommand = fmt.Sprintf("%q {packet}", path)
			}
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
			compiled, err := compiler.Compile(compiler.Options{Root: root, Config: cfg, Strict: true})
			if err != nil {
				return exitErr(exitcode.CompileFailed, "%v", err)
			}
			store, err := evidence.NewStore(root, cfg.Evidence.Directory)
			if err != nil {
				return exitErr(exitcode.Internal, "%v", err)
			}
			store.RedactPatterns = append([]string{}, cfg.Evidence.Redact.Environment...)
			runID := evidence.NewRunID()
			attempt := 0
			if agentCommand != "" && !dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: repair executes the agent on the host with your current user permissions; IntentCI v1 is not a sandbox.")
			}
			out, err := repair.Run(cmd.Context(), repair.Options{
				Root: root, Config: cfg, Store: store, AgentCommand: agentCommand,
				MaxAttempts: maxAttempts, DryRun: dryRun,
				Verify: func(ctx context.Context) (*evidence.Bundle, error) {
					attempt++
					o, err := runVerification(ctx, verify.Options{
						Root: root, All: !changed, Changed: changed, RequirementID: requirement, NoCache: true,
						Config: cfg, Document: compiled.Document, RunID: runID,
						AttemptID: fmt.Sprintf("attempt-%03d", attempt), AttemptOnly: true,
					})
					if err != nil {
						return nil, err
					}
					return o.Bundle, nil
				},
				Finalize: func(bundle *evidence.Bundle) error {
					return verify.FinalizeBundle(store, bundle)
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
	cmd.Flags().StringVar(&agent, "agent", "", "Agent name resolved as intentci-agent-NAME on PATH")
	cmd.Flags().StringVar(&agentCommand, "agent-command", "", "Shell command; {packet} is replaced with packet path")
	cmd.Flags().StringVar(&requirement, "requirement", "", "Limit to requirement id")
	cmd.Flags().IntVar(&maxAttempts, "max-attempts", 0, "Override repair.max_attempts")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Build packets without invoking an agent")
	cmd.Flags().BoolVar(&changed, "changed", false, "Verify changed requirements each attempt")
	cmd.Flags().BoolVar(&allowTest, "allow-test-changes", false, "Allow the agent to modify tests")
	return cmd
}

func validAdapterName(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
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
			compiled, err := compiler.Compile(compiler.Options{Root: root, Config: cfg})
			if err != nil {
				return exitErr(exitcode.CompileFailed, "%v", err)
			}
			latest := map[string]string{}
			for _, result := range b.Run.Requirements {
				latest[result.ID] = result.Verdict
			}
			active, verified, failed, unproven, uncertain := 0, 0, 0, 0, 0
			review, errors, skipped := 0, 0, 0
			for _, requirement := range compiled.Document.ActiveRequirements() {
				active++
				switch latest[requirement.ID] {
				case "pass":
					verified++
				case "fail":
					failed++
				case "unproven":
					unproven++
				case "uncertain":
					uncertain++
				case "review_required":
					review++
				case "error":
					errors++
				case "skipped":
					skipped++
				default:
					unproven++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Requirements: %d active\nVerified:     %d\nFailed:        %d\nUnproven:      %d\nUncertain:     %d\nReview:        %d\nErrors:        %d\nSkipped:       %d\nLast run:      %s\nCommit:        %s\n",
				active, verified, failed, unproven, uncertain, review, errors, skipped,
				b.CreatedAt.Format(timeRFC3339), short(b.HeadCommit))
			if b.RepositoryState != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Base:          %s\nDirty:         %t\nChanged files: %d\n",
					short(b.RepositoryState.BaseCommit), b.RepositoryState.WorkingTreeDirty,
					len(b.RepositoryState.ChangedFiles))
			}
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
		Short: "Print a JSON schema (requirement|evidence|verdict|repair|ir|plan|report)",
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
			case "report":
				raw = appschema.ReportJSON
			case "plan":
				raw = appschema.PlanJSON
			default:
				return exitErr(exitcode.Usage, "unknown schema %q", args[0])
			}
			_, err := cmd.OutOrStdout().Write(raw)
			return err
		},
	}
}

func explainJSONValue(bundle *evidence.Bundle, id string) (any, bool) {
	if id == bundle.RunID {
		return bundle, true
	}
	for _, requirement := range bundle.Run.Requirements {
		if requirement.ID == id {
			return requirement, true
		}
		for _, obligation := range requirement.Obligations {
			if obligation.ID == id {
				return obligation, true
			}
			for _, record := range obligation.Evidence {
				if record.ID == id || record.VerifierID == id {
					return record, true
				}
			}
		}
	}
	for key, result := range bundle.ProviderLogs {
		if key == id || strings.HasSuffix(key, "/"+id) {
			return result, true
		}
	}
	return nil, false
}

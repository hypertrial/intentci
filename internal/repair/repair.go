package repair

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/security"
	"github.com/hypertrial/intentci/internal/verdict"
)

// Packet is the structured repair packet for agents.
type Packet struct {
	RunID              string    `json:"run_id"`
	Requirement        string    `json:"requirement_id,omitempty"`
	Verdict            string    `json:"verdict"`
	Summary            string    `json:"summary"`
	Intent             string    `json:"intent,omitempty"`
	AllowedPaths       []string  `json:"allowed_paths,omitempty"`
	Forbidden          []string  `json:"forbidden_paths,omitempty"`
	ProtectedPaths     []string  `json:"protected_paths,omitempty"`
	TestChangesAllowed bool      `json:"test_changes_allowed"`
	Instructions       []string  `json:"instructions,omitempty"`
	Failures           []Failure `json:"failures"`
	Attempt            int       `json:"attempt"`
	MaxAttempts        int       `json:"max_attempts"`
}

type Failure struct {
	Requirement string   `json:"requirement_id"`
	Obligation  string   `json:"obligation_id"`
	Verdict     string   `json:"verdict"`
	Reason      string   `json:"reason"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
	Paths       []string `json:"paths,omitempty"`
}

// Options configures the repair loop.
type Options struct {
	Root         string
	Config       *config.Config
	Store        *evidence.Store
	AgentCommand string
	MaxAttempts  int
	DryRun       bool
	Verify       func(ctx context.Context) (*evidence.Bundle, error)
	Finalize     func(bundle *evidence.Bundle) error
}

// Outcome is the repair loop result.
type Outcome struct {
	Bundle   *evidence.Bundle
	Attempts int
	ExitCode int
	Stopped  string
}

// BuildPacket creates a repair packet from a failed bundle.
func BuildPacket(b *evidence.Bundle, attempt, max int, requirementID string) Packet {
	p := Packet{
		RunID: b.RunID, Requirement: requirementID, Verdict: b.Run.Verdict,
		Attempt: attempt, MaxAttempts: max,
		Summary: "IntentCI verification did not pass; repair the failing obligations.",
		Instructions: []string{
			"Change only files inside allowed paths and never files inside forbidden or protected paths.",
			"Do not weaken, remove, or bypass required verification selectors.",
			"Do not claim success; IntentCI will independently rerun verification.",
		},
	}
	for _, r := range b.Run.Requirements {
		if requirementID != "" && r.ID != requirementID {
			continue
		}
		for _, o := range r.Obligations {
			if o.Verdict == verdict.Pass || o.Verdict == verdict.Skipped {
				continue
			}
			failure := Failure{
				Requirement: r.ID, Obligation: o.ID, Verdict: o.Verdict, Reason: o.Reason,
			}
			for _, record := range o.Evidence {
				failure.EvidenceIDs = append(failure.EvidenceIDs, record.ID)
				failure.Paths = append(failure.Paths, record.Paths...)
			}
			failure.EvidenceIDs = uniqueSorted(failure.EvidenceIDs)
			failure.Paths = uniqueSorted(failure.Paths)
			p.Failures = append(p.Failures, failure)
		}
	}
	if b.Document != nil {
		for _, r := range b.Document.Requirements {
			if requirementID != "" && r.ID != requirementID {
				continue
			}
			if r.Intent != "" {
				if p.Intent != "" {
					p.Intent += "\n\n"
				}
				p.Intent += r.ID + ": " + r.Intent
			}
			p.AllowedPaths = append(p.AllowedPaths, r.Boundaries.Allowed...)
			p.Forbidden = append(p.Forbidden, r.Boundaries.Forbidden...)
		}
		p.AllowedPaths = uniqueSorted(p.AllowedPaths)
		p.Forbidden = uniqueSorted(p.Forbidden)
	}
	return p
}

// Run executes the bounded repair loop.
func Run(ctx context.Context, opt Options) (outcome *Outcome, runErr error) {
	defer func() {
		if outcome == nil || outcome.Bundle == nil || opt.Finalize == nil {
			return
		}
		if err := opt.Finalize(outcome.Bundle); err != nil && runErr == nil {
			outcome.ExitCode = exitcode.Internal
			runErr = err
		}
	}()
	limit := opt.MaxAttempts
	if limit <= 0 {
		limit = opt.Config.Repair.MaxAttempts
	}
	limit = max(limit, 1)

	var last *evidence.Bundle
	var diffFingerprints []string
	var failFingerprints []string
	agentErrors := 0
	initial, err := git.Resolve(opt.Root, "HEAD")
	if err != nil {
		return &Outcome{ExitCode: exitcode.Internal}, err
	}
	if violations := security.ProtectedViolation(initial.ChangedFiles, opt.Config.Repair.AllowRequirementChanges, opt.Config.Repair.ProtectedPaths); len(violations) > 0 {
		return &Outcome{ExitCode: exitcode.SecurityBoundary, Stopped: "preexisting_protected_path:" + strings.Join(violations, ",")}, nil
	}

	for attempt := 1; attempt <= limit; attempt++ {
		b, err := opt.Verify(ctx)
		if err != nil {
			return &Outcome{ExitCode: exitcode.Internal, Attempts: attempt}, err
		}
		last = b
		if ctx.Err() != nil || b.Interrupted {
			b.Interrupted = true
			b.Run.Verdict = verdict.Error
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.VerifierError, Stopped: "interrupted"}, nil
		}
		if b.Run.Verdict == verdict.Pass {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Pass}, nil
		}
		if b.Run.Verdict == verdict.ReviewRequired {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: verdict.ExitCode(verdict.ReviewRequired), Stopped: "review_required"}, nil
		}

		packet := BuildPacket(b, attempt, limit, "")
		packet.ProtectedPaths = uniqueSorted(append(append([]string{}, security.DefaultProtected...), opt.Config.Repair.ProtectedPaths...))
		packet.TestChangesAllowed = opt.Config.Repair.AllowTestChanges
		attemptID := b.AttemptID
		if attemptID == "" {
			attemptID = fmt.Sprintf("attempt-%03d", attempt)
		}
		packetPath, err := opt.Store.WriteRepairPacketForAttempt(b.RunID, attemptID, packet)
		if err != nil {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Internal}, err
		}

		fp := failureFingerprint(packet)
		if agentErrors == 0 && opt.Config.Repair.StopOnRepeatedFailure && contains(failFingerprints, fp) {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.RepairExhausted, Stopped: "repeated_failure"}, nil
		}
		failFingerprints = append(failFingerprints, fp)

		if attempt == limit {
			break
		}
		if opt.DryRun || opt.AgentCommand == "" {
			continue
		}

		before, err := takeSnapshot(opt.Root)
		if err != nil {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Internal}, err
		}
		ignoreStoreFiles(before, opt.Root, opt.Store.Root)
		beforePatch, err := takePatch(opt.Root, opt.Store.Root)
		if err != nil {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Internal}, err
		}
		if err := opt.Store.WriteRepairArtifact(b.RunID, attemptID, "patch-before.diff", beforePatch); err != nil {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Internal}, err
		}
		cmdStr := strings.ReplaceAll(opt.AgentCommand, "{packet}", packetPath)
		cmdStr = strings.ReplaceAll(cmdStr, "{repository}", opt.Root)
		cmdStr = strings.ReplaceAll(cmdStr, "{attempt}", fmt.Sprintf("%d", attempt))
		cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
		cmd.Dir = opt.Root
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		agentErr := cmd.Run()
		if err := opt.Store.WriteAgentLog(b.RunID, attemptID, "stdout", stdout.Bytes()); err != nil {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Internal}, err
		}
		if err := opt.Store.WriteAgentLog(b.RunID, attemptID, "stderr", stderr.Bytes()); err != nil {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Internal}, err
		}
		exitRecord := map[string]any{"status": "completed", "exit_code": 0}
		if agentErr != nil {
			exitRecord["status"] = "error"
			exitRecord["error"] = agentErr.Error()
			exitRecord["exit_code"] = agentExitCode(agentErr)
			agentErrors++
		} else {
			agentErrors = 0
		}
		exitRaw, _ := json.MarshalIndent(exitRecord, "", "  ")
		if err := opt.Store.WriteRepairArtifact(b.RunID, attemptID, "agent-exit.json", append(exitRaw, '\n')); err != nil {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Internal}, err
		}
		if ctx.Err() != nil {
			b.Interrupted = true
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.VerifierError, Stopped: "interrupted"}, nil
		}

		after, err := takeSnapshot(opt.Root)
		if err != nil {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Internal}, err
		}
		ignoreStoreFiles(after, opt.Root, opt.Store.Root)
		afterPatch, err := takePatch(opt.Root, opt.Store.Root)
		if err != nil {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Internal}, err
		}
		if err := opt.Store.WriteRepairArtifact(b.RunID, attemptID, "patch-after.diff", afterPatch); err != nil {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Internal}, err
		}
		changed := diffPaths(before, after)
		if viol := security.ProtectedViolation(changed, opt.Config.Repair.AllowRequirementChanges, opt.Config.Repair.ProtectedPaths); len(viol) > 0 {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.SecurityBoundary, Stopped: "protected_path:" + strings.Join(viol, ",")}, nil
		}
		if viol := security.BoundaryViolations(changed, packet.AllowedPaths, packet.Forbidden); len(viol) > 0 {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.SecurityBoundary, Stopped: "boundary:" + strings.Join(viol, ",")}, nil
		}
		if !opt.Config.Repair.AllowTestChanges {
			for _, c := range changed {
				if security.IsTestPath(c) {
					return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.SecurityBoundary, Stopped: "test_change:" + c}, nil
				}
			}
		}
		dfp := diffFingerprint(before, after)
		if opt.Config.Repair.StopOnRepeatedDiff && contains(diffFingerprints, dfp) {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.RepairExhausted, Stopped: "repeated_diff"}, nil
		}
		diffFingerprints = append(diffFingerprints, dfp)
		if agentErrors >= 2 {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.RepairExhausted, Stopped: "repeated_agent_error"}, nil
		}
	}

	return &Outcome{Bundle: last, Attempts: limit, ExitCode: exitcode.RepairExhausted, Stopped: "max_attempts"}, nil
}

func agentExitCode(err error) int {
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return -1
}

func capturePatch(root, storeRoot string) ([]byte, error) {
	arguments := []string{"diff", "--binary", "--no-ext-diff", "HEAD", "--", "."}
	excluded := storePrefix(root, storeRoot)
	if excluded != "" {
		arguments = append(arguments, ":(exclude)"+excluded+"/**")
	}
	tracked, err := gitOutput(root, arguments...)
	if err != nil {
		return nil, err
	}
	raw, err := gitOutput(root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	output.Write(tracked)
	for _, relative := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if relative == "" {
			continue
		}
		if excluded != "" && (relative == excluded || strings.HasPrefix(filepath.ToSlash(relative), excluded+"/")) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "\nintentci-untracked %s sha256=%s\n", filepath.ToSlash(relative), hashBytes(content))
		output.Write(content)
		if len(content) > 0 && content[len(content)-1] != '\n' {
			output.WriteByte('\n')
		}
	}
	return output.Bytes(), nil
}

var takePatch = capturePatch

var gitOutput = func(root string, arguments ...string) ([]byte, error) {
	command := exec.Command("git", arguments...)
	command.Dir = root
	return command.Output()
}

var relativePath = filepath.Rel

func storePrefix(root, storeRoot string) string {
	relative, err := relativePath(root, storeRoot)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(relative)
}

func hashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func failureFingerprint(p Packet) string {
	b, _ := json.Marshal(p.Failures)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hashStrings(ss []string) string {
	b, _ := json.Marshal(ss)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func uniqueSorted(ss []string) []string {
	sort.Strings(ss)
	out := ss[:0]
	for _, s := range ss {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func snapshotDiff(root string) (map[string]string, error) {
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, f := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if f == "" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(f))
		info, err := os.Lstat(path)
		if err != nil {
			out[filepath.ToSlash(f)] = "missing:" + err.Error()
			continue
		}
		var data []byte
		if info.Mode()&os.ModeSymlink != 0 {
			target, _ := os.Readlink(path)
			data = []byte("symlink:" + target)
		} else {
			data, _ = os.ReadFile(path)
		}
		sum := sha256.Sum256(append([]byte(info.Mode().String()+"\x00"), data...))
		out[filepath.ToSlash(f)] = hex.EncodeToString(sum[:])
	}
	return out, nil
}

var takeSnapshot = snapshotDiff

func diffPaths(before, after map[string]string) []string {
	var out []string
	for k, v := range after {
		if before[k] != v {
			out = append(out, k)
		}
	}
	for k := range before {
		if after[k] == "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func diffFingerprint(before, after map[string]string) string {
	changed := diffPaths(before, after)
	parts := make([]string, 0, len(changed))
	for _, path := range changed {
		parts = append(parts, path+"\x00"+before[path]+"\x00"+after[path])
	}
	return hashStrings(parts)
}

func ignoreStoreFiles(snapshot map[string]string, repoRoot, storeRoot string) {
	rel, err := filepath.Rel(repoRoot, storeRoot)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return
	}
	prefix := filepath.ToSlash(rel) + "/"
	for path := range snapshot {
		if strings.HasPrefix(path, prefix) {
			delete(snapshot, path)
		}
	}
}

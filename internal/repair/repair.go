package repair

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/security"
	"github.com/hypertrial/intentci/internal/verdict"
)

// Packet is the structured repair packet for agents.
type Packet struct {
	RunID        string   `json:"run_id"`
	Requirement  string   `json:"requirement_id,omitempty"`
	Verdict      string   `json:"verdict"`
	Summary      string   `json:"summary"`
	AllowedPaths []string `json:"allowed_paths,omitempty"`
	Forbidden    []string `json:"forbidden_paths,omitempty"`
	Failures     []Failure `json:"failures"`
	Attempt      int      `json:"attempt"`
	MaxAttempts  int      `json:"max_attempts"`
}

type Failure struct {
	Requirement string `json:"requirement_id"`
	Obligation  string `json:"obligation_id"`
	Verdict     string `json:"verdict"`
	Reason      string `json:"reason"`
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
	}
	for _, r := range b.Run.Requirements {
		if requirementID != "" && r.ID != requirementID {
			continue
		}
		for _, o := range r.Obligations {
			if o.Verdict == verdict.Pass || o.Verdict == verdict.Skipped {
				continue
			}
			p.Failures = append(p.Failures, Failure{
				Requirement: r.ID, Obligation: o.ID, Verdict: o.Verdict, Reason: o.Reason,
			})
		}
	}
	return p
}

// Run executes the bounded repair loop.
func Run(ctx context.Context, opt Options) (*Outcome, error) {
	max := opt.MaxAttempts
	if max <= 0 {
		max = opt.Config.Repair.MaxAttempts
	}
	if max < 1 {
		max = 1
	}

	var last *evidence.Bundle
	var diffFingerprints []string
	var failFingerprints []string

	for attempt := 1; attempt <= max; attempt++ {
		b, err := opt.Verify(ctx)
		if err != nil {
			return &Outcome{ExitCode: exitcode.Internal, Attempts: attempt}, err
		}
		last = b
		if b.Run.Verdict == verdict.Pass {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Pass}, nil
		}

		packet := BuildPacket(b, attempt, max, "")
		if err := opt.Store.WriteRepairPacket(b.RunID, packet); err != nil {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.Internal}, err
		}
		packetPath := filepath.Join(opt.Store.Dir(b.RunID), "repair-packet.json")

		fp := failureFingerprint(packet)
		if opt.Config.Repair.StopOnRepeatedFailure && contains(failFingerprints, fp) {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.RepairExhausted, Stopped: "repeated_failure"}, nil
		}
		failFingerprints = append(failFingerprints, fp)

		if opt.DryRun || opt.AgentCommand == "" {
			if attempt == max {
				break
			}
			continue
		}

		before, _ := snapshotDiff(opt.Root)
		cmdStr := strings.ReplaceAll(opt.AgentCommand, "{packet}", packetPath)
		cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
		cmd.Dir = opt.Root
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		_ = cmd.Run()
		_ = os.WriteFile(filepath.Join(opt.Store.Dir(b.RunID), fmt.Sprintf("agent-attempt-%d.log", attempt)),
			append(stdout.Bytes(), stderr.Bytes()...), 0o644)

		after, _ := snapshotDiff(opt.Root)
		changed := diffPaths(before, after)
		if viol := security.ProtectedViolation(changed, opt.Config.Repair.AllowRequirementChanges, nil); len(viol) > 0 {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.SecurityBoundary, Stopped: "protected_path:" + strings.Join(viol, ",")}, nil
		}
		if !opt.Config.Repair.AllowTestChanges {
			for _, c := range changed {
				if security.IsTestPath(c) {
					return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.SecurityBoundary, Stopped: "test_change:" + c}, nil
				}
			}
		}
		dfp := hashStrings(changed)
		if opt.Config.Repair.StopOnRepeatedDiff && contains(diffFingerprints, dfp) {
			return &Outcome{Bundle: b, Attempts: attempt, ExitCode: exitcode.RepairExhausted, Stopped: "repeated_diff"}, nil
		}
		diffFingerprints = append(diffFingerprints, dfp)
	}

	return &Outcome{Bundle: last, Attempts: max, ExitCode: exitcode.RepairExhausted, Stopped: "max_attempts"}, nil
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

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func snapshotDiff(root string) (map[string]string, error) {
	st, err := git.Resolve(root, "HEAD")
	if err != nil {
		return map[string]string{}, err
	}
	out := map[string]string{}
	for _, f := range st.ChangedFiles {
		out[f] = "1"
	}
	return out, nil
}

func diffPaths(before, after map[string]string) []string {
	var out []string
	for k := range after {
		if before[k] == "" {
			out = append(out, k)
		}
	}
	for k := range before {
		if after[k] == "" {
			out = append(out, k)
		}
	}
	return out
}

// Ensure time used for future extensions
var _ = time.Second

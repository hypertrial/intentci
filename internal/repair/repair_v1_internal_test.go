package repair

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

func repairRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, arguments := range [][]string{
		{"init"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", "."}, {"commit", "-m", "base"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	return root
}

func failedRepairBundle(runID, attemptID string) *evidence.Bundle {
	return &evidence.Bundle{
		RunID: runID, AttemptID: attemptID,
		Run: verdict.RunResult{
			Verdict: verdict.Fail,
			Requirements: []verdict.RequirementResult{{
				ID: "REQ", Verdict: verdict.Fail,
				Obligations: []verdict.ObligationResult{{
					ID: "OBL", Verdict: verdict.Fail, Reason: "failed",
				}},
			}},
		},
	}
}

func TestBuildPacketEvidenceIntentAndFiltering(t *testing.T) {
	passed, failed := true, false
	bundle := failedRepairBundle("run", "attempt")
	bundle.Run.Requirements = append(bundle.Run.Requirements, verdict.RequirementResult{
		ID: "OTHER", Obligations: []verdict.ObligationResult{{ID: "O", Verdict: verdict.Fail}},
	})
	bundle.Run.Requirements[0].Obligations = append(bundle.Run.Requirements[0].Obligations,
		verdict.ObligationResult{ID: "PASS", Verdict: verdict.Pass},
		verdict.ObligationResult{ID: "SKIP", Verdict: verdict.Skipped},
	)
	bundle.Run.Requirements[0].Obligations[0].Evidence = []provider.Evidence{
		{ID: "b", Paths: []string{"z", "a"}, Passed: &failed},
		{ID: "a", Paths: []string{"a"}, Passed: &passed},
	}
	bundle.Document = &ir.Document{Requirements: []ir.Requirement{
		{ID: "REQ", Intent: "first", Boundaries: ir.Boundaries{Allowed: []string{"b", "a"}, Forbidden: []string{"z"}}},
		{ID: "REQ-2", Intent: "second", Boundaries: ir.Boundaries{Allowed: []string{"c"}}},
	}}
	packet := BuildPacket(bundle, 1, 2, "")
	if len(packet.Failures) != 2 || len(packet.Failures[0].EvidenceIDs) != 2 ||
		packet.Failures[0].EvidenceIDs[0] != "a" || packet.Failures[0].Paths[0] != "a" ||
		packet.Intent != "REQ: first\n\nREQ-2: second" || len(packet.AllowedPaths) != 3 {
		t.Fatalf("%+v", packet)
	}
}

func TestRunInterruptedReviewAndFinalizeFailure(t *testing.T) {
	root := repairRepo(t)
	store, err := evidence.NewStore(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	outcome, err := Run(cancelled, Options{
		Root: root, Config: cfg, Store: store, MaxAttempts: 2,
		Verify: func(context.Context) (*evidence.Bundle, error) {
			return failedRepairBundle("cancelled", "attempt"), nil
		},
	})
	if err != nil || outcome.Stopped != "interrupted" || outcome.Bundle.Run.Verdict != verdict.Error {
		t.Fatalf("%+v %v", outcome, err)
	}

	outcome, err = Run(context.Background(), Options{
		Root: root, Config: cfg, Store: store, MaxAttempts: 2,
		Verify: func(context.Context) (*evidence.Bundle, error) {
			bundle := failedRepairBundle("interrupted", "attempt")
			bundle.Interrupted = true
			return bundle, nil
		},
	})
	if err != nil || outcome.Stopped != "interrupted" {
		t.Fatalf("%+v %v", outcome, err)
	}
	outcome, err = Run(context.Background(), Options{
		Root: root, Config: cfg, Store: store,
		Verify: func(context.Context) (*evidence.Bundle, error) {
			return &evidence.Bundle{RunID: "review", Run: verdict.RunResult{Verdict: verdict.ReviewRequired}}, nil
		},
	})
	if err != nil || outcome.Stopped != "review_required" {
		t.Fatalf("%+v %v", outcome, err)
	}
	outcome, err = Run(context.Background(), Options{
		Root: root, Config: cfg, Store: store,
		Verify: func(context.Context) (*evidence.Bundle, error) {
			return &evidence.Bundle{RunID: "pass", Run: verdict.RunResult{Verdict: verdict.Pass}}, nil
		},
		Finalize: func(*evidence.Bundle) error { return errors.New("finalize") },
	})
	if err == nil || outcome.ExitCode != exitcode.Internal {
		t.Fatalf("%+v %v", outcome, err)
	}
}

func TestRunCaptureAndStoreStageFailures(t *testing.T) {
	oldPatch := takePatch
	defer func() { takePatch = oldPatch }()
	for failureCall := 1; failureCall <= 2; failureCall++ {
		root := repairRepo(t)
		store, _ := evidence.NewStore(root, t.TempDir())
		calls := 0
		takePatch = func(root, storeRoot string) ([]byte, error) {
			calls++
			if calls == failureCall {
				return nil, errors.New("patch")
			}
			return oldPatch(root, storeRoot)
		}
		outcome, err := Run(context.Background(), Options{
			Root: root, Config: config.Default(), Store: store, MaxAttempts: 2, AgentCommand: "true",
			Verify: func(context.Context) (*evidence.Bundle, error) {
				return failedRepairBundle("patch-run", "attempt-001"), nil
			},
		})
		if err == nil || outcome.ExitCode != exitcode.Internal {
			t.Fatalf("call=%d outcome=%+v err=%v", failureCall, outcome, err)
		}
	}
	takePatch = oldPatch

	stages := []struct {
		name    string
		prepare func(*evidence.Store) error
		command string
	}{
		{"patch-before", func(store *evidence.Store) error {
			return store.WriteRepairArtifact("run", "attempt-001", "patch-before.diff", []byte("conflict"))
		}, "true"},
		{"stdout", func(store *evidence.Store) error {
			return store.WriteAgentLog("run", "attempt-001", "stdout", []byte("conflict"))
		}, "printf actual"},
		{"stderr", func(store *evidence.Store) error {
			return store.WriteAgentLog("run", "attempt-001", "stderr", []byte("conflict"))
		}, "printf actual >&2"},
		{"agent-exit", func(store *evidence.Store) error {
			return store.WriteRepairArtifact("run", "attempt-001", "agent-exit.json", []byte("conflict"))
		}, "true"},
		{"patch-after", func(store *evidence.Store) error {
			return store.WriteRepairArtifact("run", "attempt-001", "patch-after.diff", []byte("conflict"))
		}, "true"},
	}
	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			root := repairRepo(t)
			store, _ := evidence.NewStore(root, t.TempDir())
			if err := stage.prepare(store); err != nil {
				t.Fatal(err)
			}
			outcome, err := Run(context.Background(), Options{
				Root: root, Config: config.Default(), Store: store, MaxAttempts: 2,
				AgentCommand: stage.command,
				Verify: func(context.Context) (*evidence.Bundle, error) {
					return failedRepairBundle("run", "attempt-001"), nil
				},
			})
			if err == nil || outcome.ExitCode != exitcode.Internal {
				t.Fatalf("%+v %v", outcome, err)
			}
		})
	}
}

func TestRunSnapshotStageFailures(t *testing.T) {
	oldSnapshot := takeSnapshot
	defer func() { takeSnapshot = oldSnapshot }()
	for failureCall := 1; failureCall <= 2; failureCall++ {
		root := repairRepo(t)
		store, _ := evidence.NewStore(root, t.TempDir())
		calls := 0
		takeSnapshot = func(root string) (map[string]string, error) {
			calls++
			if calls == failureCall {
				return nil, errors.New("snapshot")
			}
			return oldSnapshot(root)
		}
		outcome, err := Run(context.Background(), Options{
			Root: root, Config: config.Default(), Store: store, MaxAttempts: 2, AgentCommand: "true",
			Verify: func(context.Context) (*evidence.Bundle, error) {
				return failedRepairBundle("snapshot-run", "attempt-001"), nil
			},
		})
		if err == nil || outcome.ExitCode != exitcode.Internal {
			t.Fatalf("call=%d outcome=%+v err=%v", failureCall, outcome, err)
		}
	}
}

func TestRunAgentErrorsAndCancellation(t *testing.T) {
	root := repairRepo(t)
	store, _ := evidence.NewStore(root, t.TempDir())
	cfg := config.Default()
	cfg.Repair.StopOnRepeatedDiff = false
	cfg.Repair.StopOnRepeatedFailure = false
	attempt := 0
	outcome, err := Run(context.Background(), Options{
		Root: root, Config: cfg, Store: store, MaxAttempts: 3, AgentCommand: "exit 7",
		Verify: func(context.Context) (*evidence.Bundle, error) {
			attempt++
			return failedRepairBundle("errors", "attempt-00"+string(rune('0'+attempt))), nil
		},
	})
	if err != nil || outcome.Stopped != "repeated_agent_error" {
		t.Fatalf("%+v %v", outcome, err)
	}
	if agentExitCode(errors.New("plain")) != -1 {
		t.Fatal("plain error had process exit code")
	}
	command := exec.Command("sh", "-c", "exit 9")
	processErr := command.Run()
	if agentExitCode(processErr) != 9 {
		t.Fatal(processErr)
	}

	root = repairRepo(t)
	store, _ = evidence.NewStore(root, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	marker := filepath.Join(root, "agent-started")
	go func() {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(marker); err == nil {
				cancel()
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	outcome, err = Run(ctx, Options{
		Root: root, Config: cfg, Store: store, MaxAttempts: 2,
		AgentCommand: "touch agent-started; sleep 2",
		Verify: func(context.Context) (*evidence.Bundle, error) {
			return failedRepairBundle("cancel-agent", "attempt-001"), nil
		},
	})
	if err != nil || outcome.Stopped != "interrupted" || !outcome.Bundle.Interrupted {
		t.Fatalf("%+v %v", outcome, err)
	}
}

func TestCapturePatchFailuresAndUntrackedContent(t *testing.T) {
	oldOutput := gitOutput
	defer func() { gitOutput = oldOutput }()
	gitOutput = func(string, ...string) ([]byte, error) { return nil, errors.New("tracked") }
	if _, err := capturePatch(t.TempDir(), ""); err == nil {
		t.Fatal("tracked diff error ignored")
	}
	calls := 0
	gitOutput = func(string, ...string) ([]byte, error) {
		calls++
		if calls == 2 {
			return nil, errors.New("untracked")
		}
		return nil, nil
	}
	if _, err := capturePatch(t.TempDir(), ""); err == nil {
		t.Fatal("untracked listing error ignored")
	}
	root := t.TempDir()
	calls = 0
	gitOutput = func(string, ...string) ([]byte, error) {
		calls++
		if calls%2 == 0 {
			return []byte("missing\n"), nil
		}
		return nil, nil
	}
	if _, err := capturePatch(root, ""); err == nil {
		t.Fatal("untracked read error ignored")
	}
	if err := os.WriteFile(filepath.Join(root, "plain"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls = 0
	gitOutput = func(string, ...string) ([]byte, error) {
		calls++
		if calls%2 == 0 {
			return []byte("\nplain\n"), nil
		}
		return []byte("tracked"), nil
	}
	raw, err := capturePatch(root, filepath.Join(root, "store"))
	if err != nil || !strings.Contains(string(raw), "intentci-untracked plain") ||
		!strings.HasSuffix(string(raw), "\n") {
		t.Fatalf("%q %v", raw, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "store"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "store", "hidden"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "empty"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	calls = 0
	gitOutput = func(string, ...string) ([]byte, error) {
		calls++
		if calls%2 == 0 {
			return []byte("store/hidden\nempty\nplain\n"), nil
		}
		return nil, nil
	}
	raw, err = capturePatch(root, filepath.Join(root, "store"))
	if err != nil || strings.Contains(string(raw), "store/hidden") ||
		!strings.Contains(string(raw), "intentci-untracked empty") {
		t.Fatalf("%q %v", raw, err)
	}
	if hashBytes([]byte("x")) == "" {
		t.Fatal("empty hash")
	}
}

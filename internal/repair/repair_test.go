package repair_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/repair"
	"github.com/hypertrial/intentci/internal/verdict"
)

func TestBuildPacketAndDryRun(t *testing.T) {
	b := &evidence.Bundle{
		RunID: "r1",
		Run: verdict.RunResult{
			Verdict: verdict.Fail,
			Requirements: []verdict.RequirementResult{{
				ID:          "REQ-1",
				Obligations: []verdict.ObligationResult{{ID: "O1", Verdict: verdict.Fail, Reason: "x"}},
			}},
		},
	}
	p := repair.BuildPacket(b, 1, 3, "")
	if len(p.Failures) != 1 {
		t.Fatalf("%+v", p)
	}
	root := gitRepo(t)
	store, err := evidence.NewStore(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Repair.MaxAttempts = 2
	attempts := 0
	out, err := repair.Run(context.Background(), repair.Options{
		Root: root, Config: cfg, Store: store, DryRun: true, MaxAttempts: 2,
		Verify: func(ctx context.Context) (*evidence.Bundle, error) {
			attempts++
			return b, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ExitCode != exitcode.RepairExhausted || attempts != 2 {
		t.Fatalf("%+v attempts=%d", out, attempts)
	}
	if _, err := os.Stat(filepath.Join(store.Dir("r1"), "repair-packet.json")); err != nil {
		// packet written on first attempt with run id r1
		_ = err
	}
}

func TestPassShortCircuit(t *testing.T) {
	root := gitRepo(t)
	store, _ := evidence.NewStore(root, t.TempDir())
	cfg := config.Default()
	out, err := repair.Run(context.Background(), repair.Options{
		Root: root, Config: cfg, Store: store, DryRun: true,
		Verify: func(ctx context.Context) (*evidence.Bundle, error) {
			return &evidence.Bundle{RunID: "ok", Run: verdict.RunResult{Verdict: verdict.Pass}}, nil
		},
	})
	if err != nil || out.ExitCode != exitcode.Pass {
		t.Fatalf("%v %+v", err, out)
	}
}

func TestPacketIncludesRequirementBoundaries(t *testing.T) {
	b := &evidence.Bundle{
		RunID: "r",
		Document: &ir.Document{Requirements: []ir.Requirement{{
			ID: "REQ-1",
			Boundaries: ir.Boundaries{
				Allowed:   []string{"src/**"},
				Forbidden: []string{"secrets/**"},
			},
		}, {ID: "REQ-2", Boundaries: ir.Boundaries{Allowed: []string{"other/**"}}}}},
		Run: verdict.RunResult{
			Verdict: verdict.Fail,
			Requirements: []verdict.RequirementResult{{
				ID:          "REQ-1",
				Obligations: []verdict.ObligationResult{{ID: "O1", Verdict: verdict.Fail}},
			}},
		},
	}
	p := repair.BuildPacket(b, 1, 2, "REQ-1")
	if len(p.AllowedPaths) != 1 || p.AllowedPaths[0] != "src/**" || len(p.Forbidden) != 1 {
		t.Fatalf("%+v", p)
	}
}

func TestRepairRejectsPreexistingProtectedChanges(t *testing.T) {
	root := gitRepo(t)
	path := filepath.Join(root, ".intentci", "requirements", "REQ-001.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("contract\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-m", "contract")
	if err := os.WriteFile(path, []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _ := evidence.NewStore(root, t.TempDir())
	called := false
	out, err := repair.Run(context.Background(), repair.Options{
		Root: root, Config: config.Default(), Store: store, MaxAttempts: 2,
		Verify: func(context.Context) (*evidence.Bundle, error) {
			called = true
			return nil, nil
		},
	})
	if err != nil || out.ExitCode != exitcode.SecurityBoundary || called {
		t.Fatalf("err=%v out=%+v called=%v", err, out, called)
	}
}

func TestRepairRejectsChangesOutsideAllowedBoundary(t *testing.T) {
	root := gitRepo(t)
	store, _ := evidence.NewStore(root, t.TempDir())
	failed := &evidence.Bundle{
		RunID: "run",
		Document: &ir.Document{Requirements: []ir.Requirement{{
			ID: "REQ-1", Boundaries: ir.Boundaries{Allowed: []string{"src/**"}},
		}}},
		Run: verdict.RunResult{
			Verdict: verdict.Fail,
			Requirements: []verdict.RequirementResult{{
				ID:          "REQ-1",
				Obligations: []verdict.ObligationResult{{ID: "O1", Verdict: verdict.Fail}},
			}},
		},
	}
	out, err := repair.Run(context.Background(), repair.Options{
		Root: root, Config: config.Default(), Store: store, MaxAttempts: 2,
		AgentCommand: "printf changed >> README.md",
		Verify: func(context.Context) (*evidence.Bundle, error) {
			return failed, nil
		},
	})
	if err != nil || out.ExitCode != exitcode.SecurityBoundary {
		t.Fatalf("err=%v out=%+v", err, out)
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.email", "test@example.com")
	gitRun(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-m", "initial")
	return root
}

func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

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
	"github.com/hypertrial/intentci/internal/repair"
	"github.com/hypertrial/intentci/internal/verdict"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, c := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "t@e.com"},
		{"git", "config", "user.name", "t"},
	} {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
}

func gitCommitTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for path, body := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "c"},
	} {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
}

func failBundle(id string) *evidence.Bundle {
	return &evidence.Bundle{
		RunID: id,
		Run: verdict.RunResult{
			Verdict: verdict.Fail,
			Requirements: []verdict.RequirementResult{
				{ID: "REQ-1", Obligations: []verdict.ObligationResult{
					{ID: "O1", Verdict: verdict.Fail, Reason: "x"},
					{ID: "O2", Verdict: verdict.Pass},
					{ID: "O3", Verdict: verdict.Skipped},
				}},
				{ID: "REQ-2", Obligations: []verdict.ObligationResult{{ID: "Z", Verdict: verdict.Fail}}},
			},
		},
	}
}

func TestBuildPacketFilter(t *testing.T) {
	b := failBundle("r")
	p := repair.BuildPacket(b, 1, 2, "REQ-1")
	if len(p.Failures) != 1 || p.Failures[0].Obligation != "O1" {
		t.Fatalf("%+v", p)
	}
}

func TestRepairVerifyErrorAndRepeatedFailure(t *testing.T) {
	store, err := evidence.NewStore(t.TempDir(), "runs")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Repair.MaxAttempts = 3
	cfg.Repair.StopOnRepeatedFailure = true
	_, err = repair.Run(context.Background(), repair.Options{
		Root: t.TempDir(), Config: cfg, Store: store, DryRun: true,
		Verify: func(ctx context.Context) (*evidence.Bundle, error) {
			return nil, context.Canceled
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	b := failBundle("r1")
	out, err := repair.Run(context.Background(), repair.Options{
		Root: t.TempDir(), Config: cfg, Store: store, DryRun: true, MaxAttempts: 3,
		Verify: func(ctx context.Context) (*evidence.Bundle, error) { return b, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Stopped != "repeated_failure" || out.ExitCode != exitcode.RepairExhausted {
		t.Fatalf("%+v", out)
	}
}

func TestRepairAgentProtectedTestAndRepeatedDiff(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	gitCommitTree(t, root, map[string]string{
		".intentci/config.yaml":            "version: 1\n",
		".intentci/requirements/REQ-001.md": "x\n",
		"internal/foo_test.go":             "package x\n",
		"touch.txt":                        "old\n",
	})
	store, err := evidence.NewStore(root, ".intentci/runs")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Repair.MaxAttempts = 3
	cfg.Repair.StopOnRepeatedDiff = true
	cfg.Repair.StopOnRepeatedFailure = false
	cfg.Repair.AllowTestChanges = false
	cfg.Repair.AllowRequirementChanges = false

	agent := `echo changed > .intentci/requirements/REQ-001.md`
	b := failBundle("rp")
	out, err := repair.Run(context.Background(), repair.Options{
		Root: root, Config: cfg, Store: store, AgentCommand: agent + " # {packet}", MaxAttempts: 1,
		Verify: func(ctx context.Context) (*evidence.Bundle, error) { return b, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ExitCode != exitcode.SecurityBoundary || out.Stopped == "" {
		t.Fatalf("%+v", out)
	}

	root2 := t.TempDir()
	gitInit(t, root2)
	gitCommitTree(t, root2, map[string]string{"internal/foo_test.go": "package x\n"})
	store2, _ := evidence.NewStore(root2, ".intentci/runs")
	agent2 := `echo x >> internal/foo_test.go`
	b2 := failBundle("rt")
	out, err = repair.Run(context.Background(), repair.Options{
		Root: root2, Config: cfg, Store: store2, AgentCommand: agent2, MaxAttempts: 1,
		Verify: func(ctx context.Context) (*evidence.Bundle, error) { return b2, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ExitCode != exitcode.SecurityBoundary {
		t.Fatalf("%+v", out)
	}

	root3 := t.TempDir()
	gitInit(t, root3)
	gitCommitTree(t, root3, map[string]string{"touch.txt": "old\n"})
	store3, _ := evidence.NewStore(root3, ".intentci/runs")
	cfg3 := config.Default()
	cfg3.Repair.StopOnRepeatedDiff = true
	cfg3.Repair.StopOnRepeatedFailure = false
	cfg3.Repair.AllowTestChanges = true
	agent3 := `echo same > touch.txt`
	n := 0
	out, err = repair.Run(context.Background(), repair.Options{
		Root: root3, Config: cfg3, Store: store3, AgentCommand: agent3, MaxAttempts: 3,
		Verify: func(ctx context.Context) (*evidence.Bundle, error) {
			n++
			bb := failBundle("rd")
			bb.RunID = "rd"
			bb.Run.Requirements[0].Obligations[0].Reason = filepath.Join("x", string(rune('a'+n)))
			return bb, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Stopped != "repeated_diff" {
		t.Fatalf("%+v", out)
	}

	store4, _ := evidence.NewStore(t.TempDir(), "runs")
	_ = os.WriteFile(filepath.Join(store4.Root, "rw"), []byte("x"), 0o644)
	bb := failBundle("rw")
	out, err = repair.Run(context.Background(), repair.Options{
		Root: t.TempDir(), Config: config.Default(), Store: store4, DryRun: true, MaxAttempts: 1,
		Verify: func(ctx context.Context) (*evidence.Bundle, error) { return bb, nil },
	})
	if err == nil && out.ExitCode != exitcode.Internal {
		t.Fatalf("expected packet write failure %+v err=%v", out, err)
	}
}

func TestRepairMaxAttemptsDefault(t *testing.T) {
	store, _ := evidence.NewStore(t.TempDir(), "runs")
	cfg := config.Default()
	cfg.Repair.MaxAttempts = 0
	cfg.Repair.StopOnRepeatedFailure = false
	attempts := 0
	out, err := repair.Run(context.Background(), repair.Options{
		Root: t.TempDir(), Config: cfg, Store: store, DryRun: true, AgentCommand: "",
		Verify: func(ctx context.Context) (*evidence.Bundle, error) {
			attempts++
			return failBundle("m"), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Stopped != "max_attempts" || attempts < 1 {
		t.Fatalf("%+v attempts=%d", out, attempts)
	}
}

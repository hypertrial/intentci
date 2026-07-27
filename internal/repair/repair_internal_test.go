package repair

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/verdict"
)

func TestHelpers(t *testing.T) {
	if hashStrings([]string{"a", "b"}) == "" {
		t.Fatal("hash")
	}
	before := map[string]string{"a": "1", "b": "1"}
	after := map[string]string{"b": "1", "c": "1"}
	got := diffPaths(before, after)
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Fatalf("%v", got)
	}
	// snapshotDiff error path (not a git repo)
	m, err := snapshotDiff(t.TempDir())
	if err == nil || len(m) != 0 {
		t.Fatalf("err=%v m=%v", err, m)
	}
	if contains([]string{"x"}, "y") || !contains([]string{"x"}, "x") {
		t.Fatal("contains")
	}
	if diffFingerprint(before, after) == "" {
		t.Fatal("diff fingerprint")
	}
	if got := uniqueSorted([]string{"b", "a", "a"}); len(got) != 2 || got[0] != "a" {
		t.Fatal(got)
	}
	snapshot := map[string]string{"runs/a": "1", "keep": "2"}
	ignoreStoreFiles(snapshot, "/repo", "/repo/runs")
	if _, ok := snapshot["runs/a"]; ok {
		t.Fatal(snapshot)
	}
	ignoreStoreFiles(snapshot, "/repo", "/outside")

	if got := storePrefix("/repo", "/repo/runs"); got != "runs" {
		t.Fatalf("child store prefix = %q", got)
	}
	for name, storeRoot := range map[string]string{
		"same":    "/repo",
		"parent":  "/",
		"sibling": "/other",
	} {
		t.Run("store-prefix-"+name, func(t *testing.T) {
			if got := storePrefix("/repo", storeRoot); got != "" {
				t.Fatalf("store prefix = %q", got)
			}
		})
	}
	oldRelativePath := relativePath
	relativePath = func(string, string) (string, error) { return "", errors.New("relative") }
	if got := storePrefix("/repo", "/repo/runs"); got != "" {
		t.Fatalf("errored store prefix = %q", got)
	}
	relativePath = oldRelativePath

	root := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	empty, err := snapshotDiff(root)
	if err != nil || len(empty) != 0 {
		t.Fatalf("%v %v", empty, err)
	}
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "link")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "add", "target", "link")
	add.Dir = root
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	files, err := snapshotDiff(root)
	if err != nil || len(files) != 2 {
		t.Fatalf("%v %v", files, err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	files, err = snapshotDiff(root)
	if err != nil || files["target"] == "" {
		t.Fatalf("%v %v", files, err)
	}
}

func TestRunSnapshotErrors(t *testing.T) {
	nonGit := t.TempDir()
	store, _ := evidence.NewStore(nonGit, t.TempDir())
	out, err := Run(context.Background(), Options{
		Root: nonGit, Config: config.Default(), Store: store,
		Verify: func(context.Context) (*evidence.Bundle, error) { return nil, nil },
	})
	if err == nil || out.ExitCode != exitcode.Internal {
		t.Fatalf("%+v %v", out, err)
	}

	root := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "initial"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, output)
		}
	}
	store, _ = evidence.NewStore(root, t.TempDir())
	failed := &evidence.Bundle{RunID: "r", Run: verdict.RunResult{Verdict: verdict.Fail}}
	old := takeSnapshot
	defer func() { takeSnapshot = old }()
	takeSnapshot = func(string) (map[string]string, error) { return nil, errors.New("snapshot") }
	out, err = Run(context.Background(), Options{
		Root: root, Config: config.Default(), Store: store, MaxAttempts: 2, AgentCommand: "true",
		Verify: func(context.Context) (*evidence.Bundle, error) { return failed, nil },
	})
	if err == nil || out.ExitCode != exitcode.Internal {
		t.Fatalf("%+v %v", out, err)
	}

	calls := 0
	takeSnapshot = func(string) (map[string]string, error) {
		calls++
		if calls == 2 {
			return nil, errors.New("snapshot")
		}
		return map[string]string{"README.md": "x"}, nil
	}
	out, err = Run(context.Background(), Options{
		Root: root, Config: config.Default(), Store: store, MaxAttempts: 2, AgentCommand: "true",
		Verify: func(context.Context) (*evidence.Bundle, error) { return failed, nil },
	})
	if err == nil || out.ExitCode != exitcode.Internal {
		t.Fatalf("%+v %v", out, err)
	}
}

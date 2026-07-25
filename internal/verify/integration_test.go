package verify_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/verify"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestFixtureGoService_PassAndFail(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	src := filepath.Join("..", "..", "fixtures", "go-service")
	src, err := filepath.Abs(src)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	copyTree(t, src, dir)
	gitInit(t, dir)

	// Baseline commit with correct code.
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "-c", "user.email=test@example.com", "-c", "user.name=test", "commit", "-m", "base")

	outcome, err := verify.Run(context.Background(), verify.Options{
		Root:    dir,
		Base:    "HEAD",
		Profile: "full",
		All:     true,
		Trust:   true,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	})
	if err != nil {
		t.Fatalf("verify correct tree: %v", err)
	}
	if outcome.ExitCode != 0 || outcome.Result.Status != protocol.StatusPass {
		t.Fatalf("expected pass, got status=%s exit=%d", outcome.Result.Status, outcome.ExitCode)
	}

	// Apply incorrect patch and commit.
	broken, err := os.ReadFile(filepath.Join(src, "patches", "incorrect", "counter.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "counter", "counter.go"), broken, 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "-c", "user.email=test@example.com", "-c", "user.name=test", "commit", "-m", "break add")

	outcome, err = verify.Run(context.Background(), verify.Options{
		Root:    dir,
		Base:    "HEAD~1",
		Profile: "full",
		All:     false,
		Trust:   true,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	})
	if err != nil {
		t.Fatalf("verify broken tree: %v", err)
	}
	if outcome.ExitCode != 10 || outcome.Result.Status != protocol.StatusFail {
		t.Fatalf("expected fail/10, got status=%s exit=%d summary=%+v", outcome.Result.Status, outcome.ExitCode, outcome.Result.Summary)
	}
	found := false
	for _, r := range outcome.Result.Requirements {
		if r.ID == "MATH-001" && r.Status == protocol.ReqFail {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected MATH-001 fail, got %#v", outcome.Result.Requirements)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	run(t, dir, "git", "init")
	run(t, dir, "git", "checkout", "-b", "main")
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "patches" || strings.HasPrefix(rel, "patches"+string(os.PathSeparator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
	if err != nil {
		t.Fatal(err)
	}
}

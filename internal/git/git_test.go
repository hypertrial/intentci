package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/git"
)

func TestResolve_MissingBase(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "init")

	_, err := git.Resolve(dir, "origin/main")
	if err == nil || !strings.Contains(err.Error(), "missing base reference") {
		t.Fatalf("expected missing base error, got %v", err)
	}
}

func TestResolve_DirtyWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := git.Resolve(dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !st.WorkingTreeDirty {
		t.Fatal("expected dirty worktree")
	}
	found := false
	for _, f := range st.ChangedFiles {
		if f == "b.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected b.txt in changed files, got %#v", st.ChangedFiles)
	}
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

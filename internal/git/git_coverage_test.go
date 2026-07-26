package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/git"
)

func TestResolveDirtyRenameAndErrors(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "t@e.com")
	run("git", "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-m", "c1")
	run("git", "branch", "base")

	// modify tracked + untracked + rename-like status
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "new.txt")
	run("git", "mv", "new.txt", "renamed.txt")

	st, err := git.Resolve(dir, "base")
	if err != nil {
		t.Fatal(err)
	}
	if !st.WorkingTreeDirty || len(st.ChangedFiles) == 0 {
		t.Fatalf("%+v", st)
	}
	// empty baseRef uses origin/main which should fail
	if _, err := git.Resolve(dir, ""); err == nil {
		t.Fatal("expected missing origin/main")
	}
	if _, err := git.Resolve(dir, "does-not-exist"); err == nil {
		t.Fatal("expected missing ref")
	}
}

func TestResolveNotRepo(t *testing.T) {
	if _, err := git.Resolve(t.TempDir(), "HEAD"); err == nil {
		t.Fatal("expected error")
	}
}

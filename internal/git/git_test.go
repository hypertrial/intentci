package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/git"
)

func TestResolveAndIsRepo(t *testing.T) {
	dir := t.TempDir()
	if git.IsRepo(dir) {
		t.Fatal("expected not repo")
	}
	run := func(args ...string) {
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
	st, err := git.Resolve(dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if st.HeadCommit == "" {
		t.Fatal("empty head")
	}
}

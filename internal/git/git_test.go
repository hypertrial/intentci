package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/git"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "git", "-c", "core.hooksPath=/dev/null", "init")
	runGit(t, dir, "git", "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "git", "add", ".")
	runGit(t, dir, "git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestResolve_MissingBase(t *testing.T) {
	dir := gitRepo(t)
	_, err := git.Resolve(dir, "origin/main")
	if err == nil || !strings.Contains(err.Error(), "missing base reference") {
		t.Fatalf("got %v", err)
	}
}

func TestResolve_DirtyWorktree(t *testing.T) {
	dir := gitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := git.Resolve(dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !st.WorkingTreeDirty {
		t.Fatal("dirty")
	}
	found := false
	for _, f := range st.ChangedFiles {
		if f == "b.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%#v", st.ChangedFiles)
	}
}

func TestResolve_CommittedAndNotRepo(t *testing.T) {
	dir := gitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "git", "add", ".")
	runGit(t, dir, "git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "second")
	st, err := git.Resolve(dir, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if st.HeadCommit == "" || st.MergeBase == "" || st.MergeBaseFull == "" {
		t.Fatalf("%+v", st)
	}
	if len(st.MergeBaseFull) < len(st.MergeBase) {
		t.Fatalf("MergeBaseFull should be full SHA: %+v", st)
	}
	if _, err := git.Resolve(t.TempDir(), "HEAD"); err == nil {
		t.Fatal("not a repo")
	}
}

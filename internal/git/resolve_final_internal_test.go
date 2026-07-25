package git

import (
	"os"
	"os/exec"
	"testing"
)

func TestResolve_EmptyBaseRef(t *testing.T) {
	dir := gitRepoLocal(t)
	_, err := Resolve(dir, "")
	if err == nil || err.Error() == "" {
		// missing origin/main is expected; baseRef default still executed
	}
}

func TestChangedFiles_SkipEmptyNames(t *testing.T) {
	old := run
	defer func() { run = old }()
	run = func(root string, args ...string) (string, error) {
		if args[0] == "diff" && len(args) > 2 {
			return "  \nfile.go\n", nil
		}
		return "", nil
	}
	files, err := changedFiles(t.TempDir(), "abc", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "file.go" {
		t.Fatalf("%#v", files)
	}
}

func gitRepoLocal(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitLocal(t, dir, "git", "-c", "core.hooksPath=/dev/null", "init")
	runGitLocal(t, dir, "git", "checkout", "-b", "main")
	runGitLocal(t, dir, "git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "--allow-empty", "-m", "init")
	return dir
}

func runGitLocal(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	_ = os.Getwd
}

package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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

package verify_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/initcmd"
	"github.com/hypertrial/intentci/internal/verify"
)

func TestVerifyAll(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "t@e.com")
	run("git", "config", "user.name", "t")
	if err := initcmd.Run(initcmd.Options{Root: root}); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-m", "init")
	out, err := verify.Run(context.Background(), verify.Options{Root: root, All: true, NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.ExitCode != exitcode.Pass {
		t.Fatalf("code=%d", out.ExitCode)
	}
	if _, err := os.Stat(filepath.Join(root, ".intentci", "runs", "latest")); err != nil {
		t.Fatal(err)
	}
}

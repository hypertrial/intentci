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

func TestVerifyChangedCompileFailAndGitFallback(t *testing.T) {
	if _, err := verify.Run(context.Background(), verify.Options{Root: t.TempDir(), All: true}); err == nil {
		t.Fatal("missing config")
	}

	root := t.TempDir()
	if err := initcmd.Run(initcmd.Options{Root: root}); err != nil {
		t.Fatal(err)
	}
	out, err := verify.Run(context.Background(), verify.Options{Root: root, All: true, NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.ExitCode != exitcode.Pass {
		t.Fatalf("code=%d", out.ExitCode)
	}

	if _, err := verify.Run(context.Background(), verify.Options{Root: root, Changed: true}); err == nil {
		t.Fatal("expected git error")
	}

	out, err = verify.Run(context.Background(), verify.Options{Root: root, RequirementID: "REQ-001", ObligationID: "OBL-001", NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.Bundle == nil {
		t.Fatal("nil bundle")
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "t@e.com")
	run("git", "config", "user.name", "t")
	run("git", "add", ".")
	run("git", "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(root, "orphan.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = verify.Run(context.Background(), verify.Options{Root: root, Changed: true, Base: "HEAD", NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	_ = out

	if err := os.WriteFile(filepath.Join(root, ".intentci", "requirements", "REQ-001.md"), []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := verify.Run(context.Background(), verify.Options{Root: root, All: true}); err == nil {
		t.Fatal("expected compile fail")
	}
}

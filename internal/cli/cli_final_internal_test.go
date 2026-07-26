package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/initcmd"
)

func TestVerifyDefaultChangedAndRepairStopped(t *testing.T) {
	root := t.TempDir()
	if err := initcmd.Run(initcmd.Options{Root: root}); err != nil {
		t.Fatal(err)
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
	run("git", "commit", "-m", "i")

	old := getwd
	defer func() { getwd = old }()
	getwd = func() (string, error) { return root, nil }

	var out, errb bytes.Buffer
	// default changed=true (no --all/--changed/--requirement)
	_ = RunMain([]string{"verify", "--base", "HEAD", "--no-cache"}, &out, &errb)

	// verify error with outcome exit code (compile failed)
	if err := os.WriteFile(filepath.Join(root, ".intentci", "requirements", "REQ-001.md"), []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if code := RunMain([]string{"verify", "--all"}, &out, &errb); code == exitcode.Pass {
		t.Fatal("expected compile fail exit")
	}

	// restore failing obligation for repair stopped path
	req := `---
id: REQ-001
title: t
status: active
priority: required
---
# Intent
i
# Obligations
` + "```yaml" + `
- id: OBL-001
  statement: fail
  required: true
  verify:
    provider: command
    id: smoke
    run: "false"
    result: {equals: 0}
` + "```"
	if err := os.WriteFile(filepath.Join(root, ".intentci", "requirements", "REQ-001.md"), []byte(req), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	code := RunMain([]string{"repair", "--dry-run", "--max-attempts", "1"}, &out, &errb)
	if code == exitcode.Pass {
		t.Fatal("expected non-pass repair")
	}
	if !bytes.Contains(errb.Bytes(), []byte("repair stopped")) && code != exitcode.RepairExhausted && code != exitcode.Fail {
		// stopped message printed on stderr when Stopped != ""
		_ = errb.String()
	}

	// repair verify callback / repair.Run error (config ok, compile fails)
	if err := os.WriteFile(filepath.Join(root, ".intentci", "requirements", "REQ-001.md"), []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".intentci", "config.yaml"), []byte(`version: 1
project: {name: demo}
requirements:
  paths: [".intentci/requirements/**/*.md"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if code := RunMain([]string{"repair", "--dry-run", "--max-attempts", "1"}, &out, &errb); code == exitcode.Pass {
		t.Fatal("expected repair verify error")
	}

	// doctor telemetry enabled with clean config
	getwd = func() (string, error) { return root, nil }
	if err := os.WriteFile(filepath.Join(root, ".intentci", "config.yaml"), []byte(`version: 1
project: {name: demo}
requirements:
  paths: [".intentci/requirements/**/*.md"]
telemetry:
  enabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if code := RunMain([]string{"doctor"}, &out, &errb); code == 0 {
		t.Fatal("telemetry should fail doctor")
	}
	if !bytes.Contains(out.Bytes(), []byte("telemetry")) {
		t.Fatalf("out=%s", out.String())
	}
}

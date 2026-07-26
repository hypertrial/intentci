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

func TestCLIErrorBranches(t *testing.T) {
	root := t.TempDir()
	if err := initcmd.Run(initcmd.Options{Root: root}); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	runGit("git", "init")
	runGit("git", "config", "user.email", "t@e.com")
	runGit("git", "config", "user.name", "t")
	runGit("git", "add", ".")
	runGit("git", "commit", "-m", "i")

	old := getwd
	defer func() { getwd = old }()
	getwd = func() (string, error) { return root, nil }

	var out, errb bytes.Buffer
	bad := filepath.Join(root, ".intentci", "requirements", "REQ-001.md")
	if err := os.WriteFile(bad, []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := RunMain([]string{"compile", "--strict"}, &out, &errb); code == 0 {
		t.Fatal("compile should fail")
	}
	if err := initcmd.Run(initcmd.Options{Root: root, Force: true}); err != nil {
		t.Fatal(err)
	}

	block := filepath.Join(root, "block")
	if err := os.WriteFile(block, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if code := RunMain([]string{"compile", "--output", filepath.Join(block, "ir.json")}, &out, &errb); code == 0 {
		t.Fatal("writeir")
	}

	out.Reset()
	errb.Reset()
	if code := RunMain([]string{"verify", "--all", "--output", filepath.Join(block, "out.json")}, &out, &errb); code == 0 {
		t.Fatal("output create")
	}

	req := `---
id: REQ-001
title: Example requirement
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
	if code := RunMain([]string{"verify", "--all", "--format", "text", "--no-cache"}, &out, &errb); code == exitcode.Pass {
		t.Fatal("expected fail verdict")
	}

	out.Reset()
	errb.Reset()
	_ = RunMain([]string{"verify", "--all", "--format", "nope", "--no-cache"}, &out, &errb)

	out.Reset()
	errb.Reset()
	_ = RunMain([]string{"explain", "NOPE"}, &out, &errb)

	empty := t.TempDir()
	getwd = func() (string, error) { return empty, nil }
	out.Reset()
	errb.Reset()
	_ = RunMain([]string{"repair", "--dry-run", "--max-attempts", "1"}, &out, &errb)

	getwd = func() (string, error) { return root, nil }
	cfgPath := filepath.Join(root, ".intentci", "config.yaml")
	body, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, append(body, []byte("\ntelemetry:\n  enabled: true\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	oldGOOS := goos
	goos = "windows"
	defer func() { goos = oldGOOS }()
	out.Reset()
	errb.Reset()
	if code := RunMain([]string{"doctor"}, &out, &errb); code == 0 {
		t.Fatal("doctor should fail")
	}

	goos = "darwin"
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".intentci", "config.yaml"), []byte(`version: 1
project: {name: d}
requirements:
  paths: [".intentci/requirements/**/*.md"]
evidence:
  directory: runs
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "runs"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	getwd = func() (string, error) { return tmp, nil }
	out.Reset()
	errb.Reset()
	_ = RunMain([]string{"doctor"}, &out, &errb)
	out.Reset()
	errb.Reset()
	_ = RunMain([]string{"status"}, &out, &errb)
	out.Reset()
	errb.Reset()
	_ = RunMain([]string{"explain", "X"}, &out, &errb)
	out.Reset()
	errb.Reset()
	_ = RunMain([]string{"repair", "--dry-run"}, &out, &errb)

	getwd = func() (string, error) { return root, nil }
	out.Reset()
	errb.Reset()
	_ = RunMain([]string{"repair", "--dry-run", "--max-attempts", "1", "--changed"}, &out, &errb)

	// compile NewStore failure via evidence.directory file
	if err := initcmd.Run(initcmd.Options{Root: root, Force: true}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".intentci", "config.yaml"), []byte(`version: 1
project: {name: demo}
requirements:
  paths: [".intentci/requirements/**/*.md"]
evidence:
  directory: blocked
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "blocked"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	_ = RunMain([]string{"compile"}, &out, &errb)
}

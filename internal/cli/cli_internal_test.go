package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCLI_ErrorBranches(t *testing.T) {
	oldGetwd := getwd
	defer func() { getwd = oldGetwd }()
	getwd = func() (string, error) { return "", errors.New("getwd") }
	var out, errb bytes.Buffer
	if code := RunMain([]string{"init"}, &out, &errb); code != 1 {
		t.Fatalf("getwd init code=%d", code)
	}
	if code := RunMain([]string{"validate"}, &out, &errb); code != 1 {
		t.Fatalf("getwd validate code=%d", code)
	}
	if code := RunMain([]string{"change", "create", "X-1"}, &out, &errb); code != 1 {
		t.Fatalf("getwd change code=%d", code)
	}
	if code := RunMain([]string{"explain", "BUILD-001"}, &out, &errb); code != 1 {
		t.Fatalf("getwd explain code=%d", code)
	}
	if code := RunMain([]string{"verify"}, &out, &errb); code != 1 {
		t.Fatalf("getwd verify code=%d", code)
	}

	dir := t.TempDir()
	getwd = func() (string, error) { return dir, nil }
	out.Reset()
	errb.Reset()
	if code := RunMain([]string{"validate"}, &out, &errb); code != 21 {
		t.Fatalf("find root validate code=%d err=%s", code, errb.String())
	}
	if code := RunMain([]string{"change", "create", "X-1"}, &out, &errb); code != 21 {
		t.Fatalf("find root change code=%d", code)
	}
	if code := RunMain([]string{"explain", "BUILD-001"}, &out, &errb); code != 21 {
		t.Fatalf("find root explain code=%d", code)
	}

	gitDir := gitRepo(t)
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(gitDir)
	getwd = func() (string, error) { return gitDir, nil }
	out.Reset()
	errb.Reset()
	if code := RunMain([]string{"init"}, &out, &errb); code != 0 {
		t.Fatalf("init %d %s", code, errb.String())
	}
	if code := RunMain([]string{"init"}, &out, &errb); code == 0 {
		t.Fatal("expected init already exists")
	}

	body := `version: 1
product: {name: x, purpose: y}
policy: {default_base: HEAD}
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    applies_to: {include: ["**"]}
    verification: {checks: [go-test]}
checks:
  - id: go-test
    command: "true"
    profiles: [fast, full]
    inputs: ["**"]
    timeout: 1m
`
	os.WriteFile(filepath.Join(gitDir, ".intentci", "contract.yaml"), []byte(body), 0o644)
	out.Reset()
	if code := RunMain([]string{"explain", "NOPE", "--base", "HEAD"}, &out, &errb); code != 20 {
		t.Fatalf("explain fail code=%d", code)
	}
	out.Reset()
	if code := RunMain([]string{"verify", "--trust", "--base", "HEAD", "--output", filepath.Join(gitDir, "no", "out.json")}, &out, &errb); code != 30 {
		t.Fatalf("report write fail code=%d err=%s", code, errb.String())
	}
	os.RemoveAll(filepath.Join(gitDir, ".intentci", "changes"))
	if err := os.WriteFile(filepath.Join(gitDir, ".intentci", "changes"), []byte("not-a-dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if code := RunMain([]string{"validate"}, &out, &errb); code != 20 {
		t.Fatalf("readdir fail code=%d err=%s", code, errb.String())
	}
}

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

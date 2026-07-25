package cli

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestValidate_GetwdError(t *testing.T) {
	old := getwd
	defer func() { getwd = old }()
	getwd = func() (string, error) { return "", errors.New("getwd") }
	var out, errb bytes.Buffer
	if code := RunMain([]string{"validate"}, &out, &errb); code != 1 {
		t.Fatalf("code=%d", code)
	}
}

func TestVerify_FindRootError(t *testing.T) {
	tmp := t.TempDir()
	old := getwd
	defer func() { getwd = old }()
	getwd = func() (string, error) { return tmp, nil }
	var out, errb bytes.Buffer
	if code := RunMain([]string{"verify"}, &out, &errb); code != 21 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
}

func TestVerify_ExitCodeOnly(t *testing.T) {
	gitDir := gitRepo(t)
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(gitDir)
	oldGetwd := getwd
	getwd = func() (string, error) { return gitDir, nil }
	defer func() { getwd = oldGetwd }()
	os.MkdirAll(gitDir+"/.intentci", 0o755)
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
    command: "false"
    profiles: [full]
    inputs: ["**"]
    timeout: 1m
`
	os.WriteFile(gitDir+"/.intentci/contract.yaml", []byte(body), 0o644)
	var out, errb bytes.Buffer
	code := RunMain([]string{"verify", "--trust", "--base", "HEAD"}, &out, &errb)
	if code != 10 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
}

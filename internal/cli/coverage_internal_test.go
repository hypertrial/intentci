package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestValidate_LoadChangeError(t *testing.T) {
	gitDir := gitRepo(t)
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(gitDir)
	oldGetwd := getwd
	getwd = func() (string, error) { return gitDir, nil }
	defer func() { getwd = oldGetwd }()

	var out, errb bytes.Buffer
	RunMain([]string{"init"}, &out, &errb)
	body := `version: 1
product: {name: x, purpose: y}
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    verification: {checks: [go-test]}
checks:
  - id: go-test
    command: "true"
    timeout: 1m
`
	os.WriteFile(filepath.Join(gitDir, ".intentci", "contract.yaml"), []byte(body), 0o644)
	os.MkdirAll(filepath.Join(gitDir, ".intentci", "changes"), 0o755)
	os.WriteFile(filepath.Join(gitDir, ".intentci", "changes", "BAD.yaml"), []byte(":\n"), 0o644)
	out.Reset()
	errb.Reset()
	if code := RunMain([]string{"validate"}, &out, &errb); code != 20 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
}

func TestVerify_ErrorWithOutcome(t *testing.T) {
	gitDir := gitRepo(t)
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(gitDir)
	oldGetwd := getwd
	getwd = func() (string, error) { return gitDir, nil }
	defer func() { getwd = oldGetwd }()
	os.MkdirAll(filepath.Join(gitDir, ".intentci"), 0o755)
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
	os.WriteFile(filepath.Join(gitDir, ".intentci", "contract.yaml"), []byte(body), 0o644)
	var out, errb bytes.Buffer
	code := RunMain([]string{"verify", "--trust", "--base", "HEAD", "--format", "json"}, &out, &errb)
	if code != 10 {
		t.Fatalf("code=%d", code)
	}
}

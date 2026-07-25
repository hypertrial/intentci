package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestValidate_SemanticAndChangeSpecError(t *testing.T) {
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
  - id: BUILD-001
    type: reliability
    title: t2
    statement: s2
    status: approved
    severity: blocking
    verification: {checks: [go-test]}
checks:
  - id: go-test
    command: "true"
    timeout: 1m
`
	os.WriteFile(filepath.Join(gitDir, ".intentci", "contract.yaml"), []byte(body), 0o644)
	out.Reset()
	if code := RunMain([]string{"validate"}, &out, &errb); code != 20 {
		t.Fatalf("semantic code=%d", code)
	}
	body = `version: 1
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
	change := `version: 1
id: BAD-1
status: approved
type: feature
summary: s
goals: [g]
acceptance:
  - id: AC-001
    statement: s
    severity: blocking
    verification: {checks: [missing]}
`
	os.WriteFile(filepath.Join(gitDir, ".intentci", "changes", "BAD-1.yaml"), []byte(change), 0o644)
	out.Reset()
	if code := RunMain([]string{"validate"}, &out, &errb); code != 20 {
		t.Fatalf("change spec code=%d err=%s", code, errb.String())
	}
}

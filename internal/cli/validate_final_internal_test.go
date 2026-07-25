package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestValidate_NoChangesDir(t *testing.T) {
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
	os.RemoveAll(filepath.Join(gitDir, ".intentci", "changes"))
	out.Reset()
	errb.Reset()
	if code := RunMain([]string{"validate"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
}

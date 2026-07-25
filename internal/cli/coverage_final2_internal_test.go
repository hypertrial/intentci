package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestValidate_ContractLoadError(t *testing.T) {
	gitDir := gitRepo(t)
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(gitDir)
	oldGetwd := getwd
	getwd = func() (string, error) { return gitDir, nil }
	defer func() { getwd = oldGetwd }()
	os.MkdirAll(filepath.Join(gitDir, ".intentci"), 0o755)
	os.WriteFile(filepath.Join(gitDir, ".intentci", "contract.yaml"), []byte(":\n"), 0o644)
	var out, errb bytes.Buffer
	if code := RunMain([]string{"validate"}, &out, &errb); code != 20 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
}

func TestVerify_ContractErrorUses20(t *testing.T) {
	gitDir := gitRepo(t)
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(gitDir)
	oldGetwd := getwd
	getwd = func() (string, error) { return gitDir, nil }
	defer func() { getwd = oldGetwd }()
	os.MkdirAll(filepath.Join(gitDir, ".intentci"), 0o755)
	os.WriteFile(filepath.Join(gitDir, ".intentci", "contract.yaml"), []byte(":\n"), 0o644)
	var out, errb bytes.Buffer
	if code := RunMain([]string{"verify", "--trust"}, &out, &errb); code != 20 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
}

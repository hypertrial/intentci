package changespec

import (
	"os/exec"
	"testing"
)

func TestRunGit_StderrPath(t *testing.T) {
	if _, err := runGit(t.TempDir(), "rev-parse", "NOTAREF"); err == nil {
		t.Fatal("expected error")
	}
	// trigger empty stderr fallback via invalid git dir
	if _, err := runGit("/nonexistent/path/intentci-git", "status"); err == nil {
		t.Fatal("expected error")
	}
	_ = exec.ErrNotFound
}

func TestCompileSchema_AddResource(t *testing.T) {
	// valid compile path with real schema
	if _, err := compileSchema(schemaJSON(), "https://intentci.dev/schemas/changespec-v1.json"); err != nil {
		t.Fatal(err)
	}
}

package initcmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_AbsError(t *testing.T) {
	old := absPath
	defer func() { absPath = old }()
	absPath = func(string) (string, error) { return "", errors.New("abs") }
	if _, err := Run("."); err == nil {
		t.Fatal("abs")
	}
}

func TestRun_MultipleDraftDetectors(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}\n"), 0o644)
	res, err := Run(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(res.ContractPath)
	if !strings.Contains(string(data), "npm-test") {
		t.Fatalf("%s", data)
	}
}

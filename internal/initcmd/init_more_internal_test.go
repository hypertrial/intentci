package initcmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_GitignoreWriteError(t *testing.T) {
	dir := t.TempDir()
	oldW := writeFile
	defer func() { writeFile = oldW }()
	calls := 0
	writeFile = func(path string, data []byte, perm os.FileMode) error {
		if filepath.Base(path) == ".gitignore" {
			return errors.New("write gitignore")
		}
		calls++
		return os.WriteFile(path, data, perm)
	}
	if _, err := Run(dir); err == nil {
		t.Fatal("gitignore write")
	}
}

func TestDetectDraftChecks_AllFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"pytest.ini", "setup.py"} {
		os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644)
	}
	out := detectDraftChecks(dir)
	if len(out) == 0 {
		t.Fatal(out)
	}
}

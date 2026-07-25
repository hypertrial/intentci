package initcmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_ContractWriteError(t *testing.T) {
	dir := t.TempDir()
	oldW := writeFile
	defer func() { writeFile = oldW }()
	writeFile = func(path string, data []byte, perm os.FileMode) error {
		if filepath.Base(path) == "contract.yaml" {
			return errors.New("write contract")
		}
		return os.WriteFile(path, data, perm)
	}
	if _, err := Run(dir); err == nil {
		t.Fatal("contract write")
	}
}

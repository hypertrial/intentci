package initcmd

import (
	"errors"
	"os"
	"testing"
)

func TestRun_ContractStatOtherError(t *testing.T) {
	dir := t.TempDir()
	oldS := fileStat
	defer func() { fileStat = oldS }()
	fileStat = func(path string) (os.FileInfo, error) {
		if filepathBase(path) == "contract.yaml" {
			return nil, errors.New("stat")
		}
		st, err := os.Stat(path)
		return st, err
	}
	if _, err := Run(dir); err == nil {
		t.Fatal("stat contract")
	}
}

func filepathBase(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

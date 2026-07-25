package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestFindRoot_AbsError(t *testing.T) {
	old := absPath
	defer func() { absPath = old }()
	absPath = func(string) (string, error) { return "", errors.New("abs") }
	if _, err := FindRoot("."); err == nil {
		t.Fatal("expected abs error")
	}
}

func TestFindRoot_WalkUp(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, contract.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := FindRoot(nested)
	if err != nil || root != dir {
		t.Fatalf("got %q %v", root, err)
	}
}

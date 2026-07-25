package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/contract"
)

func TestFindRoot_IntentCI(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, contract.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := config.FindRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Fatalf("got %s want %s", root, dir)
	}
	if config.IntentCIDir(root) != filepath.Join(root, contract.DirName) {
		t.Fatal("IntentCIDir mismatch")
	}
}

func TestFindRoot_Git(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := config.FindRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Fatalf("got %s", root)
	}
}

func TestFindRoot_Missing(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.FindRoot(dir); err == nil {
		t.Fatal("expected error")
	}
}

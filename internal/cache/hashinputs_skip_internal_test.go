package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashInputs_SkipsGitDir(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".git", "objects"), 0o755)
	os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644)
	sum, err := hashInputs(root, []string{"**"})
	if err != nil {
		t.Fatal(err)
	}
	if sum == "" {
		t.Fatal("empty")
	}
}

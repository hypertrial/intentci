package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashInputs_UnreadableFile(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "secret.go")
	os.WriteFile(f, []byte("x"), 0o644)
	os.Chmod(f, 0o000)
	defer os.Chmod(f, 0o644)
	if _, err := hashInputs(root, []string{"**"}); err == nil {
		t.Fatal("expected file sha error")
	}
}

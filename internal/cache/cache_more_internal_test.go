package cache

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestHashInputs_WalkAndFileSHAErrors(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.MkdirAll(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(blocked, 0o755)
	if _, err := hashInputs(root, []string{"**"}); err == nil {
		t.Fatal("walk error")
	}

	if _, err := fileSHA(root); err == nil {
		t.Fatal("directory sha")
	}
}

func TestFileSHA_CopyError(t *testing.T) {
	old := copyFile
	defer func() { copyFile = old }()
	copyFile = func(io.Writer, io.Reader) (int64, error) { return 0, errors.New("copy") }
	root := t.TempDir()
	f := filepath.Join(root, "f")
	os.WriteFile(f, []byte("x"), 0o644)
	if _, err := fileSHA(f); err == nil {
		t.Fatal("copy error")
	}
}

func TestKey_NoErrorOnMarshal(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.go"), []byte("x"), 0o644)
	key, ok, err := Key(KeyInput{
		Check: contract.Check{ID: "c", Command: "true", Inputs: []string{"**"}, Cache: "success", Timeout: "1m"},
		RepoRoot: root,
	})
	if err != nil || !ok || key == "" {
		t.Fatalf("%q %v %v", key, ok, err)
	}
}

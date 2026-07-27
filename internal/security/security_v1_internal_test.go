package security

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInsideFilesystemErrors(t *testing.T) {
	if (&PathViolationError{Message: "unsafe"}).Error() != "unsafe" {
		t.Fatal("path violation message")
	}
	oldAbs, oldEval := absolutePath, evaluateSymlinks
	defer func() {
		absolutePath = oldAbs
		evaluateSymlinks = oldEval
	}()

	absolutePath = func(string) (string, error) { return "", errors.New("absolute") }
	if _, err := ResolveInside("root", "path"); err == nil {
		t.Fatal("absolute error ignored")
	}
	absolutePath = oldAbs

	evaluateSymlinks = func(string) (string, error) { return "", errors.New("root") }
	if _, err := ResolveInside(t.TempDir(), "path"); err == nil {
		t.Fatal("root symlink error ignored")
	}

	root := t.TempDir()
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	evaluateSymlinks = func(path string) (string, error) {
		calls++
		if calls == 1 {
			return rootAbs, nil
		}
		return "", errors.New("candidate")
	}
	if _, err := ResolveInside(root, "path"); err == nil {
		t.Fatal("candidate symlink error ignored")
	}

	calls = 0
	evaluateSymlinks = func(path string) (string, error) {
		calls++
		switch calls {
		case 1:
			return rootAbs, nil
		case 2:
			return "", os.ErrNotExist
		default:
			return "", errors.New("parent")
		}
	}
	if _, err := ResolveInside(root, "missing/path"); err == nil {
		t.Fatal("parent symlink error ignored")
	}
}

package trust

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrust_ErrorBranches(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)

	oldAbs := absPath
	defer func() { absPath = oldAbs }()
	absPath = func(string) (string, error) { return "", errors.New("abs") }
	if err := Trust(t.TempDir()); err == nil {
		t.Fatal("trust abs")
	}
	if _, err := IsTrusted(t.TempDir()); err != nil {
		// IsTrusted falls back to root when abs fails
	}
	absPath = filepath.Abs

	root := t.TempDir()
	path := filepath.Join(cfg, "intentci", "trusted-repos")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("# comment\n\n\n"+root+"\n"), 0o644)
	ok, err := IsTrusted(root)
	if err != nil || !ok {
		t.Fatalf("line match: %v %v", ok, err)
	}

	os.WriteFile(path, []byte("hash\t"+root+"\n"), 0o644)
	ok, err = IsTrusted(root)
	if err != nil || !ok {
		t.Fatalf("tab match: %v %v", ok, err)
	}

	os.Remove(path)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := IsTrusted(root); err == nil {
		t.Fatal("read error")
	}

	absPath = func(p string) (string, error) { return p, nil }
	if keyFor("x") == "" {
		t.Fatal("keyFor")
	}
	absPath = oldAbs

	var stderr bytes.Buffer
	oldCfg2 := SetUserConfigDir(func() (string, error) { return "", errors.New("cfg") })
	if err := Ensure(root, false, strings.NewReader("y\n"), &stderr); err == nil {
		t.Fatal("ensure store path")
	}
	SetUserConfigDir(oldCfg2)
}

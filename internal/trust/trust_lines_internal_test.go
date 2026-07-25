package trust

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsTrusted_EmptyFieldLine(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	path := filepath.Join(cfg, "intentci", "trusted-repos")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("   \n"), 0o644)
	ok, err := IsTrusted(t.TempDir())
	if err != nil || ok {
		t.Fatalf("%v %v", ok, err)
	}
}

func TestEnsure_DenyPromptExplicit(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	var stderr bytes.Buffer
	if err := Ensure(t.TempDir(), false, strings.NewReader("no\n"), &stderr); err == nil {
		t.Fatal("deny")
	}
}

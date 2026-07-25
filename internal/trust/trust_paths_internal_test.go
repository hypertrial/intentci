package trust

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsure_NilStdin(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	var stderr bytes.Buffer
	if err := Ensure(t.TempDir(), true, nil, &stderr); err != nil {
		t.Fatal(err)
	}
	_ = os.Stdin
}

func TestIsTrusted_SecondFieldMatch(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	root := t.TempDir()
	path := filepath.Join(cfg, "intentci", "trusted-repos")
	os.MkdirAll(filepath.Join(cfg, "intentci"), 0o755)
	os.WriteFile(path, []byte("ignored\t"+root+"\n"), 0o644)
	ok, err := IsTrusted(root)
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
}

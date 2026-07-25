package trust

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestIsTrusted_AbsFallbackKey(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	root := t.TempDir()
	oldAbs := absPath
	absPath = func(p string) (string, error) {
		if p == root {
			return "", errors.New("abs")
		}
		return p, nil
	}
	defer func() { absPath = oldAbs }()
	key := keyFor(root)
	path := filepath.Join(cfg, "intentci", "trusted-repos")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(key+"\n"), 0o644)
	ok, err := IsTrusted(root)
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
}

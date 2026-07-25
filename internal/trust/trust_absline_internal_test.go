package trust

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTrusted_LineEqualsAbs(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	root := t.TempDir()
	path := filepath.Join(cfg, "intentci", "trusted-repos")
	os.MkdirAll(filepath.Dir(path), 0o755)
	abs, _ := filepath.Abs(root)
	os.WriteFile(path, []byte(abs+"\n"), 0o644)
	ok, err := IsTrusted(root)
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
}

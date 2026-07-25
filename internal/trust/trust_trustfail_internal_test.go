package trust

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrust_IsTrustedFailsAtStart(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	path := filepath.Join(cfg, "intentci", "trusted-repos")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.Mkdir(path, 0o755)
	if err := Trust(t.TempDir()); err == nil {
		t.Fatal("expected isTrusted error")
	}
}

package trust

import (
	"bytes"
	"testing"
)

func TestEnsure_AlreadyTrusted(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	root := t.TempDir()
	if err := Trust(root); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(root, false, nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

package trust

import (
	"bytes"
	"strings"
	"testing"
)

func TestEnsure_PromptNo(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	var stderr bytes.Buffer
	if err := Ensure(t.TempDir(), false, strings.NewReader("no\n"), &stderr); err == nil {
		t.Fatal("deny")
	}
}

func TestTrust_AlreadyTrusted(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	root := t.TempDir()
	if err := Trust(root); err != nil {
		t.Fatal(err)
	}
	if err := Trust(root); err != nil {
		t.Fatal("already trusted should noop")
	}
}

func TestIsTrusted_KeyMatch(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	root := t.TempDir()
	if err := Trust(root); err != nil {
		t.Fatal(err)
	}
	ok, err := IsTrusted(root)
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
}

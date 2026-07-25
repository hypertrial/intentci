package trust

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsTrusted_KeyOnlyEntry(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	root := t.TempDir()
	key := keyFor(root)
	path := filepath.Join(cfg, "intentci", "trusted-repos")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(key+"\n"), 0o644)
	ok, err := IsTrusted(root)
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
}

func TestEnsure_AutoTrustStderr(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	var stderr bytes.Buffer
	if err := Ensure(t.TempDir(), true, strings.NewReader(""), &stderr); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected stderr message")
	}
}

func TestEnsure_StdinNilUsesOS(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() {
		w.Write([]byte("yes\n"))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()
	if err := Ensure(t.TempDir(), false, nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

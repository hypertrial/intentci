package trust

import (
	"errors"
	"os"
	"testing"
)

func TestTrust_MkdirAndOpenErrors(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)

	oldMk := mkdirAllTrust
	oldOpen := openFileTrust
	defer func() {
		mkdirAllTrust = oldMk
		openFileTrust = oldOpen
	}()

	mkdirAllTrust = func(string, os.FileMode) error { return errors.New("mkdir") }
	if err := Trust(t.TempDir()); err == nil {
		t.Fatal("mkdir")
	}
	mkdirAllTrust = os.MkdirAll

	called := false
	openFileTrust = func(string, int, os.FileMode) (*os.File, error) {
		called = true
		return nil, errors.New("open")
	}
	if err := Trust(t.TempDir()); err == nil || !called {
		t.Fatalf("open called=%v", called)
	}
	openFileTrust = os.OpenFile
}

func TestKeyFor_AbsFallback(t *testing.T) {
	old := absPath
	defer func() { absPath = old }()
	absPath = func(string) (string, error) { return "", errors.New("abs") }
	if keyFor("relative") == "" {
		t.Fatal("keyFor fallback")
	}
}

func TestIsTrusted_EmptyFields(t *testing.T) {
	cfg := t.TempDir()
	oldCfg := SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer SetUserConfigDir(oldCfg)
	path := cfg + "/intentci/trusted-repos"
	os.MkdirAll(cfg+"/intentci", 0o755)
	os.WriteFile(path, []byte("   \n#\n"), 0o644)
	ok, err := IsTrusted(t.TempDir())
	if err != nil || ok {
		t.Fatalf("%v %v", ok, err)
	}
}

func TestEnsure_IsTrustedError(t *testing.T) {
	oldCfg := SetUserConfigDir(func() (string, error) { return "", errors.New("cfg") })
	defer SetUserConfigDir(oldCfg)
	if err := Ensure(t.TempDir(), true, nil, os.Stderr); err == nil {
		t.Fatal("ensure isTrusted error")
	}
}

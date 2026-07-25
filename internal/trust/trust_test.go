package trust_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/trust"
)

func TestTrustFlow(t *testing.T) {
	cfg := t.TempDir()
	old := trust.SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer trust.SetUserConfigDir(old)

	root := t.TempDir()
	ok, err := trust.IsTrusted(root)
	if err != nil || ok {
		t.Fatalf("expected untrusted: %v %v", ok, err)
	}
	var stderr bytes.Buffer
	if err := trust.Ensure(root, true, strings.NewReader(""), &stderr); err != nil {
		t.Fatal(err)
	}
	ok, err = trust.IsTrusted(root)
	if err != nil || !ok {
		t.Fatalf("expected trusted: %v %v", ok, err)
	}
	if err := trust.Trust(root); err != nil {
		t.Fatal(err)
	}
	// prompt yes
	root2 := t.TempDir()
	if err := trust.Ensure(root2, false, strings.NewReader("yes\n"), &stderr); err != nil {
		t.Fatal(err)
	}
	root3 := t.TempDir()
	if err := trust.Ensure(root3, false, strings.NewReader("n\n"), &stderr); err == nil {
		t.Fatal("expected deny")
	}
	path, err := trust.StorePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# c\n\n"+root+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err = trust.IsTrusted(root)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
}

func TestStorePathError(t *testing.T) {
	old := trust.SetUserConfigDir(func() (string, error) { return "", errors.New("nope") })
	defer trust.SetUserConfigDir(old)
	if _, err := trust.StorePath(); err == nil {
		t.Fatal("expected error")
	}
	if _, err := trust.IsTrusted(t.TempDir()); err == nil {
		t.Fatal("expected error")
	}
	if err := trust.Trust(t.TempDir()); err == nil {
		t.Fatal("expected error")
	}
}

func TestIsTrustedReadError(t *testing.T) {
	cfg := t.TempDir()
	old := trust.SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer trust.SetUserConfigDir(old)
	path := filepath.Join(cfg, "intentci")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	// make trusted-repos a directory to force read error
	if err := os.MkdirAll(filepath.Join(path, "trusted-repos"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := trust.IsTrusted(t.TempDir()); err == nil {
		t.Fatal("expected read error")
	}
}

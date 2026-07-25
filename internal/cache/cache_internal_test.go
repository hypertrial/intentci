package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestCacheInternalBranches(t *testing.T) {
	old := SetUserCacheDir(func() (string, error) { return "", errors.New("no") })
	if _, err := DefaultRoot(); err == nil {
		t.Fatal("expected error")
	}
	if _, err := Open(""); err == nil {
		t.Fatal("open error")
	}
	SetUserCacheDir(old)

	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.ObjectPath("abc/../x")
	_ = s.ObjectPath("ok")
	zero := 0
	_ = s.Put("k", runner.Result{Status: protocol.CheckFail, ExitCode: &zero})
	// non-pass stored incorrectly then Get rejects
	path := s.ObjectPath("badstatus")
	_ = os.WriteFile(path, []byte(`{"status":"fail"}`), 0o644)
	if _, ok := s.Get("badstatus"); ok {
		t.Fatal("fail status")
	}
	// env include
	os.Setenv("INTENTCI_TEST_ENV", "1")
	defer os.Unsetenv("INTENTCI_TEST_ENV")
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "f.go"), []byte("package f\n"), 0o644)
	_, ok, err := Key(KeyInput{
		Check: contract.Check{ID: "c", Command: "true", Inputs: []string{"**/*.go"}, Cache: "success"},
		RepoRoot: root, EnvInclude: []string{"INTENTCI_TEST_ENV"},
	})
	if err != nil || !ok {
		t.Fatal(err, ok)
	}
	// invalid pattern
	_, _, err = Key(KeyInput{
		Check: contract.Check{ID: "c", Command: "true", Inputs: []string{"["}, Cache: "success"},
		RepoRoot: root,
	})
	if err == nil {
		t.Fatal("bad pattern")
	}
	// fileSHA missing
	if _, err := fileSHA(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing file")
	}
	// Get missing
	if _, ok := s.Get("nope"); ok {
		t.Fatal()
	}
	mkdirAll = func(string, os.FileMode) error { return errors.New("mkdir") }
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("mkdir")
	}
	mkdirAll = os.MkdirAll
	s2, _ := Open(t.TempDir())
	writeFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	_ = s2.Put("z", runner.Result{Status: protocol.CheckPass, ExitCode: &zero})
	writeFile = os.WriteFile
	rename = func(string, string) error { return errors.New("rename") }
	_ = s2.Put("z", runner.Result{Status: protocol.CheckPass, ExitCode: &zero})
	rename = os.Rename
	_ = json.Marshal
}

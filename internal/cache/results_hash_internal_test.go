package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestKey_ResultsHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.xml")
	if err := os.WriteFile(path, []byte(`<testsuite/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	ch := contract.Check{
		ID: "j", Command: "true", Inputs: []string{"**/*.go"},
		Results: &contract.Results{Format: "junit", Path: "out.xml"},
	}
	// need a matching input file
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644); err != nil {
		t.Fatal(err)
	}
	k1, ok, err := Key(KeyInput{Check: ch, RepoRoot: dir, ContractHash: "c"})
	if err != nil || !ok || k1 == "" {
		t.Fatalf("%v %v %s", err, ok, k1)
	}
	ch.Results.Path = "missing.xml"
	k2, ok, err := Key(KeyInput{Check: ch, RepoRoot: dir, ContractHash: "c"})
	if err != nil || !ok {
		t.Fatal(err)
	}
	if k1 == k2 {
		t.Fatal("results hash should change key")
	}
}

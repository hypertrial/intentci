package cache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/cache"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestCache_HitMissCorruptNoInputs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := cache.SetUserCacheDir(func() (string, error) { return t.TempDir(), nil })
	defer cache.SetUserCacheDir(old)

	store, err := cache.Open("")
	if err != nil {
		t.Fatal(err)
	}
	ch := contract.Check{
		ID:      "unit",
		Command: "true",
		Inputs:  []string{"**/*.go"},
		Cache:   "success",
		Timeout: "1m",
	}
	key, ok, err := cache.Key(cache.KeyInput{
		Check:        ch,
		ContractHash: "c1",
		ChangeHash:   "",
		RepoRoot:     root,
	})
	if err != nil || !ok {
		t.Fatalf("key: %v %v", ok, err)
	}
	zero := 0
	if err := store.Put(key, runner.Result{Status: protocol.CheckPass, ExitCode: &zero}); err != nil {
		t.Fatal(err)
	}
	got, hit := store.Get(key)
	if !hit || got.Status != protocol.CheckPass {
		t.Fatalf("hit=%v %+v", hit, got)
	}
	// corrupt
	if err := os.WriteFile(store.ObjectPath(key), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, hit := store.Get(key); hit {
		t.Fatal("corrupt should miss")
	}
	// no inputs
	_, ok, err = cache.Key(cache.KeyInput{Check: contract.Check{ID: "x", Command: "true"}, RepoRoot: root})
	if err != nil || ok {
		t.Fatalf("no inputs should not cache: %v %v", ok, err)
	}
	_, ok, err = cache.Key(cache.KeyInput{Check: contract.Check{ID: "x", Command: "true", Cache: "off", Inputs: []string{"**"}}, RepoRoot: root})
	if err != nil || ok {
		t.Fatalf("cache off: %v %v", ok, err)
	}
	// fail not stored meaningfully
	one := 1
	_ = store.Put(key, runner.Result{Status: protocol.CheckFail, ExitCode: &one})
	// input change
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n//x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	key2, ok, err := cache.Key(cache.KeyInput{Check: ch, ContractHash: "c1", RepoRoot: root})
	if err != nil || !ok {
		t.Fatal(err)
	}
	if key2 == key {
		t.Fatal("expected key change")
	}
}

func TestOpenExplicit(t *testing.T) {
	dir := t.TempDir()
	s, err := cache.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Root != dir {
		t.Fatal(s.Root)
	}
}

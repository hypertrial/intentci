package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/hypertrial/intentci/internal/cache"
	"github.com/hypertrial/intentci/internal/contract"
)

func TestRun_CacheKeyErrorAndNilWriter(t *testing.T) {
	dir := t.TempDir()
	store, _ := cache.Open(t.TempDir())
	checks := map[string]contract.Check{
		"a": {ID: "a", Command: "true", Timeout: "5s", Inputs: []string{"["}, Cache: "success"},
		"b": {ID: "b", Command: "true", Timeout: "5s", DependsOn: []string{"a"}},
	}
	res := Run(context.Background(), checks, []string{"a", "b"}, schedulerOpts(dir, store, false))
	if res["a"].Status == "" {
		t.Fatal(res)
	}

	res = Run(context.Background(), checks, []string{"a"}, schedulerOpts(dir, store, true))
	if res["a"].FromCache {
		t.Fatal("no cache")
	}

	checks2 := map[string]contract.Check{
		"x": {ID: "x", Command: "true", Timeout: "5s"},
		"y": {ID: "y", Command: "true", Timeout: "5s", DependsOn: []string{"z"}},
		"z": {ID: "z", Command: "false", Timeout: "5s"},
	}
	res = Run(context.Background(), checks2, []string{"x", "y", "z"}, schedulerOpts(dir, nil, true))
	if res["y"].Status == "" {
		t.Fatal(res)
	}
}

func schedulerOpts(dir string, store *cache.Store, noCache bool) Options {
	return Options{Dir: dir, MaxParallel: 1, Cache: store, NoCache: noCache, ContractHash: "c"}
}

func TestLockedWriter_Nil(t *testing.T) {
	lw := lockedWriter{w: nil, mu: nil}
	if lw.writer() != nil {
		t.Fatal("nil writer")
	}
}

func TestPrefixWriter_WriteError(t *testing.T) {
	pw := prefixWriter(&errW{}, "id")
	if _, err := pw.Write([]byte("line\n")); err == nil {
		t.Fatal("write error")
	}
}

type errW struct{}

func (errW) Write([]byte) (int, error) { return 0, errors.New("write") }

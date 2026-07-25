package scheduler_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/hypertrial/intentci/internal/cache"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/scheduler"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestRun_SkipsDependentOnFailure(t *testing.T) {
	checks := map[string]contract.Check{
		"unit":        {ID: "unit", Command: "false", Timeout: "5s"},
		"integration": {ID: "integration", Command: "true", Timeout: "5s", DependsOn: []string{"unit"}},
	}
	res := scheduler.Run(context.Background(), checks, []string{"unit", "integration"}, scheduler.Options{Dir: t.TempDir(), MaxParallel: 2})
	if res["unit"].Status != protocol.CheckFail || res["integration"].Status != protocol.CheckSkipped {
		t.Fatalf("%+v", res)
	}
}

func TestRun_TimeoutUnknown(t *testing.T) {
	checks := map[string]contract.Check{"slow": {ID: "slow", Command: "sleep 2", Timeout: "50ms"}}
	res := scheduler.Run(context.Background(), checks, []string{"slow"}, scheduler.Options{Dir: t.TempDir()})
	if res["slow"].Status != protocol.CheckUnknown {
		t.Fatalf("%+v", res)
	}
}

func TestRun_ExclusiveStreamCache(t *testing.T) {
	dir := t.TempDir()
	store, err := cache.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]contract.Check{
		"ex": {ID: "ex", Command: "printf 'hi'", Timeout: "5s", Exclusive: true, Inputs: []string{"**"}, Cache: "success"},
		"a":  {ID: "a", Command: "true", Timeout: "5s", Inputs: []string{"**"}, Cache: "success"},
		"b":  {ID: "b", Command: "true", Timeout: "5s", Inputs: []string{"**"}, Cache: "success"},
	}
	var out, errb bytes.Buffer
	res := scheduler.Run(context.Background(), checks, []string{"ex", "a", "b"}, scheduler.Options{
		Dir: dir, MaxParallel: 2, Stdout: &out, Stderr: &errb,
		Cache: store, ContractHash: "c",
	})
	if res["ex"].Status != protocol.CheckPass {
		t.Fatalf("%+v", res)
	}
	res2 := scheduler.Run(context.Background(), checks, []string{"a"}, scheduler.Options{
		Dir: dir, Cache: store, ContractHash: "c",
	})
	if !res2["a"].FromCache {
		t.Fatalf("expected cache hit %+v", res2["a"])
	}
	_ = scheduler.Run(context.Background(), checks, []string{"a"}, scheduler.Options{
		Dir: dir, Cache: store, NoCache: true, ContractHash: "c", Stdout: &out,
	})
}

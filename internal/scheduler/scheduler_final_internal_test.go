package scheduler

import (
	"context"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestRun_MaxParallelTrim(t *testing.T) {
	checks := map[string]contract.Check{
		"a": {ID: "a", Command: "true", Timeout: "5s"},
		"b": {ID: "b", Command: "true", Timeout: "5s"},
		"c": {ID: "c", Command: "true", Timeout: "5s"},
	}
	res := Run(context.Background(), checks, []string{"a", "b", "c"}, Options{
		Dir: t.TempDir(), MaxParallel: 1,
	})
	if len(res) != 3 {
		t.Fatalf("%d", len(res))
	}
}

func TestRun_DepNotReady(t *testing.T) {
	checks := map[string]contract.Check{
		"a": {ID: "a", Command: "true", Timeout: "5s", DependsOn: []string{"b"}},
		"b": {ID: "b", Command: "true", Timeout: "5s", DependsOn: []string{"a"}},
	}
	res := Run(context.Background(), checks, []string{"a", "b"}, Options{Dir: t.TempDir()})
	if len(res) != 2 {
		t.Fatalf("%#v", res)
	}
}

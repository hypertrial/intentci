package scheduler

import (
	"context"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestRun_NumCPUFallback(t *testing.T) {
	old := numCPU
	defer func() { numCPU = old }()
	numCPU = func() int { return 0 }
	checks := map[string]contract.Check{
		"a": {ID: "a", Command: "true", Timeout: "5s"},
	}
	res := Run(context.Background(), checks, []string{"a"}, Options{Dir: t.TempDir()})
	if len(res) != 1 {
		t.Fatalf("%d", len(res))
	}
}

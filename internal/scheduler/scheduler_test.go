package scheduler_test

import (
	"context"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/scheduler"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestRun_SkipsDependentOnFailure(t *testing.T) {
	checks := map[string]contract.Check{
		"unit": {
			ID:      "unit",
			Command: "false",
			Timeout: "5s",
		},
		"integration": {
			ID:        "integration",
			Command:   "true",
			Timeout:   "5s",
			DependsOn: []string{"unit"},
		},
	}
	res := scheduler.Run(context.Background(), checks, []string{"unit", "integration"}, scheduler.Options{
		Dir:         t.TempDir(),
		MaxParallel: 2,
	})
	if res["unit"].Status != protocol.CheckFail {
		t.Fatalf("unit status=%s", res["unit"].Status)
	}
	if res["integration"].Status != protocol.CheckSkipped {
		t.Fatalf("integration status=%s want skipped", res["integration"].Status)
	}
}

func TestRun_TimeoutUnknown(t *testing.T) {
	checks := map[string]contract.Check{
		"slow": {
			ID:      "slow",
			Command: "sleep 2",
			Timeout: "50ms",
		},
	}
	res := scheduler.Run(context.Background(), checks, []string{"slow"}, scheduler.Options{
		Dir: t.TempDir(),
	})
	if res["slow"].Status != protocol.CheckUnknown {
		t.Fatalf("status=%s want unknown (%s)", res["slow"].Status, res["slow"].Reason)
	}
}

package runner

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestRun_NonExitErrorAndCanceled(t *testing.T) {
	res := Run(context.Background(), contract.Check{ID: "x", Command: "/no/such/command/xyz", Timeout: "2s"}, Options{Dir: t.TempDir()})
	if res.Status != protocol.CheckFail || res.ExitCode == nil || *res.ExitCode == 0 {
		t.Fatalf("non-exit error: %+v", res)
	}
	_ = exec.ErrNotFound

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	res = Run(ctx, contract.Check{ID: "slow", Command: "sleep 5", Timeout: "10s"}, Options{Dir: t.TempDir()})
	if res.Status != protocol.CheckUnknown || res.Reason != "check canceled" {
		t.Fatalf("canceled: %+v", res)
	}
}

func TestExitCode_Default(t *testing.T) {
	if exitCode(exec.ErrNotFound) != 1 {
		t.Fatal(exitCode(exec.ErrNotFound))
	}
}

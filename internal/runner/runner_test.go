package runner_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestRun_PassFailTimeoutCancelTruncate(t *testing.T) {
	dir := t.TempDir()
	res := runner.Run(context.Background(), contract.Check{ID: "ok", Command: "true", Timeout: "2s"}, runner.Options{Dir: dir})
	if res.Status != protocol.CheckPass || res.ExitCode == nil || *res.ExitCode != 0 {
		t.Fatalf("%+v", res)
	}
	res = runner.Run(context.Background(), contract.Check{ID: "bad", Command: "false", Timeout: "2s"}, runner.Options{Dir: dir})
	if res.Status != protocol.CheckFail {
		t.Fatalf("%+v", res)
	}
	res = runner.Run(context.Background(), contract.Check{ID: "slow", Command: "sleep 2", Timeout: "50ms"}, runner.Options{Dir: dir})
	if res.Status != protocol.CheckUnknown || !strings.Contains(res.Reason, "timed out") {
		t.Fatalf("%+v", res)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res = runner.Run(ctx, contract.Check{ID: "c", Command: "sleep 2", Timeout: "5s"}, runner.Options{Dir: dir})
	if res.Status != protocol.CheckUnknown {
		t.Fatalf("cancel: %+v", res)
	}
	res = runner.Run(context.Background(), contract.Check{ID: "badto", Command: "true", Timeout: "notaduration"}, runner.Options{Dir: dir})
	if res.Status != protocol.CheckUnknown {
		t.Fatalf("bad timeout: %+v", res)
	}
	if got := runner.Truncate("abcdef", 3); got != "abc\n...[truncated]" {
		t.Fatalf("truncate=%q", got)
	}
	if got := runner.Truncate("ab", 3); got != "ab" {
		t.Fatalf("truncate short=%q", got)
	}
	if got := runner.Truncate("ab", 0); got != "ab" {
		t.Fatalf("truncate zero=%q", got)
	}
	_ = time.Millisecond
}

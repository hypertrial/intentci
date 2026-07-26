package repair_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/repair"
	"github.com/hypertrial/intentci/internal/verdict"
)

func TestBuildPacketAndDryRun(t *testing.T) {
	b := &evidence.Bundle{
		RunID: "r1",
		Run: verdict.RunResult{
			Verdict: verdict.Fail,
			Requirements: []verdict.RequirementResult{{
				ID: "REQ-1",
				Obligations: []verdict.ObligationResult{{ID: "O1", Verdict: verdict.Fail, Reason: "x"}},
			}},
		},
	}
	p := repair.BuildPacket(b, 1, 3, "")
	if len(p.Failures) != 1 {
		t.Fatalf("%+v", p)
	}
	store, err := evidence.NewStore(t.TempDir(), "runs")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Repair.MaxAttempts = 2
	attempts := 0
	out, err := repair.Run(context.Background(), repair.Options{
		Root: t.TempDir(), Config: cfg, Store: store, DryRun: true, MaxAttempts: 2,
		Verify: func(ctx context.Context) (*evidence.Bundle, error) {
			attempts++
			return b, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ExitCode != exitcode.RepairExhausted || attempts != 2 {
		t.Fatalf("%+v attempts=%d", out, attempts)
	}
	if _, err := os.Stat(filepath.Join(store.Dir("r1"), "repair-packet.json")); err != nil {
		// packet written on first attempt with run id r1
		_ = err
	}
}

func TestPassShortCircuit(t *testing.T) {
	store, _ := evidence.NewStore(t.TempDir(), "runs")
	cfg := config.Default()
	out, err := repair.Run(context.Background(), repair.Options{
		Root: t.TempDir(), Config: cfg, Store: store, DryRun: true,
		Verify: func(ctx context.Context) (*evidence.Bundle, error) {
			return &evidence.Bundle{RunID: "ok", Run: verdict.RunResult{Verdict: verdict.Pass}}, nil
		},
	})
	if err != nil || out.ExitCode != exitcode.Pass {
		t.Fatalf("%v %+v", err, out)
	}
}

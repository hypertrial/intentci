package executor_test

import (
	"context"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/executor"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

func TestRunCommand(t *testing.T) {
	cfg := config.Default()
	cfg.Verification.DefaultTimeout = "5s"
	reqs := []ir.Requirement{{
		ID: "REQ-1", Status: "active", Priority: "required", Title: "t",
		Obligations: []ir.Obligation{{
			ID: "OBL-1", Required: true, Statement: "s",
			Verify: ir.VerifyNode{All: []ir.VerifyNode{{Provider: &ir.ProviderSpec{
				Provider: "command", ID: "c", Run: "true", Result: map[string]any{"equals": float64(0)},
			}}}},
		}},
	}}
	cache := t.TempDir()
	_, results := executor.Run(context.Background(), reqs, executor.Options{
		Root: t.TempDir(), Config: cfg, Registry: provider.DefaultRegistry(),
		RunID: "r1", IRHash: "h", CacheDir: cache,
	})
	if len(results) != 1 || results[0].Verdict != verdict.Pass {
		t.Fatalf("%+v", results)
	}
	// cache hit
	_, results = executor.Run(context.Background(), reqs, executor.Options{
		Root: t.TempDir(), Config: cfg, Registry: provider.DefaultRegistry(),
		RunID: "r2", IRHash: "h", CacheDir: cache,
	})
	if results[0].Verdict != verdict.Pass {
		t.Fatal(results)
	}
}

func TestDuplicateAnonymousCommandLeavesDoNotFalsePass(t *testing.T) {
	cfg := config.Default()
	cfg.Verification.DefaultTimeout = "5s"
	// Two command leaves without ids: one passes, one fails. Must be FAIL every time.
	reqs := []ir.Requirement{{
		ID: "REQ-1", Status: "active", Priority: "required", Title: "t",
		Obligations: []ir.Obligation{{
			ID: "OBL-1", Required: true, Statement: "s",
			Verify: ir.VerifyNode{All: []ir.VerifyNode{
				{Provider: &ir.ProviderSpec{Provider: "command", Run: "true", Result: map[string]any{"equals": float64(0)}}},
				{Provider: &ir.ProviderSpec{Provider: "command", Run: "false", Result: map[string]any{"equals": float64(0)}}},
			}},
		}},
	}}
	for i := 0; i < 20; i++ {
		_, results := executor.Run(context.Background(), reqs, executor.Options{
			Root: t.TempDir(), Config: cfg, Registry: provider.DefaultRegistry(),
			RunID: "r", IRHash: "h", NoCache: true,
		})
		if len(results) != 1 || results[0].Verdict != verdict.Fail {
			t.Fatalf("iter %d: %+v", i, results)
		}
	}
}

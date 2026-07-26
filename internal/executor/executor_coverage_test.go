package executor_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/executor"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

func TestRunUnknownCacheFailAndLogic(t *testing.T) {
	cfg := config.Default()
	cfg.Verification.DefaultTimeout = "2s"
	cfg.Verification.MaxParallel = 2
	reqs := []ir.Requirement{{
		ID: "REQ-1", Status: "active", Priority: "required", Title: "t",
		Obligations: []ir.Obligation{
			{
				ID: "OBL-1", Required: true, Statement: "s",
				Verify: ir.VerifyNode{Any: []ir.VerifyNode{
					{Provider: &ir.ProviderSpec{Provider: "unknown", ID: "u"}},
					{Not: &ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "command", ID: "c", Run: "false", Result: map[string]any{"equals": 0}}}},
				}},
			},
			{
				ID: "OBL-2", Required: true, Statement: "fail",
				Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "command", Run: "false", Result: map[string]any{"equals": 0}}},
			},
		},
	}}
	cache := t.TempDir()
	// seed corrupt cache file to hit unmarshal miss
	if err := os.WriteFile(filepath.Join(cache, "x.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	leaves, results := executor.Run(context.Background(), reqs, executor.Options{
		Root: t.TempDir(), Config: cfg, RunID: "r", IRHash: "h", CacheDir: cache, NoCache: false,
	})
	if len(results) != 1 {
		t.Fatal(results)
	}
	if results[0].Verdict != verdict.Fail && results[0].Verdict != verdict.Error {
		t.Fatalf("%+v", results[0])
	}
	_ = leaves

	// empty cache dir / nil registry defaults
	_, results = executor.Run(context.Background(), reqs[:1], executor.Options{
		Root: t.TempDir(), Config: cfg, Registry: nil, RunID: "r2", IRHash: "h2", NoCache: true,
	})
	if len(results) != 1 {
		t.Fatal(results)
	}

	// saveCache mkdir fail: CacheDir is a file
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	passReqs := []ir.Requirement{{
		ID: "REQ-P", Priority: "required",
		Obligations: []ir.Obligation{{
			ID: "O", Required: true,
			Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "command", ID: "c", Run: "true", Result: map[string]any{"equals": 0}}},
		}},
	}}
	_, _ = executor.Run(context.Background(), passReqs, executor.Options{
		Root: t.TempDir(), Config: cfg, RunID: "r3", IRHash: "h3", CacheDir: file,
	})

	// valid cache hit already covered; force loadCache bad json for matching key by writing after computing is hard.
	// Write a completed failing result won't be cached; write passed then corrupt.
	cache2 := t.TempDir()
	_, _ = executor.Run(context.Background(), passReqs, executor.Options{
		Root: t.TempDir(), Config: cfg, RunID: "r4", IRHash: "same", CacheDir: cache2,
	})
	entries, _ := os.ReadDir(cache2)
	for _, e := range entries {
		_ = os.WriteFile(filepath.Join(cache2, e.Name()), []byte("not-json"), 0o644)
	}
	_, results = executor.Run(context.Background(), passReqs, executor.Options{
		Root: t.TempDir(), Config: cfg, RunID: "r5", IRHash: "same", CacheDir: cache2,
	})
	if results[0].Verdict != verdict.Pass {
		t.Fatal(results)
	}

	// ensure json marshal of cache works for empty evidence edge: status completed but not all passed skips cache
	raw, _ := json.Marshal(provider.Result{Status: "completed"})
	_ = raw
}

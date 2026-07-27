package executor_test

import (
	"context"
	"os"
	"path/filepath"
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

func TestCacheDoesNotReuseSamePathAfterContentChanges(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "check.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	reqs := []ir.Requirement{{
		ID: "REQ-1", Status: "active", Priority: "required",
		Obligations: []ir.Obligation{{
			ID: "OBL-1", Required: true,
			Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "command", ID: "check", Run: "./check.sh"}},
		}},
	}}
	opt := executor.Options{
		Root: root, Config: cfg, Registry: provider.DefaultRegistry(), RunID: "run",
		IRHash: "same", ChangedFiles: []string{"check.sh"}, CacheDir: filepath.Join(root, "cache"),
	}
	_, first := executor.Run(context.Background(), reqs, opt)
	if first[0].Verdict != verdict.Pass {
		t.Fatal(first)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, second := executor.Run(context.Background(), reqs, opt)
	if second[0].Verdict != verdict.Fail {
		t.Fatalf("stale cache produced %s", second[0].Verdict)
	}
}

package executor

import (
	"context"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
)

func TestHelpers(t *testing.T) {
	if split3("a/b") != nil {
		t.Fatal("need 2 slashes")
	}
	if got := split3("a/b/c/d"); len(got) != 3 || got[2] != "c/d" {
		t.Fatalf("%v", got)
	}
	var specs []string
	collectLeaves(ir.VerifyNode{
		All: []ir.VerifyNode{{Provider: &ir.ProviderSpec{ID: "a"}}},
		Any: []ir.VerifyNode{{Provider: &ir.ProviderSpec{ID: "b"}}},
		Not: &ir.VerifyNode{Provider: &ir.ProviderSpec{ID: "c"}},
	}, func(s ir.ProviderSpec) { specs = append(specs, s.ID) })
	if len(specs) != 3 {
		t.Fatal(specs)
	}
	if boolPtr(true) == nil || !*boolPtr(true) {
		t.Fatal("boolPtr")
	}
}

func TestMutateResultsAndSaveCacheMarshal(t *testing.T) {
	cfg := config.Default()
	cfg.Verification.DefaultTimeout = "2s"
	old := mutateResults
	defer func() { mutateResults = old }()
	mutateResults = func(m map[string]provider.Result) {
		m["bad"] = provider.Result{}
	}
	reqs := []ir.Requirement{{
		ID: "REQ-1", Priority: "required",
		Obligations: []ir.Obligation{{
			ID: "O", Required: true,
			Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "command", ID: "c", Run: "true", Result: map[string]any{"equals": 0}}},
		}},
	}}
	_, results := Run(context.Background(), reqs, Options{
		Root: t.TempDir(), Config: cfg, RunID: "r", IRHash: "h", NoCache: true,
	})
	if len(results) != 1 {
		t.Fatal(results)
	}

	// empty leaves path when mutate clears all and no jobs - use req with no obligations
	mutateResults = nil
	_, results = Run(context.Background(), []ir.Requirement{{ID: "R", Priority: "required"}}, Options{
		Root: t.TempDir(), Config: cfg, RunID: "r2", IRHash: "h2", NoCache: true,
	})
	if len(results) != 1 {
		t.Fatal(results)
	}
}

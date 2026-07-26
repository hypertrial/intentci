package verdict_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

func boolPtr(b bool) *bool { return &b }

func TestEvaluateAndAggregate(t *testing.T) {
	leaves := map[string]provider.Result{
		"a": {Status: "completed", Evidence: []provider.Evidence{{Passed: boolPtr(true), Summary: "ok", Class: "deterministic"}}},
		"b": {Status: "completed", Evidence: []provider.Evidence{{Passed: boolPtr(false), Summary: "no", Class: "deterministic"}}},
	}
	node := ir.VerifyNode{All: []ir.VerifyNode{
		{Provider: &ir.ProviderSpec{ID: "a", Provider: "command"}},
		{Provider: &ir.ProviderSpec{ID: "b", Provider: "command"}},
	}}
	v, _, _ := verdict.EvaluateNode(node, leaves)
	if v != verdict.Fail {
		t.Fatalf("got %s", v)
	}
	anyNode := ir.VerifyNode{Any: []ir.VerifyNode{
		{Provider: &ir.ProviderSpec{ID: "a", Provider: "command"}},
		{Provider: &ir.ProviderSpec{ID: "b", Provider: "command"}},
	}}
	v, _, _ = verdict.EvaluateNode(anyNode, leaves)
	if v != verdict.Pass {
		t.Fatalf("got %s", v)
	}
	rr := verdict.AggregateRequirement(ir.Requirement{ID: "R", Title: "t", Priority: "required"}, []verdict.ObligationResult{
		{ID: "o", Required: true, Verdict: verdict.Unproven},
	})
	if rr.Verdict != verdict.Unproven {
		t.Fatal(rr.Verdict)
	}
	run := verdict.AggregateRun([]verdict.RequirementResult{rr})
	if verdict.ExitCode(run.Verdict) != 2 {
		t.Fatalf("exit=%d", verdict.ExitCode(run.Verdict))
	}
}

func TestManualAndMissing(t *testing.T) {
	leaves := map[string]provider.Result{
		"m": {Status: "completed", Evidence: []provider.Evidence{{Class: "manual", Summary: "review", Data: map[string]any{"review_required": true}}}},
	}
	v, _, _ := verdict.EvaluateNode(ir.VerifyNode{Provider: &ir.ProviderSpec{ID: "m", Provider: "manual"}}, leaves)
	if v != verdict.ReviewRequired {
		t.Fatal(v)
	}
	v, _, _ = verdict.EvaluateNode(ir.VerifyNode{Provider: &ir.ProviderSpec{ID: "missing", Provider: "command"}}, nil)
	if v != verdict.Unproven {
		t.Fatal(v)
	}
}

package verdict

import (
	"testing"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
)

func TestMutationSensitiveEvidencePolicyBoundaries(t *testing.T) {
	confidence := 0.5
	passed := true
	node := ir.VerifyNode{Provider: &ir.ProviderSpec{ID: "check"}}
	result := func(evidence []provider.Evidence, policy EvidencePolicy) string {
		value, _, _ := EvaluateNodeWithPolicy(node, map[string]provider.Result{
			"check": {Status: "completed", Evidence: evidence},
		}, policy)
		return value
	}
	if got := result([]provider.Evidence{{
		Class: "probabilistic", Confidence: &confidence, Passed: &passed,
	}}, EvidencePolicy{Class: "probabilistic", ConfidenceThreshold: &confidence}); got != Pass {
		t.Fatalf("confidence exactly at threshold = %s", got)
	}
	if got := result([]provider.Evidence{{
		Class: "probabilistic", Confidence: &confidence, Summary: "missing decision",
	}}, EvidencePolicy{Class: "probabilistic", ConfidenceThreshold: &confidence}); got != Uncertain {
		t.Fatalf("probabilistic evidence without a decision = %s", got)
	}
	if got := result([]provider.Evidence{{
		Class: "other", Passed: &passed,
	}}, EvidencePolicy{Class: "deterministic"}); got != Uncertain {
		t.Fatalf("non-deterministic evidence under deterministic policy = %s", got)
	}
	if got := result([]provider.Evidence{
		{Class: "deterministic", Data: map[string]any{"retry_superseded": true}},
		{Class: "deterministic", Passed: &passed},
	}, EvidencePolicy{Class: "deterministic"}); got != Pass {
		t.Fatalf("superseded retry evidence = %s", got)
	}
}

func TestMutationSensitiveEqualRankKeepsFirstReason(t *testing.T) {
	value, reason := worse(Fail, "first", Fail, "second")
	if value != Fail || reason != "first" {
		t.Fatalf("worse equal rank = %s %q", value, reason)
	}
	result := AggregateRequirement(ir.Requirement{
		ID: "REQ", Priority: "required",
	}, []ObligationResult{
		{ID: "A", Required: true, Verdict: Fail, Reason: "first"},
		{ID: "B", Required: true, Verdict: Fail, Reason: "second"},
	})
	if result.Reason != "first" {
		t.Fatalf("equal-rank aggregation reason = %q", result.Reason)
	}
}

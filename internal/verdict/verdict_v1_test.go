package verdict_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

func TestV1AnyAllFailedAndEvidencePolicies(t *testing.T) {
	node := ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "check"}}
	result := func(evidence ...provider.Evidence) map[string]provider.Result {
		return map[string]provider.Result{"check": {Status: "completed", Evidence: evidence}}
	}
	falseValue, trueValue := false, true
	low, high, threshold := 0.4, 0.9, 0.8

	anyNode := ir.VerifyNode{Any: []ir.VerifyNode{node, node}}
	value, _, _ := verdict.EvaluateNode(anyNode, result(provider.Evidence{
		Class: "deterministic", Passed: &falseValue, Summary: "failed",
	}))
	if value != verdict.Fail {
		t.Fatal(value)
	}

	cases := []struct {
		name     string
		policy   verdict.EvidencePolicy
		evidence []provider.Evidence
		want     string
	}{
		{"human policy", verdict.EvidencePolicy{Class: "human"}, nil, verdict.ReviewRequired},
		{"informational policy", verdict.EvidencePolicy{Class: "informational"}, nil, verdict.Unproven},
		{"informational evidence", verdict.EvidencePolicy{}, []provider.Evidence{{
			Class: "informational", Passed: &trueValue,
		}}, verdict.Unproven},
		{"probability not permitted", verdict.EvidencePolicy{}, []provider.Evidence{{
			Class: "probabilistic", Confidence: &high, Passed: &trueValue,
		}}, verdict.Uncertain},
		{"probability missing confidence", verdict.EvidencePolicy{Class: "probabilistic", ConfidenceThreshold: &threshold}, []provider.Evidence{{
			Class: "probabilistic", Passed: &trueValue,
		}}, verdict.Uncertain},
		{"probability low confidence", verdict.EvidencePolicy{Class: "probabilistic", ConfidenceThreshold: &threshold}, []provider.Evidence{{
			Class: "probabilistic", Confidence: &low, Passed: &trueValue,
		}}, verdict.Uncertain},
		{"probability failed", verdict.EvidencePolicy{Class: "probabilistic", ConfidenceThreshold: &threshold}, []provider.Evidence{{
			Class: "probabilistic", Confidence: &high, Passed: &falseValue, Summary: "model failed",
		}}, verdict.Uncertain},
		{"probability passed", verdict.EvidencePolicy{Class: "probabilistic", ConfidenceThreshold: &threshold}, []provider.Evidence{{
			Class: "probabilistic", Confidence: &high, Passed: &trueValue,
		}}, verdict.Pass},
		{"deterministic mismatch", verdict.EvidencePolicy{Class: "deterministic"}, []provider.Evidence{{
			Class: "probabilistic", Confidence: &high, Passed: &trueValue,
		}}, verdict.Uncertain},
		{"review flag", verdict.EvidencePolicy{}, []provider.Evidence{{
			Class: "deterministic", Data: map[string]any{"review_required": true},
		}}, verdict.ReviewRequired},
		{"superseded then pass", verdict.EvidencePolicy{}, []provider.Evidence{
			{Class: "deterministic", Passed: &falseValue, Data: map[string]any{"retry_superseded": true}},
			{Class: "deterministic", Passed: &trueValue},
		}, verdict.Pass},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, _, _ := verdict.EvaluateNodeWithPolicy(node, result(testCase.evidence...), testCase.policy)
			if got != testCase.want {
				t.Fatalf("got %s want %s", got, testCase.want)
			}
		})
	}
}

func TestV1SkippedAggregationAndConfiguredExit(t *testing.T) {
	requirement := verdict.AggregateRequirement(ir.Requirement{Priority: "required"}, []verdict.ObligationResult{{
		Required: true, Verdict: verdict.Skipped,
	}})
	if requirement.Verdict != verdict.Unproven ||
		requirement.Obligations[0].Reason != "required obligation was not executed" {
		t.Fatalf("%+v", requirement)
	}
	run := verdict.AggregateRun([]verdict.RequirementResult{{
		Priority: "required", Verdict: verdict.Skipped,
	}})
	if run.Verdict != verdict.Skipped {
		t.Fatal(run.Verdict)
	}
	if verdict.ExitCodeConfigured(verdict.Fail, []string{verdict.Error, verdict.Fail}) != 1 ||
		verdict.ExitCodeConfigured(verdict.Fail, []string{verdict.Error}) != 0 {
		t.Fatal("configured exit mapping")
	}
}

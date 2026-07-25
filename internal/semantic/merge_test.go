package semantic_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/semantic"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestApply_MergeMatrix(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{{
			ID:       "R-1",
			Status:   "approved",
			Severity: "blocking",
			Verification: contract.Verification{Semantic: "required", Checks: []string{"c"}},
		}},
	}
	pass := []protocol.RequirementResult{{
		ID: "R-1", Status: protocol.ReqPass, Severity: "blocking",
		Findings: []protocol.Finding{}, Evidence: []protocol.Evidence{},
	}}
	fail := []protocol.RequirementResult{{
		ID: "R-1", Status: protocol.ReqFail, Severity: "blocking",
		Findings: []protocol.Finding{{Type: "deterministic_failure", Summary: "check failed"}},
	}}

	contradiction := semantic.Finding{
		RequirementID: "R-1",
		Assessment:    semantic.AssessmentContradiction,
		Confidence:    0.95,
		Summary:       "bad",
		Evidence:      []semantic.EvidenceCite{{Path: "a.go", LineStart: 1, LineEnd: 2}},
	}

	t.Run("deterministic_fail_wins", func(t *testing.T) {
		out := semantic.Apply(fail, []semantic.Finding{{
			RequirementID: "R-1", Assessment: semantic.AssessmentAligned, Confidence: 1, Summary: "ok",
		}}, semantic.MergeOptions{Policy: contract.SemanticPolicy{Enforcement: "blocking"}, Contract: c})
		if out[0].Status != protocol.ReqFail {
			t.Fatalf("got %s", out[0].Status)
		}
	})

	t.Run("advisory_no_fail", func(t *testing.T) {
		out := semantic.Apply(pass, []semantic.Finding{contradiction}, semantic.MergeOptions{
			Policy: contract.SemanticPolicy{Enforcement: "advisory"}, Contract: c,
		})
		if out[0].Status != protocol.ReqUnverified {
			t.Fatalf("got %s", out[0].Status)
		}
	})

	t.Run("blocking_fail_gates", func(t *testing.T) {
		out := semantic.Apply(pass, []semantic.Finding{contradiction}, semantic.MergeOptions{
			Policy: contract.SemanticPolicy{Enforcement: "blocking"}, Contract: c,
			SemanticModes: map[string]string{"R-1": "required"},
		})
		if out[0].Status != protocol.ReqFail {
			t.Fatalf("got %s", out[0].Status)
		}
	})

	t.Run("blocking_low_confidence_unverified", func(t *testing.T) {
		low := contradiction
		low.Confidence = 0.1
		out := semantic.Apply(pass, []semantic.Finding{low}, semantic.MergeOptions{
			Policy: contract.SemanticPolicy{Enforcement: "blocking"}, Contract: c,
			SemanticModes: map[string]string{"R-1": "required"},
		})
		if out[0].Status != protocol.ReqUnverified {
			t.Fatalf("got %s", out[0].Status)
		}
	})

	t.Run("blocking_optional_no_fail", func(t *testing.T) {
		out := semantic.Apply(pass, []semantic.Finding{contradiction}, semantic.MergeOptions{
			Policy: contract.SemanticPolicy{Enforcement: "blocking"}, Contract: c,
			SemanticModes: map[string]string{"R-1": "optional"},
		})
		if out[0].Status != protocol.ReqUnverified {
			t.Fatalf("got %s", out[0].Status)
		}
	})

	t.Run("aligned_no_change", func(t *testing.T) {
		out := semantic.Apply(pass, []semantic.Finding{{
			RequirementID: "R-1", Assessment: semantic.AssessmentAligned, Confidence: 1, Summary: "ok",
		}}, semantic.MergeOptions{Policy: contract.SemanticPolicy{Enforcement: "advisory"}, Contract: c})
		if out[0].Status != protocol.ReqPass {
			t.Fatalf("got %s", out[0].Status)
		}
	})

	t.Run("insufficient_and_uncertain", func(t *testing.T) {
		for _, a := range []string{semantic.AssessmentInsufficientEvidence, semantic.AssessmentUncertain} {
			out := semantic.Apply(pass, []semantic.Finding{{
				RequirementID: "R-1", Assessment: a, Confidence: 0.9, Summary: a,
				MissingEvidence: []string{"need test"},
			}}, semantic.MergeOptions{Policy: contract.SemanticPolicy{Enforcement: "advisory"}, Contract: c})
			if out[0].Status != protocol.ReqUnverified {
				t.Fatalf("%s -> %s", a, out[0].Status)
			}
		}
	})

	t.Run("off_skips", func(t *testing.T) {
		out := semantic.Apply(pass, []semantic.Finding{contradiction}, semantic.MergeOptions{
			Policy:        contract.SemanticPolicy{Enforcement: "blocking"},
			Contract:      c,
			SemanticModes: map[string]string{"R-1": "off"},
		})
		if out[0].Status != protocol.ReqPass {
			t.Fatalf("got %s", out[0].Status)
		}
	})

	t.Run("waived_skips", func(t *testing.T) {
		w := []protocol.RequirementResult{{ID: "R-1", Status: protocol.ReqWaived, Severity: "blocking"}}
		out := semantic.Apply(w, []semantic.Finding{contradiction}, semantic.MergeOptions{
			Policy: contract.SemanticPolicy{Enforcement: "blocking"}, Contract: c,
			SemanticModes: map[string]string{"R-1": "required"},
		})
		if out[0].Status != protocol.ReqWaived {
			t.Fatalf("got %s", out[0].Status)
		}
	})
}

func TestMarkUnavailable(t *testing.T) {
	reqs := []protocol.RequirementResult{
		{ID: "R-1", Status: protocol.ReqPass, Severity: "blocking"},
		{ID: "R-2", Status: protocol.ReqPass, Severity: "blocking"},
		{ID: "R-3", Status: protocol.ReqFail, Severity: "blocking"},
	}
	out := semantic.MarkUnavailable(reqs, semantic.MergeOptions{
		SemanticModes: map[string]string{"R-1": "required", "R-2": "optional", "R-3": "required"},
	}, "provider down")
	if out[0].Status != protocol.ReqUnknown {
		t.Fatalf("required -> %s", out[0].Status)
	}
	if out[1].Status != protocol.ReqPass {
		t.Fatalf("optional -> %s", out[1].Status)
	}
	if out[2].Status != protocol.ReqFail {
		t.Fatalf("fail stays %s", out[2].Status)
	}
}

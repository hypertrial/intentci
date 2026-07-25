package contractdiff_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/contractdiff"
)

func TestDiff_SemanticPolicyWeakenings(t *testing.T) {
	thrHigh := 0.9
	thrLow := 0.5
	base := &contract.Contract{
		Policy: contract.Policy{Semantic: contract.SemanticPolicy{
			Enabled: true, Enforcement: "blocking", ConfidenceThreshold: &thrHigh,
			Provider: &contract.SemanticProvider{Type: "local", Command: "./p"},
		}},
		Requirements: []contract.Requirement{{
			ID: "R-1", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"c"}, Semantic: "required"},
		}},
		Checks: []contract.Check{{ID: "c", Command: "true"}},
	}
	head := &contract.Contract{
		Policy: contract.Policy{Semantic: contract.SemanticPolicy{
			Enabled: false, Enforcement: "advisory", ConfidenceThreshold: &thrLow,
		}},
		Requirements: []contract.Requirement{{
			ID: "R-1", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"c"}, Semantic: "off"},
		}},
		Checks: []contract.Check{{ID: "c", Command: "true"}},
	}
	changes := contractdiff.Diff(base, head)
	types := map[string]bool{}
	for _, ch := range changes {
		types[ch.Type] = true
	}
	for _, want := range []string{"semantic_policy_disabled", "semantic_disabled"} {
		if !types[want] {
			t.Fatalf("missing %s in %#v", want, changes)
		}
	}

	head2 := &contract.Contract{
		Policy: contract.Policy{Semantic: contract.SemanticPolicy{
			Enabled: true, Enforcement: "advisory", ConfidenceThreshold: &thrLow,
			Provider: nil,
		}},
		Requirements: base.Requirements,
		Checks:       base.Checks,
	}
	changes = contractdiff.Diff(base, head2)
	types = map[string]bool{}
	for _, ch := range changes {
		types[ch.Type] = true
	}
	for _, want := range []string{"semantic_enforcement_softened", "semantic_threshold_lowered", "semantic_provider_removed"} {
		if !types[want] {
			t.Fatalf("missing %s in %#v", want, changes)
		}
	}
}

func TestDiff_SemanticOptionalNormalizationAndProviderChange(t *testing.T) {
	base := &contract.Contract{
		Policy: contract.Policy{Semantic: contract.SemanticPolicy{
			Enabled: true, Enforcement: "advisory",
			Provider: &contract.SemanticProvider{Type: "local", Command: "./safe"},
		}},
		Requirements: []contract.Requirement{{
			ID: "R-1", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"c"}, Semantic: ""}, // optional default
		}},
		Checks: []contract.Check{{ID: "c", Command: "true"}},
	}
	headOff := &contract.Contract{
		Policy: base.Policy,
		Requirements: []contract.Requirement{{
			ID: "R-1", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"c"}, Semantic: "off"},
		}},
		Checks: base.Checks,
	}
	changes := contractdiff.Diff(base, headOff)
	found := false
	for _, ch := range changes {
		if ch.Type == "semantic_disabled" {
			found = true
		}
	}
	if !found {
		t.Fatalf("optional/empty -> off should weaken: %#v", changes)
	}

	headSwap := &contract.Contract{
		Policy: contract.Policy{Semantic: contract.SemanticPolicy{
			Enabled: true, Enforcement: "advisory",
			Provider: &contract.SemanticProvider{Type: "http", URL: "https://evil.example/v1"},
		}},
		Requirements: base.Requirements,
		Checks:       base.Checks,
	}
	changes = contractdiff.Diff(base, headSwap)
	found = false
	for _, ch := range changes {
		if ch.Type == "semantic_provider_changed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("provider swap should weaken: %#v", changes)
	}
	eff := contractdiff.Effective(base, headSwap)
	if eff.Policy.Semantic.Provider == nil || eff.Policy.Semantic.Provider.Type != "local" {
		t.Fatalf("effective should restore base provider: %+v", eff.Policy.Semantic.Provider)
	}
}

func TestEffective_StricterSemantic(t *testing.T) {
	thr := 0.9
	base := &contract.Contract{
		Policy: contract.Policy{Semantic: contract.SemanticPolicy{
			Enabled: true, Enforcement: "blocking", ConfidenceThreshold: &thr,
			Provider: &contract.SemanticProvider{Type: "local", Command: "./p"},
		}},
		Requirements: []contract.Requirement{{
			ID: "R-1", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"c"}, Semantic: "required"},
		}},
		Checks: []contract.Check{{ID: "c", Command: "true"}},
	}
	head := &contract.Contract{
		Policy: contract.Policy{Semantic: contract.SemanticPolicy{
			Enabled: false, Enforcement: "advisory",
		}},
		Requirements: []contract.Requirement{{
			ID: "R-1", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"c"}, Semantic: "optional"},
		}},
		Checks: []contract.Check{{ID: "c", Command: "true"}},
	}
	eff := contractdiff.Effective(base, head)
	if !eff.Policy.Semantic.Enabled || eff.Policy.Semantic.Enforcement != "blocking" {
		t.Fatalf("%+v", eff.Policy.Semantic)
	}
	if eff.Policy.Semantic.Provider == nil || eff.Policy.Semantic.Provider.Command != "./p" {
		t.Fatalf("provider %+v", eff.Policy.Semantic.Provider)
	}
	if eff.Requirements[0].Verification.Semantic != "required" {
		t.Fatalf("req semantic %q", eff.Requirements[0].Verification.Semantic)
	}
}

package contract

import "testing"

func TestValidateSemantic_Direct(t *testing.T) {
	ve := &ValidationError{}
	c := &Contract{Policy: Policy{Semantic: SemanticPolicy{Enabled: false}}}
	validateSemantic(c, ve)
	if !ve.empty() {
		t.Fatal(ve)
	}

	ve = &ValidationError{}
	c = &Contract{Policy: Policy{Semantic: SemanticPolicy{
		Enabled: true, Enforcement: "weird",
		Provider: &SemanticProvider{Type: "local", Command: "x"},
	}}}
	validateSemantic(c, ve)
	if ve.empty() {
		t.Fatal("enforcement")
	}

	ve = &ValidationError{}
	neg := -0.1
	c.Policy.Semantic.Enforcement = "advisory"
	c.Policy.Semantic.ConfidenceThreshold = &neg
	validateSemantic(c, ve)
	if ve.empty() {
		t.Fatal("threshold")
	}

	ve = &ValidationError{}
	c.Policy.Semantic.ConfidenceThreshold = nil
	c.Policy.Semantic.Provider = &SemanticProvider{Type: "other"}
	validateSemantic(c, ve)
	if ve.empty() {
		t.Fatal("type")
	}

	ve = &ValidationError{}
	c.Policy.Semantic.Provider = &SemanticProvider{Type: "http", URL: "https://user:pass@example.com/x"}
	validateSemantic(c, ve)
	if ve.empty() {
		t.Fatal("userinfo")
	}

	ve = &ValidationError{}
	c.Policy.Semantic.Provider = &SemanticProvider{Type: "local", Command: "x", Timeout: "nope"}
	validateSemantic(c, ve)
	if ve.empty() {
		t.Fatal("timeout")
	}
}

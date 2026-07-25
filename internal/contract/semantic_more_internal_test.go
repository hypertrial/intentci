package contract

import (
	"testing"
)

func TestSemanticHelpersAndValidate(t *testing.T) {
	thr := 0.7
	p := SemanticPolicy{Enforcement: "blocking", ConfidenceThreshold: &thr}
	if p.EnforcementOrDefault() != "blocking" || p.ConfidenceThresholdOrDefault() != 0.7 {
		t.Fatal(p)
	}

	c := &Contract{
		Version: 1,
		Product: Product{Name: "x", Purpose: "y"},
		Requirements: []Requirement{{
			ID: "R-1", Type: "behavior", Title: "t", Statement: "s",
			Status: "approved", Severity: "blocking",
			Verification: Verification{Checks: []string{"c"}},
		}},
		Checks: []Check{{ID: "c", Command: "true"}},
		Policy: Policy{Semantic: SemanticPolicy{Enabled: true, Enforcement: "nope", Provider: &SemanticProvider{Type: "local", Command: "x"}}},
	}
	if err := Validate(c); err == nil {
		t.Fatal("bad enforcement")
	}
	c.Policy.Semantic.Enforcement = "advisory"
	c.Policy.Semantic.Provider = &SemanticProvider{Type: "bad"}
	if err := Validate(c); err == nil {
		t.Fatal("bad type")
	}
	c.Policy.Semantic.Provider = &SemanticProvider{Type: "http", URL: ""}
	if err := Validate(c); err == nil {
		t.Fatal("empty url")
	}
	c.Policy.Semantic.Provider = &SemanticProvider{Type: "local", Command: "x", Timeout: "nope"}
	if err := Validate(c); err == nil {
		t.Fatal("bad timeout")
	}
	zero := 0.0
	c.Policy.Semantic.Provider = &SemanticProvider{Type: "local", Command: "x"}
	c.Policy.Semantic.ConfidenceThreshold = &zero
	if err := Validate(c); err == nil {
		t.Fatal("zero threshold")
	}

	m := map[string]any{
		"policy": map[string]any{
			"semantic": map[string]any{
				"enabled":              true,
				"enforcement":          "advisory",
				"confidence_threshold": 0.8,
				"provider": map[string]any{
					"type": "", "command": "", "url": "", "timeout": "",
				},
			},
		},
	}
	normalizeForSchema(m)
	sem := m["policy"].(map[string]any)["semantic"].(map[string]any)
	if _, ok := sem["provider"]; ok {
		t.Fatalf("provider should be stripped: %#v", sem)
	}
}

package contract_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestValidateSemanticPolicy(t *testing.T) {
	base := func() *contract.Contract {
		return &contract.Contract{
			Version: 1,
			Product: contract.Product{Name: "x", Purpose: "y"},
			Requirements: []contract.Requirement{{
				ID: "R-1", Type: "behavior", Title: "t", Statement: "s",
				Status: "approved", Severity: "blocking",
				Verification: contract.Verification{Checks: []string{"c"}},
			}},
			Checks: []contract.Check{{ID: "c", Command: "true"}},
		}
	}

	c := base()
	c.Policy.Semantic = contract.SemanticPolicy{Enabled: true, Enforcement: "advisory"}
	if err := contract.Validate(c); err == nil {
		t.Fatal("missing provider")
	}

	c = base()
	thr := 1.5
	c.Policy.Semantic = contract.SemanticPolicy{
		Enabled: true, Enforcement: "advisory", ConfidenceThreshold: &thr,
		Provider: &contract.SemanticProvider{Type: "local", Command: "./p"},
	}
	if err := contract.Validate(c); err == nil {
		t.Fatal("bad threshold")
	}

	c = base()
	ok := 0.8
	c.Policy.Semantic = contract.SemanticPolicy{
		Enabled: true, Enforcement: "blocking", ConfidenceThreshold: &ok,
		Provider: &contract.SemanticProvider{Type: "http", URL: "https://example.com/v1", Timeout: "1m"},
	}
	if err := contract.Validate(c); err != nil {
		t.Fatal(err)
	}

	c = base()
	c.Policy.Semantic = contract.SemanticPolicy{
		Enabled: true, Enforcement: "advisory",
		Provider: &contract.SemanticProvider{Type: "http", URL: "ftp://bad"},
	}
	if err := contract.Validate(c); err == nil {
		t.Fatal("bad url")
	}

	c = base()
	c.Policy.Semantic = contract.SemanticPolicy{
		Enabled: true, Enforcement: "advisory",
		Provider: &contract.SemanticProvider{Type: "http", URL: "https://user:pass@example.com/v1"},
	}
	if err := contract.Validate(c); err == nil {
		t.Fatal("embedded credentials")
	}

	c = base()
	c.Policy.Semantic = contract.SemanticPolicy{
		Enabled: true, Enforcement: "advisory",
		Provider: &contract.SemanticProvider{Type: "local"},
	}
	if err := contract.Validate(c); err == nil {
		t.Fatal("missing command")
	}

	p := contract.SemanticPolicy{}
	if p.ConfidenceThresholdOrDefault() != contract.DefaultConfidenceThreshold {
		t.Fatal("default threshold")
	}
	if p.EnforcementOrDefault() != "advisory" {
		t.Fatal("default enforcement")
	}
}

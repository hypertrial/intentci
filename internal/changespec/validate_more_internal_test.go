package changespec

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestValidate_SchemaErrorReturns(t *testing.T) {
	s := &Spec{Version: 99}
	c := &contract.Contract{Version: 1, Product: contract.Product{Name: "x", Purpose: "y"}}
	if err := Validate(s, c); err == nil {
		t.Fatal("schema error")
	}
}

func TestValidate_MissingIDWithOtherErrors(t *testing.T) {
	s := &Spec{
		Version: 1, Status: "draft", Type: "feature", Summary: "s", Goals: []string{"g"},
		Acceptance: []Acceptance{
			{ID: "AC-1", Statement: "s", Severity: "blocking", Verification: Verification{Checks: []string{"missing"}}},
			{ID: "AC-1", Statement: "s2", Severity: "blocking", Verification: Verification{Checks: []string{"missing"}}},
		},
	}
	c := validContractForTest()
	if err := Validate(s, c); err == nil {
		t.Fatal("expected errors")
	}
}

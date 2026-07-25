package impact

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestResolve_AddDependency(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{ID: "A-001", Status: "approved", Severity: "blocking", AppliesTo: contract.AppliesTo{Include: []string{"**"}}, Verification: contract.Verification{Checks: []string{"a"}}},
		},
		Checks: []contract.Check{
			{ID: "a", Command: "true", Profiles: []string{"full"}, Inputs: []string{"**"}, DependsOn: []string{"b"}},
			{ID: "b", Command: "true", Profiles: []string{"full"}, Inputs: []string{"**"}},
		},
	}
	sel := Resolve(c, []string{"x.go"}, Options{Profile: "full"})
	foundB := false
	for _, id := range sel.CheckIDs {
		if id == "b" {
			foundB = true
		}
	}
	if !foundB {
		t.Fatalf("expected dependency b in %#v", sel.CheckIDs)
	}
}

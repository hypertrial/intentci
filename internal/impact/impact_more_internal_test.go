package impact

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestResolve_ForceDupAndNoProfile(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{
				ID: "A-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{Include: []string{"a/**"}},
				Verification: contract.Verification{Checks: []string{"fast-only"}},
			},
			{
				ID: "B-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{},
				Verification: contract.Verification{Checks: []string{"full-only"}},
			},
		},
		Checks: []contract.Check{
			{ID: "fast-only", Command: "true", Profiles: []string{"fast"}, Inputs: []string{"a/**"}},
			{ID: "full-only", Command: "true", Profiles: []string{"full"}, Inputs: []string{"**"}},
			{ID: "dep", Command: "true", Profiles: []string{"full"}, DependsOn: []string{"ghost"}},
		},
	}
	sel := Resolve(c, []string{"a/x.go", "z.go"}, Options{
		Profile:             "full",
		ForceRequirementIDs: []string{"A-001"},
		ExtraRequirements: []contract.Requirement{{
			ID: "A-001", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"full-only"}},
		}},
	})
	if len(sel.Requirements) < 2 {
		t.Fatalf("force/extra: %#v", sel.Requirements)
	}
	hasFull := false
	for _, id := range sel.CheckIDs {
		if id == "full-only" {
			hasFull = true
		}
		if id == "fast-only" {
			t.Fatal("fast-only should not be in full profile")
		}
	}
	if !hasFull {
		t.Fatalf("checks %#v", sel.CheckIDs)
	}
}

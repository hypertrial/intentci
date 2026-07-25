package impact

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestResolve_AllForcesStarMatch(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{
				ID: "A-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{Include: []string{"nope/**"}},
				Verification: contract.Verification{Checks: []string{"a"}},
			},
		},
		Checks: []contract.Check{
			{ID: "a", Command: "true", Profiles: []string{"full"}, Inputs: []string{"nope/**"}},
		},
	}
	sel := Resolve(c, []string{"other.go"}, Options{All: true, Profile: "full"})
	if len(sel.Requirements) != 1 || sel.Requirements[0].AffectedBy[0] != "*" {
		t.Fatalf("%#v", sel.Requirements)
	}
}

func TestResolve_CheckEmptyInputs(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{
				ID: "A-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{Include: []string{"a/**"}},
				Verification: contract.Verification{Checks: []string{"a"}},
			},
		},
		Checks: []contract.Check{
			{ID: "a", Command: "true", Profiles: []string{"full"}, Inputs: []string{}},
		},
	}
	sel := Resolve(c, []string{"z.go"}, Options{Profile: "full", ForceRequirementIDs: []string{"A-001"}})
	if len(sel.CheckIDs) != 1 {
		t.Fatalf("%#v", sel.CheckIDs)
	}
}

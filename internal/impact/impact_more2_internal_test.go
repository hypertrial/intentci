package impact

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestResolve_NoInputMatchAndBadDep(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{
				ID: "A-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{Include: []string{"a/**"}},
				Verification: contract.Verification{Checks: []string{"narrow"}},
			},
		},
		Checks: []contract.Check{
			{ID: "narrow", Command: "true", Profiles: []string{"full"}, Inputs: []string{"b/**"}},
			{ID: "ghost", Command: "true", Profiles: []string{"full"}, DependsOn: []string{"missing"}},
		},
	}
	sel := Resolve(c, []string{"z.go"}, Options{Profile: "full", ForceCheckIDs: []string{"narrow"}})
	if len(sel.CheckIDs) != 1 || sel.CheckIDs[0] != "narrow" {
		t.Fatalf("force check only: %#v", sel.CheckIDs)
	}

	c2 := &contract.Contract{
		Requirements: []contract.Requirement{
			{ID: "A-001", Status: "approved", Severity: "blocking", AppliesTo: contract.AppliesTo{Include: []string{"a/**"}}, Verification: contract.Verification{Checks: []string{"a"}}},
		},
		Checks: []contract.Check{
			{ID: "a", Command: "true", Profiles: []string{"full"}, Inputs: []string{"a/**"}},
		},
	}
	sel = Resolve(c2, []string{"a/x.go"}, Options{Profile: "full", ForceRequirementIDs: []string{"A-001"}})
	if len(sel.Requirements) != 1 {
		t.Fatalf("%#v", sel.Requirements)
	}
}

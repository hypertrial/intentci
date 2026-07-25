package impact

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestResolve_AlreadySelectedReq(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{ID: "A-001", Status: "approved", Severity: "blocking", AppliesTo: contract.AppliesTo{Include: []string{"a/**"}}, Verification: contract.Verification{Checks: []string{"a"}}},
		},
		Checks: []contract.Check{{ID: "a", Command: "true", Profiles: []string{"full"}, Inputs: []string{"a/**"}}},
	}
	sel := Resolve(c, []string{"a/x.go"}, Options{
		Profile:             "full",
		ForceRequirementIDs: []string{"A-001"},
		ExtraRequirements: []contract.Requirement{
			{ID: "A-001", Status: "approved", Severity: "blocking", Verification: contract.Verification{Checks: []string{"a"}}},
		},
	})
	if len(sel.Requirements) != 1 {
		t.Fatalf("%#v", sel.Requirements)
	}
}

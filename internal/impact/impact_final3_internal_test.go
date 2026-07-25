package impact

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestResolve_MissingCheckInReq(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{
				ID: "A-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{Include: []string{"a/**"}},
				Verification: contract.Verification{Checks: []string{"missing", "a"}},
			},
		},
		Checks: []contract.Check{
			{ID: "a", Command: "true", Profiles: []string{"full"}, Inputs: []string{"a/**"}},
		},
	}
	sel := Resolve(c, []string{"a/x.go"}, Options{Profile: "full"})
	if len(sel.CheckIDs) != 1 || sel.CheckIDs[0] != "a" {
		t.Fatalf("%#v", sel.CheckIDs)
	}
}

func TestResolve_CheckSetMissingDep(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{ID: "A-001", Status: "approved", Severity: "blocking", AppliesTo: contract.AppliesTo{Include: []string{"a/**"}}, Verification: contract.Verification{Checks: []string{"a"}}},
		},
		Checks: []contract.Check{
			{ID: "a", Command: "true", Profiles: []string{"full"}, Inputs: []string{"a/**"}, DependsOn: []string{"ghost"}},
		},
	}
	sel := Resolve(c, []string{"a/x.go"}, Options{Profile: "full"})
	if len(sel.CheckIDs) != 1 {
		t.Fatalf("%#v", sel.CheckIDs)
	}
}

package impact

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestResolve_DepMissingCheck(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{
				ID: "A-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{Include: []string{"a/**"}},
				Verification: contract.Verification{Checks: []string{"a"}},
			},
		},
		Checks: []contract.Check{
			{ID: "a", Command: "true", Profiles: []string{"full"}, Inputs: []string{"a/**"}, DependsOn: []string{"missing"}},
		},
	}
	sel := Resolve(c, []string{"a/x.go"}, Options{Profile: "full"})
	if len(sel.CheckIDs) != 1 || sel.CheckIDs[0] != "a" {
		t.Fatalf("%#v", sel.CheckIDs)
	}
}

func TestResolve_EmptyIncludeUsesChanged(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{
				ID: "A-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{},
				Verification: contract.Verification{Checks: []string{"a"}},
			},
		},
		Checks: []contract.Check{
			{ID: "a", Command: "true", Profiles: []string{"full"}},
		},
	}
	sel := Resolve(c, []string{"readme.md"}, Options{Profile: "full"})
	if len(sel.Requirements) != 1 {
		t.Fatalf("%#v", sel.Requirements)
	}
}

func TestResolve_ForceCheckNoProfile(t *testing.T) {
	c := &contract.Contract{
		Checks: []contract.Check{{ID: "fast", Command: "true", Profiles: []string{"fast"}}},
	}
	sel := Resolve(c, nil, Options{Profile: "full", ForceCheckIDs: []string{"fast"}})
	if len(sel.CheckIDs) != 0 {
		t.Fatalf("%#v", sel.CheckIDs)
	}
}

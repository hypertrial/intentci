package impact

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestResolve_DepWrongProfile(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{ID: "A-001", Status: "approved", Severity: "blocking", AppliesTo: contract.AppliesTo{Include: []string{"**"}}, Verification: contract.Verification{Checks: []string{"a"}}},
		},
		Checks: []contract.Check{
			{ID: "a", Command: "true", Profiles: []string{"full"}, Inputs: []string{"**"}, DependsOn: []string{"fast"}},
			{ID: "fast", Command: "true", Profiles: []string{"fast"}},
		},
	}
	sel := Resolve(c, []string{"x.go"}, Options{Profile: "full"})
	for _, id := range sel.CheckIDs {
		if id == "fast" {
			t.Fatal("fast dep should not be included")
		}
	}
}

func TestResolve_ForcedCheckWrongProfile(t *testing.T) {
	c := &contract.Contract{
		Checks: []contract.Check{{ID: "fast", Command: "true", Profiles: []string{"fast"}}},
	}
	sel := Resolve(c, nil, Options{Profile: "full", ForceCheckIDs: []string{"fast", "missing"}})
	if len(sel.CheckIDs) != 0 {
		t.Fatalf("%#v", sel.CheckIDs)
	}
}

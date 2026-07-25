package impact

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestResolve_EmptyIncludeUsesAllChanged(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{
				ID: "A-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{},
				Verification: contract.Verification{Checks: []string{"a"}},
			},
		},
		Checks: []contract.Check{
			{ID: "a", Command: "true", Profiles: []string{"full"}, Inputs: []string{"z/**"}},
		},
	}
	sel := Resolve(c, []string{"readme.md"}, Options{Profile: "full"})
	if len(sel.Requirements) != 1 || len(sel.Requirements[0].AffectedBy) == 0 {
		t.Fatalf("%#v", sel.Requirements)
	}
}

package impact_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/impact"
)

func TestResolve_PathMatching(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{
				ID:       "STATE-001",
				Status:   "approved",
				Severity: "blocking",
				AppliesTo: contract.AppliesTo{
					Include: []string{"src/state/**"},
					Exclude: []string{"src/state/**/*_test.go"},
				},
				Verification: contract.Verification{Checks: []string{"state-unit"}},
			},
			{
				ID:       "API-001",
				Status:   "approved",
				Severity: "blocking",
				AppliesTo: contract.AppliesTo{
					Include: []string{"src/api/**"},
				},
				Verification: contract.Verification{Checks: []string{"api-unit"}},
			},
		},
		Checks: []contract.Check{
			{ID: "state-unit", Command: "true", Profiles: []string{"full"}, Inputs: []string{"src/state/**"}},
			{ID: "api-unit", Command: "true", Profiles: []string{"full"}, Inputs: []string{"src/api/**"}},
		},
	}
	sel := impact.Resolve(c, []string{"src/state/commit.go", "README.md"}, impact.Options{Profile: "full"})
	if len(sel.Requirements) != 1 || sel.Requirements[0].Requirement.ID != "STATE-001" {
		t.Fatalf("expected STATE-001 only, got %#v", sel.Requirements)
	}
	if len(sel.CheckIDs) != 1 || sel.CheckIDs[0] != "state-unit" {
		t.Fatalf("expected state-unit, got %#v", sel.CheckIDs)
	}
}

func TestResolve_All(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{
				ID: "A-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{Include: []string{"a/**"}},
				Verification: contract.Verification{Checks: []string{"a"}},
			},
			{
				ID: "B-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{Include: []string{"b/**"}},
				Verification: contract.Verification{Checks: []string{"b"}},
			},
		},
		Checks: []contract.Check{
			{ID: "a", Command: "true", Profiles: []string{"full"}},
			{ID: "b", Command: "true", Profiles: []string{"full"}},
		},
	}
	sel := impact.Resolve(c, []string{"a/x.go"}, impact.Options{All: true, Profile: "full"})
	if len(sel.Requirements) != 2 {
		t.Fatalf("expected 2 requirements, got %d", len(sel.Requirements))
	}
}

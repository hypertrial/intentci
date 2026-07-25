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
				ID: "STATE-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{Include: []string{"src/state/**"}, Exclude: []string{"src/state/**/*_test.go"}},
				Verification: contract.Verification{Checks: []string{"state-unit"}},
			},
			{
				ID: "API-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{Include: []string{"src/api/**"}},
				Verification: contract.Verification{Checks: []string{"api-unit"}},
			},
		},
		Checks: []contract.Check{
			{ID: "state-unit", Command: "true", Profiles: []string{"full"}, Inputs: []string{"src/state/**"}},
			{ID: "api-unit", Command: "true", Profiles: []string{"full"}, Inputs: []string{"src/api/**"}, DependsOn: []string{"state-unit"}},
		},
	}
	sel := impact.Resolve(c, []string{"src/state/commit.go", "README.md"}, impact.Options{Profile: "full"})
	if len(sel.Requirements) != 1 || sel.Requirements[0].Requirement.ID != "STATE-001" {
		t.Fatalf("%#v", sel.Requirements)
	}
}

func TestResolve_AllForceExtra(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{ID: "A-001", Status: "approved", Severity: "blocking", AppliesTo: contract.AppliesTo{Include: []string{"a/**"}}, Verification: contract.Verification{Checks: []string{"a"}}},
			{ID: "B-001", Status: "approved", Severity: "blocking", AppliesTo: contract.AppliesTo{Include: []string{"b/**"}}, Verification: contract.Verification{Checks: []string{"b"}}},
			{ID: "C-001", Status: "approved", Severity: "blocking", AppliesTo: contract.AppliesTo{}, Verification: contract.Verification{Checks: []string{"c"}}},
		},
		Checks: []contract.Check{
			{ID: "a", Command: "true", Profiles: []string{"full"}},
			{ID: "b", Command: "true", Profiles: []string{"full"}},
			{ID: "c", Command: "true", Profiles: []string{"full"}},
			{ID: "d", Command: "true", Profiles: []string{"full"}},
		},
	}
	sel := impact.Resolve(c, []string{"a/x.go"}, impact.Options{All: true, Profile: "full"})
	if len(sel.Requirements) != 3 {
		t.Fatalf("all: %d", len(sel.Requirements))
	}
	sel = impact.Resolve(c, []string{"readme"}, impact.Options{
		Profile:             "full",
		ForceRequirementIDs: []string{"B-001"},
		ForceCheckIDs:       []string{"d"},
		ExtraRequirements: []contract.Requirement{{
			ID: "AC-001", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"a"}},
		}},
	})
	ids := map[string]bool{}
	for _, r := range sel.Requirements {
		ids[r.Requirement.ID] = true
	}
	if !ids["B-001"] || !ids["AC-001"] || !ids["C-001"] {
		t.Fatalf("%v", ids)
	}
	foundD := false
	for _, id := range sel.CheckIDs {
		if id == "d" {
			foundD = true
		}
	}
	if !foundD {
		t.Fatalf("checks %#v", sel.CheckIDs)
	}
}

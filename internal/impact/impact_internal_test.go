package impact

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestResolve_ExcludeAnyMatchForceDupProfile(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{
				ID: "A-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{Include: []string{"src/**"}, Exclude: []string{"src/secret/**"}},
				Verification: contract.Verification{Checks: []string{"a-check"}},
			},
			{
				ID: "B-001", Status: "approved", Severity: "blocking",
				AppliesTo:    contract.AppliesTo{Include: []string{"other/**"}},
				Verification: contract.Verification{Checks: []string{"b-check"}},
			},
		},
		Checks: []contract.Check{
			{ID: "a-check", Command: "true", Profiles: []string{"full"}, Inputs: []string{"src/**"}},
			{ID: "b-check", Command: "true", Profiles: []string{"fast"}, Inputs: []string{"other/**"}},
			{ID: "orphan", Command: "true", Profiles: []string{"full"}, Inputs: []string{"nope/**"}},
		},
	}
	sel := impactResolve(c, []string{"src/secret/x.go", "src/ok.go"}, Options{Profile: "full"})
	if len(sel.Requirements) != 1 || sel.Requirements[0].Requirement.ID != "A-001" {
		t.Fatalf("exclude: %#v", sel.Requirements)
	}
	foundA := false
	for _, id := range sel.CheckIDs {
		if id == "a-check" {
			foundA = true
		}
		if id == "orphan" {
			t.Fatal("orphan should not match")
		}
	}
	if !foundA {
		t.Fatalf("checks %#v", sel.CheckIDs)
	}

	sel = Resolve(c, []string{"readme"}, Options{
		Profile:             "full",
		ForceRequirementIDs: []string{"A-001", "A-001"},
	})
	if len(sel.Requirements) != 1 {
		t.Fatalf("force dup: %d", len(sel.Requirements))
	}

	sel = Resolve(c, []string{"other/x.go"}, Options{Profile: "fast"})
	foundB := false
	for _, id := range sel.CheckIDs {
		if id == "b-check" {
			foundB = true
		}
	}
	if !foundB {
		t.Fatalf("profile check: %#v", sel.CheckIDs)
	}
}

func impactResolve(c *contract.Contract, changed []string, opt Options) Selection {
	return Resolve(c, changed, opt)
}

func TestPathMatchesAndAnyMatch(t *testing.T) {
	if pathMatches("x.go", contract.AppliesTo{}) {
		t.Fatal("empty include")
	}
	if pathMatches("secret/x.go", contract.AppliesTo{Include: []string{"**"}, Exclude: []string{"secret/**"}}) {
		t.Fatal("exclude should block")
	}
	if anyMatch([]string{"a.go"}, []string{"b/**"}) {
		t.Fatal("anyMatch miss")
	}
	if !anyMatch([]string{"a.go"}, []string{"**"}) {
		t.Fatal("anyMatch hit")
	}
}

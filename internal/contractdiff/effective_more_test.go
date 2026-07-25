package contractdiff_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/contractdiff"
)

func TestEffective_RestoreDeletedCheckAndNarrow(t *testing.T) {
	base := &contract.Contract{
		Version: 1,
		Product: contract.Product{Name: "x", Purpose: "y"},
		Requirements: []contract.Requirement{{
			ID: "A-001", Type: "behavior", Title: "t", Statement: "s",
			Status: "approved", Severity: "blocking",
			AppliesTo:     contract.AppliesTo{Include: []string{"a/**", "b/**"}},
			Verification: contract.Verification{Mode: "all", Checks: []string{"unit", "int"}},
		}},
		Checks: []contract.Check{
			{ID: "unit", Command: "true", Timeout: "1m"},
			{ID: "int", Command: "true", Timeout: "1m", DependsOn: []string{"unit"}},
		},
	}
	head := &contract.Contract{
		Version: 1,
		Product: contract.Product{Name: "x", Purpose: "y"},
		Requirements: []contract.Requirement{{
			ID: "A-001", Type: "behavior", Title: "t", Statement: "s",
			Status: "approved", Severity: "blocking",
			AppliesTo:     contract.AppliesTo{Include: []string{"a/**"}, Exclude: []string{"a/vendor/**"}},
			Verification: contract.Verification{Mode: "any", Checks: []string{"unit"}},
		}, {
			ID: "B-001", Type: "behavior", Title: "new", Statement: "s",
			Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"unit"}},
		}},
		Checks: []contract.Check{
			{ID: "unit", Command: "false", Timeout: "2m"},
		},
	}
	eff := contractdiff.Effective(base, head)
	if _, ok := eff.CheckByID("int"); !ok {
		t.Fatal("expected restored int check")
	}
	unit, _ := eff.CheckByID("unit")
	if unit.Command != "true" {
		t.Fatalf("expected base unit command, got %s", unit.Command)
	}
	reqs := eff.ApprovedBlocking()
	if len(reqs) < 2 {
		t.Fatalf("%+v", reqs)
	}
}

func TestDiff_UnusedCheckDeleteIgnored(t *testing.T) {
	base := &contract.Contract{
		Requirements: []contract.Requirement{{
			ID: "A-001", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"unit"}},
		}},
		Checks: []contract.Check{
			{ID: "unit", Command: "true"},
			{ID: "orphan", Command: "true"},
		},
	}
	head := &contract.Contract{
		Requirements: base.Requirements,
		Checks:       []contract.Check{{ID: "unit", Command: "true"}},
	}
	for _, c := range contractdiff.Diff(base, head) {
		if c.ID == "orphan" {
			t.Fatalf("orphan should not flag: %+v", c)
		}
	}
}

func TestNarrowedAppliesTo_AllPaths(t *testing.T) {
	base := baseContract()
	base.Requirements[0].AppliesTo = contract.AppliesTo{}
	head := baseContract()
	head.Requirements[0].AppliesTo = contract.AppliesTo{Include: []string{"cmd/**"}}
	changes := contractdiff.Diff(base, head)
	found := false
	for _, c := range changes {
		if c.Type == "applies_to_narrowed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", changes)
	}
}

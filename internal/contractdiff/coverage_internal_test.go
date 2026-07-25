package contractdiff

import (
	"errors"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestWeakenedBranches(t *testing.T) {
	base := contract.Requirement{
		Status: "approved", Severity: "blocking",
		AppliesTo:     contract.AppliesTo{Include: []string{"a/**", "b/**"}},
		Verification: contract.Verification{Mode: "all", Checks: []string{"u", "i"}},
	}
	if !weakened(base, contract.Requirement{Status: "approved", Severity: "advisory", Verification: base.Verification, AppliesTo: base.AppliesTo}) {
		t.Fatal("severity")
	}
	if !weakened(base, contract.Requirement{Status: "approved", Severity: "blocking", AppliesTo: base.AppliesTo, Verification: contract.Verification{Mode: "any", Checks: []string{"u", "i"}}}) {
		t.Fatal("mode")
	}
	if !weakened(base, contract.Requirement{Status: "approved", Severity: "blocking", AppliesTo: base.AppliesTo, Verification: contract.Verification{Mode: "all", Checks: []string{"u"}}}) {
		t.Fatal("checks")
	}
	if !weakened(base, contract.Requirement{Status: "approved", Severity: "blocking", AppliesTo: contract.AppliesTo{Include: []string{"a/**"}}, Verification: base.Verification}) {
		t.Fatal("applies")
	}
	if weakened(base, base) {
		t.Fatal("equal")
	}
	if narrowedAppliesTo(contract.AppliesTo{Include: []string{"a/**"}}, contract.AppliesTo{Include: []string{"a/**", "b/**"}}) {
		t.Fatal("widened include is not narrowing")
	}
	if narrowedAppliesTo(contract.AppliesTo{Include: []string{"cmd/**"}}, contract.AppliesTo{Include: []string{"**"}}) {
		t.Fatal("broadening to ** is not narrowing")
	}
	if narrowedAppliesTo(contract.AppliesTo{Include: []string{"cmd/**"}}, contract.AppliesTo{Include: []string{"pkg/**"}}) {
		t.Fatal("pattern rewrite is not narrowing")
	}
	if !narrowedAppliesTo(contract.AppliesTo{Include: []string{"a/**"}}, contract.AppliesTo{Include: []string{"a/**"}, Exclude: []string{"a/vendor/**"}}) {
		t.Fatal("exclude growth is narrowing")
	}
	if narrowedAppliesTo(contract.AppliesTo{Include: []string{"a/**"}}, contract.AppliesTo{Include: []string{"a/**"}}) {
		t.Fatal("identical applies_to")
	}
}

func TestDiff_PolicyAndResults(t *testing.T) {
	f := false
	base := &contract.Contract{
		Policy: contract.Policy{},
		Requirements: []contract.Requirement{{
			ID: "A-001", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"unit"}},
		}},
		Checks: []contract.Check{{
			ID: "unit", Command: "true",
			Results: &contract.Results{Format: "junit", Path: "out.xml"},
		}},
	}
	head := &contract.Contract{
		Policy: contract.Policy{UnknownBlocks: &f, UnverifiedBlocks: &f},
		Requirements: []contract.Requirement{{
			ID: "A-001", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"unit"}},
		}},
		Checks: []contract.Check{{ID: "unit", Command: "true"}},
	}
	changes := Diff(base, head)
	types := map[string]bool{}
	for _, c := range changes {
		types[c.Type] = true
	}
	if !types["policy_unknown_blocks_disabled"] || !types["policy_unverified_blocks_disabled"] || !types["check_modified"] {
		t.Fatalf("%+v", changes)
	}
	eff := Effective(base, head)
	if !eff.Policy.BlocksOnUnknown() || !eff.Policy.BlocksOnUnverified() {
		t.Fatalf("expected stricter policy: %+v", eff.Policy)
	}
	unit, _ := eff.CheckByID("unit")
	if unit.Results == nil || unit.Results.Format != "junit" {
		t.Fatalf("expected restored junit results: %+v", unit.Results)
	}
}

func TestGitErrMessage(t *testing.T) {
	if gitErrMessage("  boom  ", errors.New("x")) != "boom" {
		t.Fatal("stderr")
	}
	if gitErrMessage("", errors.New("fallback")) != "fallback" {
		t.Fatal("empty stderr")
	}
}

func TestResultsEqual(t *testing.T) {
	if !resultsEqual(nil, nil) {
		t.Fatal("both nil")
	}
	a := &contract.Results{Format: "junit", Path: "a.xml"}
	if resultsEqual(nil, a) || resultsEqual(a, nil) {
		t.Fatal("nil mismatch")
	}
	b := &contract.Results{Format: "junit", Path: "a.xml"}
	if !resultsEqual(a, b) {
		t.Fatal("equal")
	}
	b.Path = "b.xml"
	if resultsEqual(a, b) {
		t.Fatal("path mismatch")
	}
}

func TestEffective_RemovedBaseReq(t *testing.T) {
	base := &contract.Contract{
		Requirements: []contract.Requirement{{
			ID: "A-001", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"unit"}},
		}},
		Checks: []contract.Check{{ID: "unit", Command: "true"}, {ID: "orphan", Command: "true"}},
	}
	head := &contract.Contract{
		Requirements: []contract.Requirement{{
			ID: "B-001", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"unit"}},
		}},
		Checks: []contract.Check{{ID: "unit", Command: "true"}},
	}
	eff := Effective(base, head)
	found := false
	for _, r := range eff.Requirements {
		if r.ID == "A-001" && r.Status == "approved" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", eff.Requirements)
	}
}

func TestDiff_SkipDraftBaseAndSort(t *testing.T) {
	base := &contract.Contract{
		Requirements: []contract.Requirement{
			{ID: "D-001", Status: "draft", Severity: "blocking", Verification: contract.Verification{Checks: []string{"u"}}},
			{ID: "A-001", Status: "approved", Severity: "blocking", Verification: contract.Verification{Checks: []string{"u"}}},
			{ID: "B-001", Status: "approved", Severity: "blocking", Verification: contract.Verification{Checks: []string{"u"}}},
		},
		Checks: []contract.Check{{ID: "u", Command: "true"}},
	}
	head := &contract.Contract{
		Requirements: nil,
		Checks:       nil,
	}
	changes := Diff(base, head)
	if len(changes) < 2 {
		t.Fatalf("%+v", changes)
	}
}

func TestRunGit_EmptyStderr(t *testing.T) {
	// Force a failing git invocation; stderr may be empty for some failures.
	_, err := runGit("/", "rev-parse", "--verify", "refs/does-not-exist-xyz")
	if err == nil {
		t.Fatal("expected error")
	}
}

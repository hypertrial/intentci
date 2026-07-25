package evidence_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestAssign_PassFailUnverifiedUnknown(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{
			{ID: "A-001", Title: "A", Status: "approved", Severity: "blocking", Verification: contract.Verification{Mode: "all", Checks: []string{"ok"}}},
			{ID: "B-001", Title: "B", Status: "approved", Severity: "blocking", Verification: contract.Verification{Mode: "all", Checks: []string{"bad"}}},
			{ID: "C-001", Title: "C", Status: "approved", Severity: "blocking", Verification: contract.Verification{Mode: "all", Checks: []string{"missing"}}},
			{ID: "D-001", Title: "D", Status: "approved", Severity: "blocking", Verification: contract.Verification{Mode: "all", Checks: []string{"slow"}}},
			{ID: "E-001", Title: "E", Status: "approved", Severity: "blocking", Verification: contract.Verification{Mode: "any", Checks: []string{"ok", "bad"}}},
			{ID: "F-001", Title: "F", Status: "approved", Severity: "blocking", Verification: contract.Verification{Mode: "all", Checks: []string{"fast-only"}}},
			{ID: "G-001", Title: "G", Status: "approved", Severity: "blocking", Verification: contract.Verification{Mode: "all", Checks: []string{"skip"}}},
			{ID: "H-001", Title: "H", Status: "approved", Severity: "blocking", Verification: contract.Verification{Mode: "any", Checks: []string{"bad"}}},
			{ID: "I-001", Title: "I", Status: "approved", Severity: "blocking", Verification: contract.Verification{Mode: "all", Checks: []string{"gone"}}},
		},
		Checks: []contract.Check{
			{ID: "ok", Command: "true", Profiles: []string{"full"}},
			{ID: "bad", Command: "false", Profiles: []string{"full"}},
			{ID: "missing", Command: "true", Profiles: []string{"full"}},
			{ID: "slow", Command: "true", Profiles: []string{"full"}},
			{ID: "fast-only", Command: "true", Profiles: []string{"fast"}},
			{ID: "skip", Command: "true", Profiles: []string{"full"}},
		},
	}
	zero, one := 0, 1
	sel := impact.Selection{Requirements: []impact.SelectedRequirement{
		{Requirement: c.Requirements[0]}, {Requirement: c.Requirements[1]}, {Requirement: c.Requirements[2]},
		{Requirement: c.Requirements[3]}, {Requirement: c.Requirements[4]}, {Requirement: c.Requirements[5]},
		{Requirement: c.Requirements[6]}, {Requirement: c.Requirements[7]}, {Requirement: c.Requirements[8]},
	}}
	results := map[string]runner.Result{
		"ok":   {Status: protocol.CheckPass, ExitCode: &zero},
		"bad":  {Status: protocol.CheckFail, ExitCode: &one, Reason: "failed", Stderr: "err\nline"},
		"slow": {Status: protocol.CheckUnknown, Reason: "timed out"},
		"skip": {Status: protocol.CheckSkipped, Reason: "dep"},
	}
	got := evidence.Assign(sel, results, "full", c, nil)
	want := map[string]string{
		"A-001": protocol.ReqPass, "B-001": protocol.ReqFail, "C-001": protocol.ReqUnverified,
		"D-001": protocol.ReqUnknown, "E-001": protocol.ReqFail, "F-001": protocol.ReqUnverified,
		"G-001": protocol.ReqUnverified, "H-001": protocol.ReqFail, "I-001": protocol.ReqUnverified,
	}
	for _, r := range got {
		if want[r.ID] != r.Status {
			t.Fatalf("%s: got %s want %s (%s)", r.ID, r.Status, want[r.ID], r.Reason)
		}
	}
	s := evidence.Summarize(got, 2)
	if s.Pass+s.Fail+s.Unverified+s.Unknown == 0 {
		t.Fatal(s)
	}
	s = evidence.Summarize([]protocol.RequirementResult{{Status: protocol.ReqNotAffected}}, 0)
	if s.NotAffected != 1 {
		t.Fatal(s)
	}
}

func TestOverall_ExitCodes(t *testing.T) {
	unknown := true
	unverified := true
	policy := contract.Policy{UnknownBlocks: &unknown, UnverifiedBlocks: &unverified}
	cases := []struct {
		status string
		code   int
		reqs   []protocol.RequirementResult
	}{
		{protocol.StatusPass, 0, []protocol.RequirementResult{{Status: protocol.ReqPass, Severity: "blocking"}}},
		{protocol.StatusFail, 10, []protocol.RequirementResult{{Status: protocol.ReqFail, Severity: "blocking"}}},
		{protocol.StatusUnverified, 11, []protocol.RequirementResult{{Status: protocol.ReqUnverified, Severity: "blocking"}}},
		{protocol.StatusUnknown, 12, []protocol.RequirementResult{{Status: protocol.ReqUnknown, Severity: "blocking"}}},
		{protocol.StatusPass, 0, []protocol.RequirementResult{{Status: protocol.ReqFail, Severity: "advisory"}}},
	}
	for _, tc := range cases {
		st, code := evidence.Overall(tc.reqs, policy)
		if st != tc.status || code != tc.code {
			t.Fatalf("got %s/%d want %s/%d", st, code, tc.status, tc.code)
		}
	}
	f := false
	policy.UnknownBlocks = &f
	policy.UnverifiedBlocks = &f
	st, code := evidence.Overall([]protocol.RequirementResult{{Status: protocol.ReqUnknown, Severity: "blocking"}}, policy)
	if st != protocol.StatusUnknown || code != 0 {
		t.Fatalf("%s %d", st, code)
	}
	st, code = evidence.Overall([]protocol.RequirementResult{{Status: protocol.ReqUnverified, Severity: "blocking"}}, policy)
	if st != protocol.StatusUnverified || code != 0 {
		t.Fatalf("%s %d", st, code)
	}
}

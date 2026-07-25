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
			{
				ID:       "A-001",
				Title:    "A",
				Status:   "approved",
				Severity: "blocking",
				Verification: contract.Verification{
					Mode:   "all",
					Checks: []string{"ok"},
				},
			},
			{
				ID:       "B-001",
				Title:    "B",
				Status:   "approved",
				Severity: "blocking",
				Verification: contract.Verification{
					Mode:   "all",
					Checks: []string{"bad"},
				},
			},
			{
				ID:       "C-001",
				Title:    "C",
				Status:   "approved",
				Severity: "blocking",
				Verification: contract.Verification{
					Mode:   "all",
					Checks: []string{"missing"},
				},
			},
			{
				ID:       "D-001",
				Title:    "D",
				Status:   "approved",
				Severity: "blocking",
				Verification: contract.Verification{
					Mode:   "all",
					Checks: []string{"slow"},
				},
			},
		},
		Checks: []contract.Check{
			{ID: "ok", Command: "true", Profiles: []string{"full"}},
			{ID: "bad", Command: "false", Profiles: []string{"full"}},
			{ID: "missing", Command: "true", Profiles: []string{"full"}},
			{ID: "slow", Command: "true", Profiles: []string{"full"}},
		},
	}
	zero, one := 0, 1
	sel := impact.Selection{
		Requirements: []impact.SelectedRequirement{
			{Requirement: c.Requirements[0]},
			{Requirement: c.Requirements[1]},
			{Requirement: c.Requirements[2]},
			{Requirement: c.Requirements[3]},
		},
	}
	results := map[string]runner.Result{
		"ok":   {Status: protocol.CheckPass, ExitCode: &zero},
		"bad":  {Status: protocol.CheckFail, ExitCode: &one, Reason: "failed"},
		"slow": {Status: protocol.CheckUnknown, Reason: "timed out"},
	}
	got := evidence.Assign(sel, results, "full", c)
	want := map[string]string{
		"A-001": protocol.ReqPass,
		"B-001": protocol.ReqFail,
		"C-001": protocol.ReqUnverified,
		"D-001": protocol.ReqUnknown,
	}
	for _, r := range got {
		if want[r.ID] != r.Status {
			t.Fatalf("%s: got %s want %s", r.ID, r.Status, want[r.ID])
		}
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
	}
	for _, tc := range cases {
		st, code := evidence.Overall(tc.reqs, policy)
		if st != tc.status || code != tc.code {
			t.Fatalf("got %s/%d want %s/%d", st, code, tc.status, tc.code)
		}
	}
}

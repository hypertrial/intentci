package evidence_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestAssign_Waived(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{{
			ID: "BUILD-001", Title: "t", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"go-test"}},
		}},
		Checks: []contract.Check{{ID: "go-test", Command: "false", Profiles: []string{"full"}}},
	}
	sel := impact.Selection{Requirements: []impact.SelectedRequirement{{Requirement: c.Requirements[0]}}}
	one := 1
	results := map[string]runner.Result{"go-test": {Status: protocol.CheckFail, ExitCode: &one}}
	waived := map[string]protocol.Waiver{
		"BUILD-001": {ID: "W-001", Requirement: "BUILD-001", Reason: "temp"},
	}
	got := evidence.Assign(sel, results, "full", c, waived)
	if len(got) != 1 || got[0].Status != protocol.ReqWaived {
		t.Fatalf("%+v", got)
	}
	status, code := evidence.Overall(got, contract.Policy{})
	if status != protocol.StatusPass || code != 0 {
		t.Fatalf("%s %d", status, code)
	}
	sum := evidence.Summarize(got, 0)
	if sum.Waived != 1 {
		t.Fatalf("%+v", sum)
	}
}

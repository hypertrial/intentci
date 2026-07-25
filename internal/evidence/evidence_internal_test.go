package evidence

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestAssign_ReasonFromFinding(t *testing.T) {
	c := &contract.Contract{
		Requirements: []contract.Requirement{{
			ID: "X-001", Title: "x", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Mode: "all", Checks: []string{"bad"}},
		}},
		Checks: []contract.Check{{ID: "bad", Command: "false", Profiles: []string{"full"}}},
	}
	one := 1
	sel := impact.Selection{Requirements: []impact.SelectedRequirement{{Requirement: c.Requirements[0]}}}
	got := Assign(sel, map[string]runner.Result{
		"bad": {Status: protocol.CheckFail, ExitCode: &one, Stdout: "only stdout"},
	}, "full", c)
	if len(got) != 1 || got[0].Reason == "" {
		t.Fatalf("%+v", got)
	}
}

func TestFirstLine_StdoutOnly(t *testing.T) {
	if firstLine("", "line1\nline2") != "line1" {
		t.Fatal(firstLine("", "line1\nline2"))
	}
}

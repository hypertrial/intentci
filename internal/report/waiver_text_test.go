package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/report"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestWriteText_WaiversAndContractChanges(t *testing.T) {
	var buf bytes.Buffer
	res := &protocol.Result{
		Status:     protocol.StatusPass,
		BaseCommit: "a",
		HeadCommit: "b",
		Profile:    "full",
		ContractChanges: []protocol.ContractChange{{
			Type: "requirement_removed", Summary: "gone", ID: "X-1",
		}},
		Waivers: []protocol.Waiver{{
			ID: "W-001", Requirement: "X-1", Reason: "temp", Approver: "bob", Expires: "2099-01-01",
		}},
		Requirements: []protocol.RequirementResult{{
			ID: "X-1", Title: "t", Status: protocol.ReqWaived, Severity: "blocking",
		}},
		Summary: protocol.Summary{Pass: 0, Waived: 1},
	}
	if err := report.WriteText(&buf, res); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, want := range []string{"Contract changes", "Waivers", "W-001", "bob", "waived"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
}

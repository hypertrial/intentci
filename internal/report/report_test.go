package report_test

import (
	"bytes"
	"testing"

	"github.com/hypertrial/intentci/internal/report"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestWriteJSON_ValidatesAgainstSchema(t *testing.T) {
	res := &protocol.Result{
		SchemaVersion: 1,
		RunID:         "01TEST",
		Status:        protocol.StatusPass,
		BaseCommit:    "abc1234",
		HeadCommit:    "def5678",
		ContractHash:  "sha256:deadbeef",
		Profile:       "full",
		Requirements: []protocol.RequirementResult{
			{
				ID:       "BUILD-001",
				Status:   protocol.ReqPass,
				Severity: "blocking",
				AffectedBy: []string{},
				Checks:   []protocol.CheckRef{},
				Evidence: []protocol.Evidence{},
				Findings: []protocol.Finding{},
			},
		},
		Checks:          []protocol.CheckResult{},
		Waivers:         []any{},
		ContractChanges: []any{},
		Summary: protocol.Summary{
			Pass: 1,
		},
	}
	if err := report.ValidateResultSchema(res); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
	var buf bytes.Buffer
	if err := report.WriteJSON(&buf, res); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("empty json")
	}
}

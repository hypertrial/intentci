package report_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/report"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func sampleResult(status string) *protocol.Result {
	return &protocol.Result{
		SchemaVersion:    1,
		RunID:            "01TEST",
		Status:           status,
		BaseCommit:       "abc1234",
		HeadCommit:       "def5678",
		ContractHash:     "sha256:deadbeef",
		WorkingTreeDirty: true,
		Profile:          "full",
		ChangeSpec:       &protocol.ChangeSpecRef{ID: "DEMO-1", Hash: "h"},
		ChangeFindings:   []protocol.ChangeFinding{{Type: "change_spec_modified", Summary: "changed"}},
		Requirements: []protocol.RequirementResult{
			{ID: "A-001", Title: "ok", Status: protocol.ReqPass, Severity: "blocking", AffectedBy: []string{}, Checks: []protocol.CheckRef{}, Evidence: []protocol.Evidence{}, Findings: []protocol.Finding{}},
			{ID: "B-001", Title: "bad", Status: protocol.ReqFail, Severity: "blocking", AffectedBy: []string{"x.go"}, Checks: []protocol.CheckRef{}, Evidence: []protocol.Evidence{}, Findings: []protocol.Finding{{Type: "deterministic_failure", Summary: "boom"}}, Reason: "failed"},
		},
		Checks:          []protocol.CheckResult{},
		Waivers:         []any{},
		ContractChanges: []any{},
		Summary:         protocol.Summary{Pass: 1, Fail: 1, ChecksExecuted: 1, ChecksCached: 2},
	}
}

func TestWriteTextAndJSONAndFile(t *testing.T) {
	for _, st := range []string{protocol.StatusPass, protocol.StatusFail, protocol.StatusUnverified, protocol.StatusUnknown} {
		var buf bytes.Buffer
		if err := report.WriteText(&buf, sampleResult(st)); err != nil {
			t.Fatal(err)
		}
		if buf.Len() == 0 {
			t.Fatal("empty")
		}
	}
	res := sampleResult(protocol.StatusPass)
	res.Requirements = nil
	var buf bytes.Buffer
	if err := report.WriteText(&buf, res); err != nil {
		t.Fatal(err)
	}
	if err := report.ValidateResultSchema(sampleResult(protocol.StatusPass)); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "r.json")
	if err := report.Write("json", out, sampleResult(protocol.StatusPass), &buf); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
	if err := report.Write("text", "", sampleResult(protocol.StatusPass), &buf); err != nil {
		t.Fatal(err)
	}
	if err := report.Write("yaml", "", sampleResult(protocol.StatusPass), &buf); err == nil {
		t.Fatal("bad format")
	}
	if err := report.Write("json", filepath.Join(t.TempDir(), "no", "such", "r.json"), sampleResult(protocol.StatusPass), &buf); err == nil {
		t.Fatal("expected create error")
	}
	if err := report.WriteText(&errWriter{}, sampleResult(protocol.StatusPass)); err == nil {
		t.Fatal("expected write error")
	}
	bad := sampleResult(protocol.StatusPass)
	bad.Status = "not-a-status"
	_ = report.ValidateResultSchema(bad)
}

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, os.ErrClosed }

package report_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/report"
	"github.com/hypertrial/intentci/internal/verdict"
)

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("write") }

func TestWriteBranchesAndExplain(t *testing.T) {
	b := &evidence.Bundle{
		RunID:    "R",
		Unmapped: []string{"u.txt"},
		Run: verdict.RunResult{
			Verdict: verdict.Fail,
			Requirements: []verdict.RequirementResult{{
				ID: "REQ-1", Title: "t", Verdict: verdict.Fail,
				Obligations: []verdict.ObligationResult{
					{ID: "O1", Verdict: verdict.Fail, Reason: "r", Statement: "s", Evidence: []provider.Evidence{{Class: "deterministic", Summary: "sum"}}},
					{ID: "O2", Verdict: verdict.Error, Reason: "e", Statement: "s"},
					{ID: "O3", Verdict: verdict.Unproven, Reason: "u", Statement: "s"},
					{ID: "O4", Verdict: verdict.Uncertain, Reason: "c", Statement: "s"},
					{ID: "O5", Verdict: verdict.ReviewRequired, Reason: "m", Statement: "s"},
					{ID: "O6", Verdict: verdict.Pass, Statement: "s"},
				},
			}},
		},
	}
	var buf bytes.Buffer
	if err := report.Write(&buf, "", b); err != nil {
		t.Fatal(err)
	}
	if err := report.Write(&buf, "junit", b); err != nil {
		t.Fatal(err)
	}
	if err := report.Write(errWriter{}, "junit", b); err == nil {
		t.Fatal("junit write error")
	}
	if err := report.Write(&buf, "nope", b); err == nil {
		t.Fatal("unsupported")
	}
	if err := report.Explain(&buf, b, "missing", false); err == nil {
		t.Fatal("missing req")
	}
	if err := report.Explain(&buf, b, "REQ-1", true); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	if err := report.WriteGitHubStepSummary(b); err != nil {
		t.Fatal(err)
	}
	sum := filepath.Join(t.TempDir(), "sum.md")
	t.Setenv("GITHUB_STEP_SUMMARY", sum)
	if err := report.WriteGitHubStepSummary(b); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sum); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_STEP_SUMMARY", filepath.Join(t.TempDir(), "missing", "sum.md"))
	if err := report.WriteGitHubStepSummary(b); err == nil {
		t.Fatal("expected open error")
	}
}

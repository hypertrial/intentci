package report_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/report"
	"github.com/hypertrial/intentci/internal/verdict"
)

func sample() *evidence.Bundle {
	return &evidence.Bundle{
		RunID: "RUN1", CreatedAt: time.Now().UTC(),
		Run: verdict.RunResult{
			Verdict: verdict.Fail,
			Requirements: []verdict.RequirementResult{{
				ID: "REQ-1", Title: "t", Verdict: verdict.Fail,
				Obligations: []verdict.ObligationResult{{ID: "OBL-1", Verdict: verdict.Fail, Reason: "x", Statement: "s"}},
			}},
		},
	}
}

func TestFormats(t *testing.T) {
	b := sample()
	var buf bytes.Buffer
	if err := report.Write(&buf, "text", b); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := report.Write(&buf, "json", b); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := report.Write(&buf, "junit", b); err != nil {
		t.Fatal(err)
	}
	if err := report.Explain(&buf, b, "REQ-1", true); err != nil {
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
}

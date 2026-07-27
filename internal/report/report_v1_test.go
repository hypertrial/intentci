package report_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/report"
	"github.com/hypertrial/intentci/internal/verdict"
)

type failAfterXML struct {
	closed bool
}

func (writer *failAfterXML) Write(value []byte) (int, error) {
	if writer.closed {
		return 0, errors.New("trailing newline")
	}
	if bytes.Contains(value, []byte("</testsuites>")) {
		writer.closed = true
	}
	return len(value), nil
}

func explanatoryBundle() *evidence.Bundle {
	passed := true
	record := provider.Evidence{
		ID: "evidence-id", VerifierID: "verifier-id", Status: "passed",
		Class: "deterministic", Summary: "evidence summary", Passed: &passed,
	}
	return &evidence.Bundle{
		RunID: "run",
		Run: verdict.RunResult{
			Verdict: verdict.Pass,
			Requirements: []verdict.RequirementResult{{
				ID: "REQ", Title: "Requirement", Priority: "required", Verdict: verdict.Pass,
				Obligations: []verdict.ObligationResult{{
					ID: "OBL", Statement: "Statement", Required: true, Verdict: verdict.Pass,
					Evidence: []provider.Evidence{record},
				}},
			}},
		},
		ProviderLogs: map[string]provider.Result{
			"REQ/OBL/verifier-id": {Stdout: "stdout", Stderr: "stderr"},
			"REQ/OBL/fallback":    {Stdout: "fallback\n"},
			"OTHER/unused":        {Stdout: "unused"},
		},
	}
}

func TestV1JSONValidationAndWriterErrors(t *testing.T) {
	invalid := &evidence.Bundle{RunID: "run", Run: verdict.RunResult{Verdict: "invalid"}}
	if err := report.Write(&bytes.Buffer{}, "json", invalid); err == nil {
		t.Fatal("schema-invalid report written")
	}
	if err := report.Write(errWriter{}, "json", explanatoryBundle()); err == nil {
		t.Fatal("JSON writer error ignored")
	}
	if err := report.Write(&failAfterXML{}, "junit", explanatoryBundle()); err == nil {
		t.Fatal("JUnit trailing write error ignored")
	}
}

func TestV1ExplainEveryIdentifierAndLogs(t *testing.T) {
	bundle := explanatoryBundle()
	cases := []struct {
		id       string
		evidence bool
		logs     bool
		want     string
	}{
		{"run", false, true, "stdout"},
		{"REQ", true, true, "Requirement"},
		{"OBL", true, true, "evidence summary"},
		{"verifier-id", false, true, "PASSED"},
		{"evidence-id", false, false, "evidence summary"},
		{"fallback", false, false, "Verifier REQ/OBL/fallback"},
	}
	for _, testCase := range cases {
		t.Run(testCase.id, func(t *testing.T) {
			var output bytes.Buffer
			err := report.ExplainWithOptions(&output, bundle, testCase.id, report.ExplainOptions{
				ShowEvidence: testCase.evidence, ShowLogs: testCase.logs,
			})
			if err != nil || !strings.Contains(output.String(), testCase.want) {
				t.Fatalf("output=%q err=%v", output.String(), err)
			}
		})
	}
}

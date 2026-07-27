package provider

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/security"
)

// JUnitProvider runs a command and/or reads a JUnit XML report.
type JUnitProvider struct {
	Exec func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func (p *JUnitProvider) Name() string    { return "junit" }
func (p *JUnitProvider) Version() string { return "1.0.0" }

func (p *JUnitProvider) Validate(spec ir.ProviderSpec) []Diagnostic {
	if spec.Report == "" {
		return []Diagnostic{{Message: "report required"}}
	}
	return nil
}

type junitTestSuites struct {
	XMLName xml.Name         `xml:"testsuites"`
	Suites  []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	XMLName  xml.Name        `xml:"testsuite"`
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Errors   int             `xml:"errors,attr"`
	Cases    []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string     `xml:"name,attr"`
	Classname string     `xml:"classname,attr"`
	Time      float64    `xml:"time,attr"`
	Failure   *junitFail `xml:"failure"`
	Error     *junitFail `xml:"error"`
	Skipped   *junitFail `xml:"skipped"`
}

type junitFail struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

func (p *JUnitProvider) Execute(ctx context.Context, req Request) Result {
	start := time.Now()
	res := Result{Provider: p.Name(), ProviderVersion: p.Version(), Status: "completed"}
	report := req.Spec.Report
	if report != "" {
		var err error
		report, err = security.ResolveInside(req.Root, report)
		if err != nil {
			res.Status = "error"
			res.Diagnostics = []string{err.Error()}
			res.SecurityViolation = security.IsPathViolation(err)
			return res
		}
	}
	var commandErr error
	if req.Spec.Run != "" {
		if report == "" {
			res.Status = "error"
			res.Diagnostics = []string{"report path required after run"}
			res.DurationMS = time.Since(start).Milliseconds()
			return res
		}
		if err := os.Remove(report); err != nil && !os.IsNotExist(err) {
			res.Status = "error"
			res.Diagnostics = []string{"remove stale junit report: " + err.Error()}
			res.DurationMS = time.Since(start).Milliseconds()
			return res
		}
		process := runProcess(ctx, req, "sh", []string{"-c", req.Spec.Run}, nil, p.Exec)
		commandErr = process.Err
		res.SecurityViolation = process.SecurityViolation
		res.ExitCode = process.ExitCode
		if req.RetainStdout {
			res.Stdout = process.Stdout
		}
		if req.RetainStderr {
			res.Stderr = process.Stderr
		}
		if process.TimedOut {
			res.Status = "error"
			res.Diagnostics = []string{"junit command timed out"}
			res.DurationMS = time.Since(start).Milliseconds()
			res.Evidence = []Evidence{{ID: req.Spec.ID, Class: "deterministic", Summary: "timed out", Passed: boolPtr(false)}}
			return res
		}
	}
	if report == "" {
		res.Status = "error"
		res.Diagnostics = []string{"report path required after run"}
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}
	data, err := os.ReadFile(report)
	if err != nil {
		res.Status = "error"
		res.Diagnostics = []string{err.Error()}
		res.DurationMS = time.Since(start).Milliseconds()
		res.Evidence = []Evidence{{ID: req.Spec.ID, Class: "deterministic", Summary: err.Error(), Passed: boolPtr(false)}}
		return res
	}
	summaryData, parseErr := parseJUnitSummary(data)
	if parseErr != nil {
		res.Status = "error"
		res.Diagnostics = []string{parseErr.Error()}
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}
	passed := summaryData.Failures == 0 && summaryData.Errors == 0
	if passed && commandErr != nil {
		res.Status = "error"
		res.Diagnostics = []string{"junit generator failed: " + commandErr.Error()}
		res.DurationMS = time.Since(start).Milliseconds()
		res.Evidence = []Evidence{{
			ID: firstNonEmpty(req.Spec.ID, "junit"), Class: "deterministic",
			Summary: "generator failed despite passing report", Passed: boolPtr(false),
		}}
		return res
	}
	summary := fmt.Sprintf(
		"junit: %d tests, %d failures, %d errors, %d skipped",
		summaryData.Tests, summaryData.Failures, summaryData.Errors, summaryData.Skipped,
	)
	var passedValue *bool
	if summaryData.Tests == 0 {
		passedValue = nil
		summary = "junit: report contained no tests"
	} else if summaryData.Skipped == summaryData.Tests {
		passedValue = nil
		summary = "junit: all tests were skipped"
	} else {
		passedValue = boolPtr(passed)
	}
	res.Evidence = []Evidence{{
		ID: firstNonEmpty(req.Spec.ID, "junit"), Class: firstNonEmpty(req.Spec.EvidenceClass, req.EvidenceClass, "deterministic"),
		Summary: summary, Passed: passedValue,
		Data: map[string]any{
			"tests": summaryData.Tests, "failures": summaryData.Failures,
			"errors": summaryData.Errors, "skipped": summaryData.Skipped,
			"duration_seconds": summaryData.Duration, "failure_messages": summaryData.Messages,
		},
	}}
	res.DurationMS = time.Since(start).Milliseconds()
	return res
}

func parseJUnit(data []byte) (failures, total int, err error) {
	summary, err := parseJUnitSummary(data)
	return summary.Failures + summary.Errors, summary.Tests, err
}

type junitSummary struct {
	Tests    int
	Failures int
	Errors   int
	Skipped  int
	Duration float64
	Messages []string
}

func parseJUnitSummary(data []byte) (junitSummary, error) {
	var suites junitTestSuites
	if err := xml.Unmarshal(data, &suites); err == nil && (len(suites.Suites) > 0 || suites.XMLName.Local == "testsuites") {
		var summary junitSummary
		for _, s := range suites.Suites {
			addJUnitSuite(&summary, s)
		}
		return summary, nil
	}
	var suite junitTestSuite
	if err := xml.Unmarshal(data, &suite); err != nil {
		return junitSummary{}, fmt.Errorf("parse junit: %w", err)
	}
	var summary junitSummary
	addJUnitSuite(&summary, suite)
	return summary, nil
}

func addJUnitSuite(summary *junitSummary, suite junitTestSuite) {
	tests := suite.Tests
	if tests == 0 {
		tests = len(suite.Cases)
	}
	summary.Tests += tests
	summary.Failures += suite.Failures
	summary.Errors += suite.Errors
	for _, testCase := range suite.Cases {
		summary.Duration += testCase.Time
		if testCase.Skipped != nil {
			summary.Skipped++
		}
		if testCase.Failure != nil {
			if suite.Failures == 0 {
				summary.Failures++
			}
			summary.Messages = append(summary.Messages, firstNonEmpty(testCase.Failure.Message, testCase.Failure.Body))
		}
		if testCase.Error != nil {
			if suite.Errors == 0 {
				summary.Errors++
			}
			summary.Messages = append(summary.Messages, firstNonEmpty(testCase.Error.Message, testCase.Error.Body))
		}
	}
}

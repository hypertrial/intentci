package provider

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hypertrial/intentci/internal/ir"
)

// JUnitProvider runs a command and/or reads a JUnit XML report.
type JUnitProvider struct {
	Exec func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func (p *JUnitProvider) Name() string    { return "junit" }
func (p *JUnitProvider) Version() string { return "1.0.0" }

func (p *JUnitProvider) Validate(spec ir.ProviderSpec) []Diagnostic {
	if spec.Report == "" && spec.Run == "" {
		return []Diagnostic{{Message: "run or report required"}}
	}
	return nil
}

type junitTestSuites struct {
	XMLName xml.Name        `xml:"testsuites"`
	Suites  []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	XMLName  xml.Name       `xml:"testsuite"`
	Name     string         `xml:"name,attr"`
	Tests    int            `xml:"tests,attr"`
	Failures int            `xml:"failures,attr"`
	Errors   int            `xml:"errors,attr"`
	Cases    []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string      `xml:"name,attr"`
	Classname string      `xml:"classname,attr"`
	Failure   *junitFail  `xml:"failure"`
	Error     *junitFail  `xml:"error"`
}

type junitFail struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

func (p *JUnitProvider) Execute(ctx context.Context, req Request) Result {
	start := time.Now()
	res := Result{Provider: p.Name(), ProviderVersion: p.Version(), Status: "completed"}
	if req.Spec.Run != "" {
		run := p.Exec
		if run == nil {
			run = exec.CommandContext
		}
		timeout := req.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Minute
		}
		cctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cmd := run(cctx, "sh", "-c", req.Spec.Run)
		cmd.Dir = req.Root
		out, err := cmd.CombinedOutput()
		if req.RetainStdout {
			res.Stdout = string(out)
		}
		if err != nil && cctx.Err() == context.DeadlineExceeded {
			res.Status = "error"
			res.Diagnostics = []string{"junit command timed out"}
			res.DurationMS = time.Since(start).Milliseconds()
			res.Evidence = []Evidence{{ID: req.Spec.ID, Class: "deterministic", Summary: "timed out", Passed: boolPtr(false)}}
			return res
		}
	}
	report := req.Spec.Report
	if report == "" {
		res.Status = "error"
		res.Diagnostics = []string{"report path required after run"}
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}
	if !filepath.IsAbs(report) {
		report = filepath.Join(req.Root, report)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		res.Status = "error"
		res.Diagnostics = []string{err.Error()}
		res.DurationMS = time.Since(start).Milliseconds()
		res.Evidence = []Evidence{{ID: req.Spec.ID, Class: "deterministic", Summary: err.Error(), Passed: boolPtr(false)}}
		return res
	}
	failures, total, parseErr := parseJUnit(data)
	if parseErr != nil {
		res.Status = "error"
		res.Diagnostics = []string{parseErr.Error()}
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}
	passed := failures == 0
	summary := fmt.Sprintf("junit: %d tests, %d failures", total, failures)
	res.Evidence = []Evidence{{
		ID: firstNonEmpty(req.Spec.ID, "junit"), Class: "deterministic",
		Summary: summary, Passed: boolPtr(passed),
		Data: map[string]any{"tests": total, "failures": failures},
	}}
	res.DurationMS = time.Since(start).Milliseconds()
	return res
}

func parseJUnit(data []byte) (failures, total int, err error) {
	var suites junitTestSuites
	if err := xml.Unmarshal(data, &suites); err == nil && (len(suites.Suites) > 0 || suites.XMLName.Local == "testsuites") {
		for _, s := range suites.Suites {
			total += s.Tests
			failures += s.Failures + s.Errors
			if s.Tests == 0 {
				for _, c := range s.Cases {
					total++
					if c.Failure != nil || c.Error != nil {
						failures++
					}
				}
			}
		}
		return failures, total, nil
	}
	var suite junitTestSuite
	if err := xml.Unmarshal(data, &suite); err != nil {
		return 0, 0, fmt.Errorf("parse junit: %w", err)
	}
	total = suite.Tests
	failures = suite.Failures + suite.Errors
	if total == 0 {
		for _, c := range suite.Cases {
			total++
			if c.Failure != nil || c.Error != nil {
				failures++
			}
		}
	}
	return failures, total, nil
}

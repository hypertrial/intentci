package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/verdict"
)

// Write renders a bundle in the requested format.
func Write(w io.Writer, format string, b *evidence.Bundle) error {
	switch strings.ToLower(format) {
	case "", "text":
		return writeText(w, b)
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(b)
	case "junit":
		return writeJUnit(w, b)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func writeText(w io.Writer, b *evidence.Bundle) error {
	fmt.Fprintf(w, "Run %s  verdict=%s\n", b.RunID, b.Run.Verdict)
	if len(b.Unmapped) > 0 {
		fmt.Fprintf(w, "Unmapped files: %s\n", strings.Join(b.Unmapped, ", "))
	}
	for _, r := range b.Run.Requirements {
		fmt.Fprintf(w, "\n%s %s  %s\n", strings.ToUpper(r.Verdict), r.ID, r.Title)
		for _, o := range r.Obligations {
			fmt.Fprintf(w, "  %s %s  %s\n", strings.ToUpper(o.Verdict), o.ID, o.Statement)
			if o.Reason != "" {
				fmt.Fprintf(w, "    %s\n", o.Reason)
			}
		}
	}
	return nil
}

type junitSuites struct {
	XMLName xml.Name    `xml:"testsuites"`
	Suites  []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Errors   int         `xml:"errors,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	Error     *junitFailure `xml:"error,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

func writeJUnit(w io.Writer, b *evidence.Bundle) error {
	suite := junitSuite{Name: "intentci"}
	for _, r := range b.Run.Requirements {
		for _, o := range r.Obligations {
			suite.Tests++
			tc := junitCase{Name: o.ID, Classname: r.ID}
			switch o.Verdict {
			case verdict.Fail:
				suite.Failures++
				tc.Failure = &junitFailure{Message: o.Verdict, Body: o.Reason}
			case verdict.Error:
				suite.Errors++
				tc.Error = &junitFailure{Message: o.Verdict, Body: o.Reason}
			case verdict.Unproven, verdict.Uncertain, verdict.ReviewRequired:
				suite.Failures++
				tc.Failure = &junitFailure{Message: o.Verdict, Body: o.Reason}
			}
			suite.Cases = append(suite.Cases, tc)
		}
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(junitSuites{Suites: []junitSuite{suite}}); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// WriteGitHubStepSummary appends a markdown summary when GITHUB_STEP_SUMMARY is set.
func WriteGitHubStepSummary(b *evidence.Bundle) error {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintf(f, "## IntentCI %s\n\nVerdict: `%s`\n\n", b.RunID, b.Run.Verdict)
	fmt.Fprintf(f, "| Requirement | Verdict |\n| --- | --- |\n")
	for _, r := range b.Run.Requirements {
		fmt.Fprintf(f, "| %s | %s |\n", r.ID, r.Verdict)
	}
	return nil
}

// Explain writes a detailed explanation for a requirement.
func Explain(w io.Writer, b *evidence.Bundle, id string, showEvidence bool) error {
	for _, r := range b.Run.Requirements {
		if r.ID != id {
			continue
		}
		fmt.Fprintf(w, "%s: %s\nVerdict: %s\n\n", r.ID, r.Title, strings.ToUpper(r.Verdict))
		for _, o := range r.Obligations {
			fmt.Fprintf(w, "%s  %s\n  %s\n", strings.ToUpper(o.Verdict), o.ID, o.Statement)
			if o.Reason != "" {
				fmt.Fprintf(w, "  Evidence: %s\n", o.Reason)
			}
			if showEvidence {
				for _, e := range o.Evidence {
					fmt.Fprintf(w, "  - [%s] %s\n", e.Class, e.Summary)
				}
			}
			fmt.Fprintln(w)
		}
		return nil
	}
	return fmt.Errorf("requirement %q not found in run %s", id, b.RunID)
}

package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/verdict"
	appschema "github.com/hypertrial/intentci/pkg/schema"
)

// Write renders a bundle in the requested format.
func Write(w io.Writer, format string, b *evidence.Bundle) error {
	switch strings.ToLower(format) {
	case "", "text":
		return writeText(w, b)
	case "json":
		if err := appschema.Validate("report", b); err != nil {
			return err
		}
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
	var output strings.Builder
	fmt.Fprintf(&output, "Run %s  verdict=%s\n", b.RunID, b.Run.Verdict)
	if len(b.Unmapped) > 0 {
		fmt.Fprintf(&output, "Unmapped files: %s\n", strings.Join(b.Unmapped, ", "))
	}
	for _, r := range b.Run.Requirements {
		fmt.Fprintf(&output, "\n%s %s  %s\n", strings.ToUpper(r.Verdict), r.ID, r.Title)
		for _, o := range r.Obligations {
			fmt.Fprintf(&output, "  %s %s  %s\n", strings.ToUpper(o.Verdict), o.ID, o.Statement)
			if o.Reason != "" {
				fmt.Fprintf(&output, "    %s\n", o.Reason)
			}
		}
	}
	_, err := io.WriteString(w, output.String())
	return err
}

type junitSuites struct {
	XMLName xml.Name     `xml:"testsuites"`
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
	suites := make([]junitSuite, 0, len(b.Run.Requirements))
	for _, r := range b.Run.Requirements {
		suite := junitSuite{Name: r.ID}
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
		suites = append(suites, suite)
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(junitSuites{Suites: suites}); err != nil {
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
	return ExplainWithOptions(w, b, id, ExplainOptions{ShowEvidence: showEvidence})
}

// ExplainOptions controls optional evidence and log detail.
type ExplainOptions struct {
	ShowEvidence bool
	ShowLogs     bool
}

// ExplainWithOptions explains a run, requirement, obligation, or verifier.
func ExplainWithOptions(w io.Writer, b *evidence.Bundle, id string, options ExplainOptions) error {
	if id == b.RunID {
		_ = writeText(w, b)
		if options.ShowLogs {
			writeLogs(w, b, "")
		}
		return nil
	}
	for _, r := range b.Run.Requirements {
		if r.ID == id {
			fmt.Fprintf(w, "%s: %s\nVerdict: %s\n\n", r.ID, r.Title, strings.ToUpper(r.Verdict))
			for _, o := range r.Obligations {
				writeObligation(w, o, options.ShowEvidence)
			}
			if options.ShowLogs {
				writeLogs(w, b, r.ID+"/")
			}
			return nil
		}
		for _, o := range r.Obligations {
			if o.ID == id {
				fmt.Fprintf(w, "%s / %s\n", r.ID, r.Title)
				writeObligation(w, o, options.ShowEvidence)
				if options.ShowLogs {
					writeLogs(w, b, r.ID+"/"+o.ID+"/")
				}
				return nil
			}
			for _, record := range o.Evidence {
				if record.VerifierID == id || record.ID == id {
					fmt.Fprintf(w, "%s / %s\n", r.ID, o.ID)
					fmt.Fprintf(w, "%s [%s] %s\n", strings.ToUpper(record.Status), record.Class, record.Summary)
					if options.ShowLogs {
						writeLogs(w, b, r.ID+"/"+o.ID+"/"+record.VerifierID)
					}
					return nil
				}
			}
		}
	}
	for key := range b.ProviderLogs {
		if strings.HasSuffix(key, "/"+id) {
			fmt.Fprintf(w, "Verifier %s\n", key)
			writeLogs(w, b, key)
			return nil
		}
	}
	return fmt.Errorf("identifier %q not found in run %s", id, b.RunID)
}

func writeObligation(w io.Writer, obligation verdict.ObligationResult, showEvidence bool) {
	fmt.Fprintf(w, "%s  %s\n  %s\n", strings.ToUpper(obligation.Verdict), obligation.ID, obligation.Statement)
	if obligation.Reason != "" {
		fmt.Fprintf(w, "  Evidence: %s\n", obligation.Reason)
	}
	if showEvidence {
		for _, record := range obligation.Evidence {
			fmt.Fprintf(w, "  - [%s] %s\n", record.Class, record.Summary)
		}
	}
	fmt.Fprintln(w)
}

func writeLogs(w io.Writer, bundle *evidence.Bundle, prefix string) {
	keys := make([]string, 0, len(bundle.ProviderLogs))
	for key := range bundle.ProviderLogs {
		if prefix == "" || strings.HasPrefix(key, prefix) || key == prefix {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		result := bundle.ProviderLogs[key]
		if result.Stdout != "" {
			fmt.Fprintf(w, "\n%s stdout:\n%s", key, result.Stdout)
			if !strings.HasSuffix(result.Stdout, "\n") {
				fmt.Fprintln(w)
			}
		}
		if result.Stderr != "" {
			fmt.Fprintf(w, "\n%s stderr:\n%s", key, result.Stderr)
			if !strings.HasSuffix(result.Stderr, "\n") {
				fmt.Fprintln(w)
			}
		}
	}
}

package junit

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
)

// Suite is a minimal JUnit testsuite.
type Suite struct {
	XMLName  xml.Name `xml:"testsuite"`
	Name     string   `xml:"name,attr"`
	Tests    int      `xml:"tests,attr"`
	Failures int      `xml:"failures,attr"`
	Errors   int      `xml:"errors,attr"`
	Cases    []Case   `xml:"testcase"`
}

// Case is a minimal JUnit testcase.
type Case struct {
	Name      string   `xml:"name,attr"`
	ClassName string   `xml:"classname,attr"`
	Failure   *Failure `xml:"failure"`
	Error     *Failure `xml:"error"`
}

// Failure captures failure/error text.
type Failure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

// Result is the parsed outcome for IntentCI.
type Result struct {
	OK       bool
	Failures []string
}

// ParseFile reads and parses a JUnit XML file.
func ParseFile(path string) (Result, error) {
	data, err := readFile(path)
	if err != nil {
		return Result{}, err
	}
	return Parse(data)
}

// Parse parses JUnit XML bytes.
func Parse(data []byte) (Result, error) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return Result{}, fmt.Errorf("empty junit xml")
	}
	// Support both <testsuites> wrapper and bare <testsuite>.
	type Suites struct {
		XMLName xml.Name `xml:"testsuites"`
		Suites  []Suite  `xml:"testsuite"`
	}
	var suites Suites
	if err := xml.Unmarshal(data, &suites); err == nil && (suites.XMLName.Local == "testsuites" || len(suites.Suites) > 0) {
		return fromSuites(suites.Suites), nil
	}
	var suite Suite
	if err := xml.Unmarshal(data, &suite); err != nil {
		return Result{}, fmt.Errorf("parse junit xml: %w", err)
	}
	return fromSuites([]Suite{suite}), nil
}

func fromSuites(suites []Suite) Result {
	var failures []string
	for _, s := range suites {
		if s.Failures > 0 || s.Errors > 0 {
			// Prefer detailed case messages when present.
		}
		for _, c := range s.Cases {
			name := c.Name
			if c.ClassName != "" {
				name = c.ClassName + "." + c.Name
			}
			if c.Failure != nil {
				msg := c.Failure.Message
				if msg == "" {
					msg = strings.TrimSpace(c.Failure.Body)
				}
				if msg == "" {
					msg = "failure"
				}
				failures = append(failures, fmt.Sprintf("%s: %s", name, msg))
			}
			if c.Error != nil {
				msg := c.Error.Message
				if msg == "" {
					msg = strings.TrimSpace(c.Error.Body)
				}
				if msg == "" {
					msg = "error"
				}
				failures = append(failures, fmt.Sprintf("%s: %s", name, msg))
			}
		}
		if len(failures) == 0 && (s.Failures > 0 || s.Errors > 0) {
			failures = append(failures, fmt.Sprintf("suite %s reported failures=%d errors=%d", s.Name, s.Failures, s.Errors))
		}
	}
	return Result{OK: len(failures) == 0, Failures: failures}
}

var readFile = os.ReadFile

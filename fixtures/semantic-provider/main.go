// Command semantic-provider is a deterministic IntentCI semantic provider for tests.
package main

import (
	"encoding/json"
	"os"
	"strings"
)

type request struct {
	Requirements []struct {
		ID       string `json:"id"`
		Statement string `json:"statement"`
	} `json:"requirements"`
	Diff string `json:"diff"`
}

type finding struct {
	RequirementID   string `json:"requirement_id"`
	Assessment      string `json:"assessment"`
	Confidence      float64 `json:"confidence"`
	Summary         string `json:"summary"`
	Evidence        []struct {
		Path      string `json:"path"`
		LineStart int    `json:"line_start"`
		LineEnd   int    `json:"line_end"`
	} `json:"evidence,omitempty"`
	MissingEvidence []string `json:"missing_evidence,omitempty"`
}

type response struct {
	ProtocolVersion int       `json:"protocol_version"`
	Findings        []finding `json:"findings"`
}

func main() {
	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		os.Exit(1)
	}
	mode := os.Getenv("INTENTCI_SEMANTIC_FIXTURE")
	if mode == "" {
		mode = "aligned"
	}
	resp := response{ProtocolVersion: 1, Findings: []finding{}}
	for _, r := range req.Requirements {
		f := finding{
			RequirementID: r.ID,
			Confidence:    0.95,
			Summary:       "fixture provider: " + mode,
		}
		switch mode {
		case "contradiction":
			f.Assessment = "contradiction"
			f.Evidence = []struct {
				Path      string `json:"path"`
				LineStart int    `json:"line_start"`
				LineEnd   int    `json:"line_end"`
			}{{Path: "pkg/counter/counter.go", LineStart: 1, LineEnd: 3}}
		case "insufficient":
			f.Assessment = "insufficient_evidence"
			f.MissingEvidence = []string{"A regression test for the affected behavior"}
		case "uncertain":
			f.Assessment = "uncertain"
		case "fail":
			os.Exit(2)
		default:
			f.Assessment = "aligned"
		}
		if strings.Contains(req.Diff, "FORCE_CONTRADICTION") {
			f.Assessment = "contradiction"
			f.Evidence = []struct {
				Path      string `json:"path"`
				LineStart int    `json:"line_start"`
				LineEnd   int    `json:"line_end"`
			}{{Path: "forced.go", LineStart: 1, LineEnd: 1}}
		}
		resp.Findings = append(resp.Findings, f)
	}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}

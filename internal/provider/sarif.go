package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/security"
)

// SARIFProvider runs a command and/or reads a SARIF report.
type SARIFProvider struct {
	Exec func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func (p *SARIFProvider) Name() string    { return "sarif" }
func (p *SARIFProvider) Version() string { return "1.0.0" }

func (p *SARIFProvider) Validate(spec ir.ProviderSpec) []Diagnostic {
	if spec.Report == "" {
		return []Diagnostic{{Message: "report required"}}
	}
	for key := range spec.Match {
		if key != "rule_id" && key != "severity" && key != "path" &&
			key != "result_level" && key != "baseline_state" {
			return []Diagnostic{{Message: fmt.Sprintf("unsupported SARIF match field %q", key)}}
		}
	}
	for key, value := range spec.Allow {
		switch key {
		case "max_findings":
			if parsed, ok := integer(value); !ok || parsed < 0 {
				return []Diagnostic{{Message: "sarif allow.max_findings must be a non-negative integer"}}
			}
		case "levels":
			if len(stringValues(value)) == 0 {
				return []Diagnostic{{Message: "sarif allow.levels must be a non-empty string list"}}
			}
		default:
			return []Diagnostic{{Message: fmt.Sprintf("unsupported SARIF allow field %q", key)}}
		}
	}
	return nil
}

func (p *SARIFProvider) Execute(ctx context.Context, req Request) Result {
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
			res.Diagnostics = []string{"remove stale sarif report: " + err.Error()}
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
			res.Diagnostics = []string{"sarif command timed out"}
			res.DurationMS = time.Since(start).Milliseconds()
			res.Evidence = []Evidence{{ID: req.Spec.ID, Class: "deterministic", Summary: "timed out", Passed: boolPtr(false)}}
			return res
		}
	}
	if report == "" {
		res.Status = "error"
		res.Diagnostics = []string{"report path required"}
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
	findings, maximum, err := evaluateSARIF(data, req.Spec)
	if err != nil {
		res.Status = "error"
		res.Diagnostics = []string{err.Error()}
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}
	passed := findings <= maximum
	if passed && commandErr != nil {
		res.Status = "error"
		res.Diagnostics = []string{"sarif generator failed: " + commandErr.Error()}
		res.DurationMS = time.Since(start).Milliseconds()
		res.Evidence = []Evidence{{
			ID: firstNonEmpty(req.Spec.ID, "sarif"), Class: "deterministic",
			Summary: "generator failed despite passing report", Passed: boolPtr(false),
		}}
		return res
	}
	res.Evidence = []Evidence{{
		ID: firstNonEmpty(req.Spec.ID, "sarif"), Class: firstNonEmpty(req.Spec.EvidenceClass, req.EvidenceClass, "deterministic"),
		Summary: fmt.Sprintf("sarif: %d findings (max %d)", findings, maximum), Passed: boolPtr(passed),
		Data: map[string]any{"findings": findings, "max_findings": maximum},
	}}
	res.DurationMS = time.Since(start).Milliseconds()
	return res
}

func countSARIF(data []byte) (int, error) {
	var doc struct {
		Runs []struct {
			Results []json.RawMessage `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return 0, fmt.Errorf("parse sarif: %w", err)
	}
	n := 0
	for _, r := range doc.Runs {
		n += len(r.Results)
	}
	return n, nil
}

type sarifDocument struct {
	Version string `json:"version"`
	Runs    []struct {
		Results []sarifResult `json:"results"`
	} `json:"runs"`
}

type sarifResult struct {
	RuleID        string         `json:"ruleId"`
	Level         string         `json:"level"`
	BaselineState string         `json:"baselineState"`
	Properties    map[string]any `json:"properties"`
	Locations     []struct {
		PhysicalLocation struct {
			ArtifactLocation struct {
				URI string `json:"uri"`
			} `json:"artifactLocation"`
		} `json:"physicalLocation"`
	} `json:"locations"`
}

func evaluateSARIF(data []byte, spec ir.ProviderSpec) (int, int, error) {
	var document sarifDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return 0, 0, fmt.Errorf("parse sarif: %w", err)
	}
	if document.Version != "2.1.0" || document.Runs == nil {
		return 0, 0, fmt.Errorf("parse sarif: version 2.1.0 and runs are required")
	}
	maximum := 0
	if raw, ok := spec.Allow["max_findings"]; ok {
		if parsed, ok := integer(raw); ok {
			maximum = parsed
		} else {
			return 0, 0, fmt.Errorf("sarif allow.max_findings must be an integer")
		}
	}
	levels := stringValues(spec.Allow["levels"])
	findings := 0
	for _, run := range document.Runs {
		for _, result := range run.Results {
			if len(levels) > 0 && !containsString(levels, result.Level) {
				continue
			}
			if !matchesSARIF(result, spec.Match) {
				continue
			}
			findings++
		}
	}
	return findings, maximum, nil
}

func matchesSARIF(result sarifResult, match map[string]any) bool {
	if match == nil {
		return true
	}
	if want, ok := match["rule_id"]; ok && result.RuleID != fmt.Sprint(want) {
		return false
	}
	if want, ok := match["result_level"]; ok && result.Level != fmt.Sprint(want) {
		return false
	}
	if want, ok := match["baseline_state"]; ok && result.BaselineState != fmt.Sprint(want) {
		return false
	}
	if want, ok := match["severity"]; ok && fmt.Sprint(result.Properties["security-severity"]) != fmt.Sprint(want) {
		return false
	}
	if want, ok := match["path"]; ok {
		found := false
		for _, location := range result.Locations {
			if matched, _ := doublestar.Match(fmt.Sprint(want), location.PhysicalLocation.ArtifactLocation.URI); matched {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func integer(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case float64:
		return int(typed), typed == float64(int(typed))
	default:
		return 0, false
	}
}

func stringValues(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]string); ok {
			return typed
		}
		return nil
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		values = append(values, fmt.Sprint(item))
	}
	return values
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

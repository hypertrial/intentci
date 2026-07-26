package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hypertrial/intentci/internal/ir"
)

// JSONProvider reads a JSON file and checks assert equality on a key path.
type JSONProvider struct{}

func (p *JSONProvider) Name() string    { return "json" }
func (p *JSONProvider) Version() string { return "1.0.0" }

func (p *JSONProvider) Validate(spec ir.ProviderSpec) []Diagnostic {
	if spec.Report == "" {
		return []Diagnostic{{Message: "report required"}}
	}
	return nil
}

func (p *JSONProvider) Execute(ctx context.Context, req Request) Result {
	_ = ctx
	start := time.Now()
	report := req.Spec.Report
	if !filepath.IsAbs(report) {
		report = filepath.Join(req.Root, report)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		return Result{
			Provider: p.Name(), ProviderVersion: p.Version(), Status: "error",
			DurationMS: time.Since(start).Milliseconds(),
			Diagnostics: []string{err.Error()},
			Evidence: []Evidence{{ID: req.Spec.ID, Class: "deterministic", Summary: err.Error(), Passed: boolPtr(false)}},
		}
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return Result{
			Provider: p.Name(), ProviderVersion: p.Version(), Status: "error",
			DurationMS: time.Since(start).Milliseconds(),
			Diagnostics: []string{err.Error()},
		}
	}
	passed := true
	summary := "json assert passed"
	if req.Spec.Assert != nil {
		for k, want := range req.Spec.Assert {
			got := lookup(doc, k)
			if fmt.Sprint(got) != fmt.Sprint(want) {
				passed = false
				summary = fmt.Sprintf("assert %s: got %v want %v", k, got, want)
				break
			}
		}
	}
	return Result{
		Provider: p.Name(), ProviderVersion: p.Version(), Status: "completed",
		DurationMS: time.Since(start).Milliseconds(),
		Evidence: []Evidence{{
			ID: firstNonEmpty(req.Spec.ID, "json"), Class: "deterministic",
			Summary: summary, Passed: boolPtr(passed),
		}},
	}
}

func lookup(doc any, key string) any {
	m, ok := doc.(map[string]any)
	if !ok {
		return nil
	}
	return m[key]
}

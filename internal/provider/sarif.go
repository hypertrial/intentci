package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hypertrial/intentci/internal/ir"
)

// SARIFProvider runs a command and/or reads a SARIF report.
type SARIFProvider struct {
	Exec func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func (p *SARIFProvider) Name() string    { return "sarif" }
func (p *SARIFProvider) Version() string { return "1.0.0" }

func (p *SARIFProvider) Validate(spec ir.ProviderSpec) []Diagnostic {
	if spec.Report == "" && spec.Run == "" {
		return []Diagnostic{{Message: "run or report required"}}
	}
	return nil
}

func (p *SARIFProvider) Execute(ctx context.Context, req Request) Result {
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
		out, _ := cmd.CombinedOutput()
		if req.RetainStdout {
			res.Stdout = string(out)
		}
	}
	report := req.Spec.Report
	if report == "" {
		res.Status = "error"
		res.Diagnostics = []string{"report path required"}
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
	findings, err := countSARIF(data)
	if err != nil {
		res.Status = "error"
		res.Diagnostics = []string{err.Error()}
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}
	passed := findings == 0
	res.Evidence = []Evidence{{
		ID: firstNonEmpty(req.Spec.ID, "sarif"), Class: "deterministic",
		Summary: fmt.Sprintf("sarif: %d findings", findings), Passed: boolPtr(passed),
		Data: map[string]any{"findings": findings},
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

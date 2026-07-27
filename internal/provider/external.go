package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hypertrial/intentci/internal/ir"
)

const externalProtocolVersion = "1.0"

// ExternalProvider invokes intentci-provider-NAME using the v1 subprocess protocol.
type ExternalProvider struct {
	ProviderName string
	Path         string
}

func (p *ExternalProvider) Name() string    { return p.ProviderName }
func (p *ExternalProvider) Version() string { return "external" }
func (p *ExternalProvider) Validate(ir.ProviderSpec) []Diagnostic {
	return nil
}

func (p *ExternalProvider) Execute(ctx context.Context, req Request) Result {
	request := map[string]any{
		"protocol_version": externalProtocolVersion,
		"run_id":           req.RunID, "attempt_id": req.AttemptID,
		"requirement_id": req.RequirementID, "obligation_id": req.ObligationID,
		"repository": map[string]any{
			"root": req.Root, "commit": req.HeadCommit, "base_commit": req.BaseCommit,
			"diff_hash": req.DiffHash, "changed_files": req.ChangedFiles,
		},
		"verifier":      req.Spec,
		"configuration": req.Spec.Configuration,
		"timeout_ms":    req.Timeout.Milliseconds(),
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return externalError(p, err, "")
	}
	process := runProcess(ctx, req, p.Path, nil, bytes.NewReader(raw), nil)
	if process.SecurityViolation {
		result := externalError(p, process.Err, process.Stderr)
		result.SecurityViolation = true
		return result
	}
	if process.TimedOut {
		return externalError(p, fmt.Errorf("external provider timed out"), process.Stderr)
	}
	if process.Err != nil {
		return externalError(p, fmt.Errorf("external provider: %w", process.Err), process.Stderr)
	}
	var response struct {
		ProtocolVersion string         `json:"protocol_version"`
		Provider        string         `json:"provider"`
		ProviderVersion string         `json:"provider_version"`
		Status          string         `json:"status"`
		Evidence        []Evidence     `json:"evidence"`
		Diagnostics     []string       `json:"diagnostics"`
		Extra           map[string]any `json:"extra"`
	}
	if err := json.Unmarshal([]byte(process.Stdout), &response); err != nil {
		return externalError(p, fmt.Errorf("parse external provider response: %w", err), process.Stderr)
	}
	if major(response.ProtocolVersion) != major(externalProtocolVersion) {
		return externalError(p, fmt.Errorf("incompatible external provider protocol %q", response.ProtocolVersion), process.Stderr)
	}
	if response.ProviderVersion == "" {
		return externalError(p, fmt.Errorf("external provider omitted provider_version"), process.Stderr)
	}
	if response.Status != "completed" && response.Status != "error" && response.Status != "skipped" {
		return externalError(p, fmt.Errorf("external provider returned invalid status %q", response.Status), process.Stderr)
	}
	result := Result{
		Provider: p.Name(), ProviderVersion: response.ProviderVersion,
		Status: response.Status, Evidence: response.Evidence,
		Diagnostics: response.Diagnostics, DurationMS: process.EndedAt.Sub(process.StartedAt).Milliseconds(),
		ExitCode: process.ExitCode, Extra: response.Extra,
	}
	if req.RetainStdout {
		result.Stdout = process.Stdout
	}
	if req.RetainStderr {
		result.Stderr = process.Stderr
	}
	if strings.TrimSpace(process.Stderr) != "" {
		result.Diagnostics = append(result.Diagnostics, strings.TrimSpace(process.Stderr))
	}
	return result
}

func externalError(p *ExternalProvider, err error, stderr string) Result {
	diagnostics := []string{err.Error()}
	if strings.TrimSpace(stderr) != "" {
		diagnostics = append(diagnostics, strings.TrimSpace(stderr))
	}
	return Result{
		Provider: p.Name(), ProviderVersion: p.Version(), Status: "error",
		Diagnostics: diagnostics,
		Evidence: []Evidence{{
			ID: p.Name(), Class: "deterministic", Summary: err.Error(), Passed: boolPtr(false),
		}},
	}
}

func major(version string) string {
	return strings.SplitN(version, ".", 2)[0]
}

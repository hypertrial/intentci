package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/hypertrial/intentci/internal/ir"
)

// GitDiffProvider asserts that specific paths did or did not change.
type GitDiffProvider struct{}

func (p *GitDiffProvider) Name() string    { return "git-diff" }
func (p *GitDiffProvider) Version() string { return "1.0.0" }

func (p *GitDiffProvider) Validate(spec ir.ProviderSpec) []Diagnostic {
	if len(spec.Paths) == 0 && len(spec.Forbidden) == 0 {
		return []Diagnostic{{Message: "paths or forbidden required"}}
	}
	return nil
}

func (p *GitDiffProvider) Execute(ctx context.Context, req Request) Result {
	_ = ctx
	start := time.Now()
	patterns := append([]string{}, specPaths(req.Spec)...)
	var hits []string
	for _, f := range req.ChangedFiles {
		for _, pat := range patterns {
			ok, err := doublestar.Match(pat, f)
			if err == nil && ok {
				hits = append(hits, f)
				break
			}
		}
	}
	// default: forbidden paths must not change
	expectUnchanged := true
	if req.Spec.Expect != nil {
		if v, ok := req.Spec.Expect["changed"].(bool); ok {
			expectUnchanged = !v
		}
	}
	passed := true
	summary := "git-diff check passed"
	if expectUnchanged {
		passed = len(hits) == 0
		if !passed {
			summary = fmt.Sprintf("unexpected changes: %v", hits)
		}
	} else {
		passed = len(hits) > 0
		if !passed {
			summary = "expected changes matching paths, found none"
		}
	}
	return Result{
		Provider:        p.Name(),
		ProviderVersion: p.Version(),
		Status:          "completed",
		DurationMS:      time.Since(start).Milliseconds(),
		Evidence: []Evidence{{
			ID: firstNonEmpty(req.Spec.ID, "git-diff"), Class: "deterministic",
			Summary: summary, Paths: hits, Passed: boolPtr(passed),
		}},
	}
}

func specPaths(spec ir.ProviderSpec) []string {
	out := append([]string{}, spec.Paths...)
	out = append(out, spec.Forbidden...)
	return out
}

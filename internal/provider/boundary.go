package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/hypertrial/intentci/internal/ir"
)

// BoundaryProvider checks changed files against allowed/forbidden globs.
type BoundaryProvider struct{}

func (p *BoundaryProvider) Name() string    { return "boundary" }
func (p *BoundaryProvider) Version() string { return "1.0.0" }

func (p *BoundaryProvider) Validate(spec ir.ProviderSpec) []Diagnostic {
	if len(spec.Forbidden) == 0 && len(spec.Allowed) == 0 {
		return []Diagnostic{{Message: "allowed or forbidden required"}}
	}
	return nil
}

func (p *BoundaryProvider) Execute(ctx context.Context, req Request) Result {
	_ = ctx
	start := time.Now()
	var violations []string
	for _, f := range req.ChangedFiles {
		for _, pat := range req.Spec.Forbidden {
			ok, err := doublestar.Match(pat, f)
			if err == nil && ok {
				violations = append(violations, f)
				break
			}
		}
	}
	if len(req.Spec.Allowed) > 0 {
		for _, f := range req.ChangedFiles {
			allowed := false
			for _, pat := range req.Spec.Allowed {
				ok, err := doublestar.Match(pat, f)
				if err == nil && ok {
					allowed = true
					break
				}
			}
			if !allowed {
				// only flag if also not already in violations for forbidden-only semantics;
				// allowed means changes outside allowed are violations
				violations = append(violations, f)
			}
		}
	}
	// dedupe
	violations = unique(violations)
	passed := len(violations) == 0
	summary := "no boundary violations"
	if !passed {
		summary = fmt.Sprintf("boundary violations: %v", violations)
	}
	return Result{
		Provider:        p.Name(),
		ProviderVersion: p.Version(),
		Status:          "completed",
		DurationMS:      time.Since(start).Milliseconds(),
		Evidence: []Evidence{{
			ID:      firstNonEmpty(req.Spec.ID, "boundary"),
			Class:   "deterministic",
			Summary: summary,
			Paths:   violations,
			Passed:  boolPtr(passed),
		}},
	}
}

func unique(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

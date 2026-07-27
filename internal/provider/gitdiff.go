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
	var matchedChanges []Change
	for _, f := range req.ChangedFiles {
		for _, pat := range patterns {
			ok, err := doublestar.Match(pat, f)
			if err == nil && ok {
				hits = append(hits, f)
				matchedChanges = append(matchedChanges, changeForPath(req.Changes, f))
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
	if passed {
		var reason string
		passed, reason = evaluateChangeExpectations(req.Spec.Expect, matchedChanges)
		if !passed {
			summary = reason
		}
	}
	return Result{
		Provider:        p.Name(),
		ProviderVersion: p.Version(),
		Status:          "completed",
		DurationMS:      time.Since(start).Milliseconds(),
		Evidence: []Evidence{{
			ID:      firstNonEmpty(req.Spec.ID, "git-diff"),
			Class:   firstNonEmpty(req.Spec.EvidenceClass, req.EvidenceClass, "deterministic"),
			Summary: summary, Paths: hits, Passed: boolPtr(passed),
			Data: map[string]any{"changes": matchedChanges},
		}},
	}
}

func changeForPath(changes []Change, path string) Change {
	for _, change := range changes {
		if change.Path == path {
			return change
		}
	}
	return Change{Path: path, Status: "modified"}
}

func evaluateChangeExpectations(expect map[string]any, changes []Change) (bool, string) {
	if expect == nil {
		return true, ""
	}
	statuses := stringValues(expect["status"])
	if single, ok := expect["status"].(string); ok {
		statuses = []string{single}
	}
	additions, deletions := 0, 0
	for _, change := range changes {
		additions += change.Additions
		deletions += change.Deletions
		if len(statuses) > 0 && !containsString(statuses, change.Status) {
			return false, fmt.Sprintf("change %s has status %s", change.Path, change.Status)
		}
	}
	for key, status := range map[string]string{
		"renamed": "renamed", "deleted": "deleted", "binary": "binary",
	} {
		expected, ok := expect[key].(bool)
		if !ok {
			continue
		}
		found := false
		for _, change := range changes {
			found = found || (status == "binary" && change.Binary) || change.Status == status
		}
		if found != expected {
			return false, fmt.Sprintf("expected %s=%t", key, expected)
		}
	}
	for key, got := range map[string]int{"max_additions": additions, "max_deletions": deletions} {
		if maximum, ok := integer(expect[key]); ok && got > maximum {
			return false, fmt.Sprintf("%s exceeded: %d > %d", key, got, maximum)
		}
	}
	return true, ""
}

func specPaths(spec ir.ProviderSpec) []string {
	out := append([]string{}, spec.Paths...)
	out = append(out, spec.Forbidden...)
	return out
}

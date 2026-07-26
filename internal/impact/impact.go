package impact

import (
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/hypertrial/intentci/internal/ir"
)

// Selection is the set of requirements/obligations to verify.
type Selection struct {
	Requirements []ir.Requirement
	Unmapped     []string
}

// Options configures impact analysis.
type Options struct {
	All           bool
	RequirementID string
	ObligationID  string
	ChangedFiles  []string
}

// Select chooses active requirements affected by changed files.
func Select(doc *ir.Document, opt Options) Selection {
	active := doc.ActiveRequirements()
	if opt.RequirementID != "" {
		var filtered []ir.Requirement
		for _, r := range active {
			if r.ID == opt.RequirementID {
				if opt.ObligationID != "" {
					r = filterObligation(r, opt.ObligationID)
				}
				filtered = append(filtered, r)
			}
		}
		return Selection{Requirements: filtered}
	}
	if opt.All {
		out := active
		if opt.ObligationID != "" {
			tmp := make([]ir.Requirement, 0, len(out))
			for _, r := range out {
				tmp = append(tmp, filterObligation(r, opt.ObligationID))
			}
			out = tmp
		}
		return Selection{Requirements: out}
	}
	// Changed-mode with no diff: nothing affected (do not silently verify all).
	if len(opt.ChangedFiles) == 0 {
		return Selection{Requirements: nil, Unmapped: nil}
	}

	// dependency closure of path-matched requirements
	matched := map[string]bool{}
	for _, r := range active {
		if matchesPaths(r, opt.ChangedFiles) {
			matched[r.ID] = true
		}
	}
	// propagate depends_on reverse: if A depends on B and B matched, A is affected;
	// also if A matched, dependencies of A should run.
	byID := map[string]ir.Requirement{}
	for _, r := range active {
		byID[r.ID] = r
	}
	changed := true
	for changed {
		changed = false
		for _, r := range active {
			if matched[r.ID] {
				for _, dep := range r.DependsOn {
					if !matched[dep] {
						if _, ok := byID[dep]; ok {
							matched[dep] = true
							changed = true
						}
					}
				}
				continue
			}
			for _, dep := range r.DependsOn {
				if matched[dep] {
					matched[r.ID] = true
					changed = true
					break
				}
			}
		}
	}

	var selected []ir.Requirement
	for _, r := range active {
		if matched[r.ID] {
			if opt.ObligationID != "" {
				r = filterObligation(r, opt.ObligationID)
			}
			selected = append(selected, r)
		}
	}

	var unmapped []string
	for _, f := range opt.ChangedFiles {
		hit := false
		for _, r := range active {
			if pathMatches(r.AppliesTo.Paths, f) || pathMatches(r.Boundaries.Allowed, f) {
				hit = true
				break
			}
		}
		if !hit {
			unmapped = append(unmapped, f)
		}
	}
	return Selection{Requirements: selected, Unmapped: unmapped}
}

func filterObligation(r ir.Requirement, id string) ir.Requirement {
	var obs []ir.Obligation
	for _, o := range r.Obligations {
		if o.ID == id {
			obs = append(obs, o)
		}
	}
	r.Obligations = obs
	return r
}

func matchesPaths(r ir.Requirement, files []string) bool {
	paths := r.AppliesTo.Paths
	if len(paths) == 0 {
		// no applies_to → affected by any change (conservative)
		return len(files) > 0
	}
	for _, f := range files {
		if pathMatches(paths, f) {
			return true
		}
	}
	return false
}

func pathMatches(patterns []string, file string) bool {
	file = filepath.ToSlash(file)
	for _, p := range patterns {
		p = filepath.ToSlash(p)
		ok, err := doublestar.Match(p, file)
		if err == nil && ok {
			return true
		}
	}
	return false
}

// PathMatches reports whether file matches any pattern.
func PathMatches(patterns []string, file string) bool {
	return pathMatches(patterns, file)
}

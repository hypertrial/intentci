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
	All                     bool
	RequirementID           string
	ObligationID            string
	ChangedFiles            []string
	GlobalPaths             []string
	RunUnmappedRequirements bool
}

// Select chooses active requirements affected by changed files.
func Select(doc *ir.Document, opt Options) Selection {
	active := doc.ActiveRequirements()
	if opt.RequirementID != "" {
		byID := make(map[string]ir.Requirement, len(active))
		for _, requirement := range active {
			byID[requirement.ID] = requirement
		}
		selected := map[string]bool{opt.RequirementID: true}
		for changed := true; changed; {
			changed = false
			for id := range selected {
				for _, dependency := range byID[id].DependsOn {
					if !selected[dependency] {
						selected[dependency] = true
						changed = true
					}
				}
			}
		}
		var filtered []ir.Requirement
		for _, requirement := range active {
			if !selected[requirement.ID] {
				continue
			}
			if opt.ObligationID != "" && requirement.ID == opt.RequirementID {
				requirement = filterObligation(requirement, opt.ObligationID)
			}
			filtered = append(filtered, requirement)
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
	globalInvalidation := false
	for _, file := range opt.ChangedFiles {
		if pathMatches(opt.GlobalPaths, file) {
			globalInvalidation = true
			break
		}
	}
	for _, r := range active {
		if globalInvalidation || matchesPaths(r, opt.ChangedFiles) || contains(opt.ChangedFiles, r.SourcePath) || verifierInputsMatch(r, opt.ChangedFiles) {
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
			if pathMatches(r.AppliesTo.Paths, f) || pathMatches(r.Boundaries.Allowed, f) ||
				pathMatches(providerInputs(r), f) || r.SourcePath == f || pathMatches(opt.GlobalPaths, f) {
				hit = true
				break
			}
		}
		if !hit {
			unmapped = append(unmapped, f)
		}
	}
	if opt.RunUnmappedRequirements && len(unmapped) > 0 {
		for _, requirement := range active {
			if len(requirement.AppliesTo.Paths) == 0 {
				matched[requirement.ID] = true
			}
		}
		selected = selected[:0]
		for _, requirement := range active {
			if matched[requirement.ID] {
				selected = append(selected, requirement)
			}
		}
	}
	return Selection{Requirements: selected, Unmapped: unmapped}
}

func filterObligation(r ir.Requirement, id string) ir.Requirement {
	byID := make(map[string]ir.Obligation, len(r.Obligations))
	for _, obligation := range r.Obligations {
		byID[obligation.ID] = obligation
	}
	selected := map[string]bool{id: true}
	for changed := true; changed; {
		changed = false
		for obligationID := range selected {
			for _, dependency := range byID[obligationID].DependsOn {
				if !selected[dependency] {
					selected[dependency] = true
					changed = true
				}
			}
		}
	}
	var obs []ir.Obligation
	for _, o := range r.Obligations {
		if selected[o.ID] {
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

func verifierInputsMatch(requirement ir.Requirement, files []string) bool {
	inputs := providerInputs(requirement)
	for _, file := range files {
		if pathMatches(inputs, file) {
			return true
		}
	}
	return false
}

func providerInputs(requirement ir.Requirement) []string {
	var inputs []string
	var walk func(ir.VerifyNode)
	walk = func(node ir.VerifyNode) {
		if node.Provider != nil {
			inputs = append(inputs, node.Provider.Inputs...)
		}
		for _, child := range node.All {
			walk(child)
		}
		for _, child := range node.Any {
			walk(child)
		}
		if node.Not != nil {
			walk(*node.Not)
		}
	}
	for _, obligation := range requirement.Obligations {
		walk(obligation.Verify)
	}
	return inputs
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
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

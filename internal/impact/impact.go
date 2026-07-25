package impact

import (
	"sort"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/hypertrial/intentci/internal/contract"
)

// Selection is the set of affected requirements and checks to run.
type Selection struct {
	Requirements []SelectedRequirement
	CheckIDs     []string
}

// SelectedRequirement is an affected requirement with matching files.
type SelectedRequirement struct {
	Requirement contract.Requirement
	AffectedBy  []string
}

// Options controls impact analysis.
type Options struct {
	All                 bool
	Profile             string
	ForceRequirementIDs []string
	ForceCheckIDs       []string
	ExtraRequirements   []contract.Requirement
}

// Resolve maps changed files to requirements and checks.
func Resolve(c *contract.Contract, changed []string, opt Options) Selection {
	reqs := c.ApprovedBlocking()
	forceReq := map[string]struct{}{}
	for _, id := range opt.ForceRequirementIDs {
		forceReq[id] = struct{}{}
	}
	var selected []SelectedRequirement
	selectedIDs := map[string]struct{}{}
	checkSet := map[string]struct{}{}

	selectReq := func(r contract.Requirement, matched []string, forced bool) {
		if _, ok := selectedIDs[r.ID]; ok {
			return
		}
		if len(matched) == 0 {
			if forced || opt.All {
				matched = []string{"*"}
			} else if len(r.AppliesTo.Include) == 0 {
				matched = append([]string{}, changed...)
			}
		}
		selected = append(selected, SelectedRequirement{Requirement: r, AffectedBy: matched})
		selectedIDs[r.ID] = struct{}{}
		for _, id := range r.Verification.Checks {
			ch, ok := c.CheckByID(id)
			if !ok || !ch.HasProfile(opt.Profile) {
				continue
			}
			if forced || opt.All || len(ch.Inputs) == 0 || anyMatch(changed, ch.Inputs) || len(matched) > 0 {
				checkSet[id] = struct{}{}
			}
		}
	}

	for _, r := range reqs {
		matched := matchingFiles(changed, r.AppliesTo)
		_, forced := forceReq[r.ID]
		affects := opt.All || forced || len(matched) > 0 || len(r.AppliesTo.Include) == 0
		if !affects {
			continue
		}
		selectReq(r, matched, forced)
	}

	// Force-select requirements by ID even if not approved-blocking path-matched
	// (still only approved blocking from contract list above). Also add extras (ACs).
	for _, r := range opt.ExtraRequirements {
		selectReq(r, []string{"*"}, true)
	}

	for _, id := range opt.ForceCheckIDs {
		if ch, ok := c.CheckByID(id); ok && ch.HasProfile(opt.Profile) {
			checkSet[id] = struct{}{}
		}
	}

	checks := c.CheckMap()
	changedDeps := true
	for changedDeps {
		changedDeps = false
		for id := range checkSet {
			ch := checks[id]
			for _, dep := range ch.DependsOn {
				if _, ok := checkSet[dep]; !ok {
					if d, ok := checks[dep]; ok && d.HasProfile(opt.Profile) {
						checkSet[dep] = struct{}{}
						changedDeps = true
					}
				}
			}
		}
	}

	ids := make([]string, 0, len(checkSet))
	for id := range checkSet {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return Selection{Requirements: selected, CheckIDs: ids}
}

func matchingFiles(files []string, applies contract.AppliesTo) []string {
	var out []string
	for _, f := range files {
		if pathMatches(f, applies) {
			out = append(out, f)
		}
	}
	return out
}

func pathMatches(path string, applies contract.AppliesTo) bool {
	if len(applies.Include) == 0 {
		return false
	}
	included := false
	for _, g := range applies.Include {
		if match(g, path) {
			included = true
			break
		}
	}
	if !included {
		return false
	}
	for _, g := range applies.Exclude {
		if match(g, path) {
			return false
		}
	}
	return true
}

func anyMatch(files, patterns []string) bool {
	for _, f := range files {
		for _, p := range patterns {
			if match(p, f) {
				return true
			}
		}
	}
	return false
}

func match(pattern, path string) bool {
	ok, err := doublestar.PathMatch(pattern, path)
	return err == nil && ok
}

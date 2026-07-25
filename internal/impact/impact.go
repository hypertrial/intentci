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
	// All selects every approved blocking requirement regardless of paths.
	All bool
	// Profile is "fast" or "full".
	Profile string
}

// Resolve maps changed files to requirements and checks.
func Resolve(c *contract.Contract, changed []string, opt Options) Selection {
	reqs := c.ApprovedBlocking()
	var selected []SelectedRequirement
	checkSet := map[string]struct{}{}

	for _, r := range reqs {
		matched := matchingFiles(changed, r.AppliesTo)
		affects := opt.All || len(matched) > 0 || len(r.AppliesTo.Include) == 0
		if !affects {
			continue
		}
		if len(matched) == 0 {
			if !opt.All && len(r.AppliesTo.Include) == 0 {
				// No path rules: conservatively treat as affected for any change.
				matched = append([]string{}, changed...)
			} else if opt.All {
				// Full verification without path hits still records the forced selection.
				matched = []string{"*"}
			}
		}
		selected = append(selected, SelectedRequirement{
			Requirement: r,
			AffectedBy:  matched,
		})
		for _, id := range r.Verification.Checks {
			ch, ok := c.CheckByID(id)
			if !ok {
				continue
			}
			if !ch.HasProfile(opt.Profile) {
				continue
			}
			// Also select checks whose inputs match changed files, even if
			// already selected via requirement mapping.
			if opt.All || len(ch.Inputs) == 0 || anyMatch(changed, ch.Inputs) || len(matched) > 0 {
				checkSet[id] = struct{}{}
			}
		}
	}

	// Include dependency prerequisites.
	checks := c.CheckMap()
	changedDeps := true
	for changedDeps {
		changedDeps = false
		for id := range checkSet {
			ch, ok := checks[id]
			if !ok {
				continue
			}
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

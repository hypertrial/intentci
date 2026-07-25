package contractdiff

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/pkg/protocol"
)

// Diff reports weakenings of the Product Contract from base to head.
func Diff(base, head *contract.Contract) []protocol.ContractChange {
	if base == nil || head == nil {
		return nil
	}
	var out []protocol.ContractChange
	if base.Policy.BlocksOnUnknown() && !head.Policy.BlocksOnUnknown() {
		out = append(out, protocol.ContractChange{
			Type:    "policy_unknown_blocks_disabled",
			Summary: "policy.unknown_blocks was disabled relative to the base commit.",
		})
	}
	if base.Policy.BlocksOnUnverified() && !head.Policy.BlocksOnUnverified() {
		out = append(out, protocol.ContractChange{
			Type:    "policy_unverified_blocks_disabled",
			Summary: "policy.unverified_blocks was disabled relative to the base commit.",
		})
	}
	out = append(out, semanticPolicyWeakenings(base.Policy.Semantic, head.Policy.Semantic)...)
	baseReqs := indexReqs(base.Requirements)
	headReqs := indexReqs(head.Requirements)
	baseChecks := indexChecks(base.Checks)
	headChecks := indexChecks(head.Checks)

	for id, br := range baseReqs {
		if br.Status != "approved" || br.Severity != "blocking" {
			continue
		}
		hr, ok := headReqs[id]
		if !ok {
			out = append(out, protocol.ContractChange{
				Type:    "requirement_removed",
				ID:      id,
				Summary: fmt.Sprintf("Approved blocking requirement %s was removed.", id),
			})
			continue
		}
		if hr.Status == "draft" {
			out = append(out, protocol.ContractChange{
				Type:    "requirement_demoted_draft",
				ID:      id,
				Summary: fmt.Sprintf("Approved requirement %s was demoted to draft.", id),
			})
		}
		if hr.Status == "deprecated" {
			out = append(out, protocol.ContractChange{
				Type:    "requirement_deprecated",
				ID:      id,
				Summary: fmt.Sprintf("Approved requirement %s was marked deprecated.", id),
			})
		}
		if br.Severity == "blocking" && hr.Severity == "advisory" {
			out = append(out, protocol.ContractChange{
				Type:    "severity_lowered",
				ID:      id,
				Summary: fmt.Sprintf("Requirement %s severity lowered from blocking to advisory.", id),
			})
		}
		if br.Verification.VerificationMode() == "all" && hr.Verification.VerificationMode() == "any" {
			out = append(out, protocol.ContractChange{
				Type:    "mode_narrowed",
				ID:      id,
				Summary: fmt.Sprintf("Requirement %s verification mode narrowed from all to any.", id),
			})
		}
		if semanticRequirementWeakened(br.Verification.Semantic, hr.Verification.Semantic) {
			out = append(out, protocol.ContractChange{
				Type:    "semantic_disabled",
				ID:      id,
				Summary: fmt.Sprintf("Requirement %s disabled or weakened semantic analysis.", id),
			})
		}
		removedChecks := missingStrings(br.Verification.Checks, hr.Verification.Checks)
		for _, cid := range removedChecks {
			out = append(out, protocol.ContractChange{
				Type:    "check_removed",
				ID:      id,
				Summary: fmt.Sprintf("Requirement %s removed required check %s.", id, cid),
			})
		}
		if narrowedAppliesTo(br.AppliesTo, hr.AppliesTo) {
			out = append(out, protocol.ContractChange{
				Type:    "applies_to_narrowed",
				ID:      id,
				Summary: fmt.Sprintf("Requirement %s applicability was narrowed.", id),
			})
		}
	}

	for id, bc := range baseChecks {
		hc, ok := headChecks[id]
		if !ok {
			// Only flag if referenced by a base approved blocking requirement.
			if checkUsedByApprovedBlocking(base, id) {
				out = append(out, protocol.ContractChange{
					Type:    "check_removed",
					ID:      id,
					Summary: fmt.Sprintf("Blocking check %s was deleted.", id),
				})
			}
			continue
		}
		if checkModified(bc, hc) && checkUsedByApprovedBlocking(base, id) {
			out = append(out, protocol.ContractChange{
				Type:    "check_modified",
				ID:      id,
				Summary: fmt.Sprintf("Blocking check %s was modified.", id),
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Type == out[j].Type {
			return out[i].ID < out[j].ID
		}
		return out[i].Type < out[j].Type
	})
	return out
}

func indexReqs(reqs []contract.Requirement) map[string]contract.Requirement {
	m := make(map[string]contract.Requirement, len(reqs))
	for _, r := range reqs {
		m[r.ID] = r
	}
	return m
}

func indexChecks(checks []contract.Check) map[string]contract.Check {
	m := make(map[string]contract.Check, len(checks))
	for _, c := range checks {
		m[c.ID] = c
	}
	return m
}

func missingStrings(want, have []string) []string {
	set := map[string]struct{}{}
	for _, s := range have {
		set[s] = struct{}{}
	}
	var out []string
	for _, s := range want {
		if _, ok := set[s]; !ok {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func narrowedAppliesTo(base, head contract.AppliesTo) bool {
	// Exclude growth always narrows.
	if len(missingStrings(head.Exclude, base.Exclude)) > 0 {
		return true
	}
	// Head covers all paths → not an include narrowing (including ** / empty include).
	if coversAllPaths(head) {
		return false
	}
	// Base covered all, head is restricted → narrowed.
	if coversAllPaths(base) {
		return true
	}
	// Both restricted: treat as narrowing only when head includes are a strict subset
	// of base includes. Pattern rewrites (different strings) are not treated as narrowing
	// so Effective does not under-enforce by restoring narrower base globs.
	missingFromHead := missingStrings(base.Include, head.Include)
	extraInHead := missingStrings(head.Include, base.Include)
	return len(missingFromHead) > 0 && len(extraInHead) == 0
}

func coversAllPaths(a contract.AppliesTo) bool {
	if len(a.Include) == 0 {
		return true
	}
	for _, p := range a.Include {
		if p == "**" {
			return true
		}
	}
	return false
}

func checkUsedByApprovedBlocking(c *contract.Contract, checkID string) bool {
	for _, r := range c.ApprovedBlocking() {
		for _, id := range r.Verification.Checks {
			if id == checkID {
				return true
			}
		}
	}
	return false
}

func checkModified(a, b contract.Check) bool {
	return a.Command != b.Command ||
		a.Timeout != b.Timeout ||
		a.Cache != b.Cache ||
		a.Exclusive != b.Exclusive ||
		!reflect.DeepEqual(sortedCopy(a.DependsOn), sortedCopy(b.DependsOn)) ||
		!reflect.DeepEqual(sortedCopy(a.Profiles), sortedCopy(b.Profiles)) ||
		!reflect.DeepEqual(sortedCopy(a.Inputs), sortedCopy(b.Inputs)) ||
		!resultsEqual(a.Results, b.Results)
}

func resultsEqual(a, b *contract.Results) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Format == b.Format && a.Path == b.Path
}

func sortedCopy(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func semanticPolicyWeakenings(base, head contract.SemanticPolicy) []protocol.ContractChange {
	var out []protocol.ContractChange
	if base.Enabled && !head.Enabled {
		out = append(out, protocol.ContractChange{
			Type:    "semantic_policy_disabled",
			Summary: "policy.semantic.enabled was disabled relative to the base commit.",
		})
	}
	if base.Enabled && head.Enabled {
		if base.EnforcementOrDefault() == "blocking" && head.EnforcementOrDefault() == "advisory" {
			out = append(out, protocol.ContractChange{
				Type:    "semantic_enforcement_softened",
				Summary: "policy.semantic.enforcement was softened from blocking to advisory.",
			})
		}
		if base.ConfidenceThresholdOrDefault() > head.ConfidenceThresholdOrDefault() {
			out = append(out, protocol.ContractChange{
				Type:    "semantic_threshold_lowered",
				Summary: "policy.semantic.confidence_threshold was lowered relative to the base commit.",
			})
		}
		if base.Provider != nil && head.Provider == nil {
			out = append(out, protocol.ContractChange{
				Type:    "semantic_provider_removed",
				Summary: "policy.semantic.provider was removed relative to the base commit.",
			})
		} else if providerChanged(base.Provider, head.Provider) {
			out = append(out, protocol.ContractChange{
				Type:    "semantic_provider_changed",
				Summary: "policy.semantic.provider was changed relative to the base commit.",
			})
		}
	}
	return out
}

func providerChanged(base, head *contract.SemanticProvider) bool {
	if base == nil || head == nil {
		return false
	}
	return base.Type != head.Type || base.Command != head.Command || base.URL != head.URL
}

func semanticRequirementWeakened(base, head string) bool {
	rank := func(s string) int {
		switch normalizeSemanticMode(s) {
		case "required":
			return 2
		case "optional":
			return 1
		default: // off
			return 0
		}
	}
	return rank(base) > rank(head)
}

func normalizeSemanticMode(s string) string {
	switch s {
	case "required", "optional", "off":
		return s
	default:
		// Omitted/empty matches runtime default (optional).
		return "optional"
	}
}

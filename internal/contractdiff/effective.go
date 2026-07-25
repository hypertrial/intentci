package contractdiff

import (
	"sort"

	"github.com/hypertrial/intentci/internal/contract"
)

// Effective builds a verification contract that is at least as strong as base.
// Base approved+blocking requirements that head removes or weakens remain active
// using the base definition. Head additions and strengthenings also apply.
func Effective(base, head *contract.Contract) *contract.Contract {
	if head == nil {
		return base
	}
	if base == nil {
		return head
	}
	out := *head
	reqByID := indexReqs(head.Requirements)
	var reqs []contract.Requirement
	seen := map[string]struct{}{}

	// Start with head requirements, restoring weakened base approved+blocking defs.
	for _, hr := range head.Requirements {
		if br, ok := indexReqs(base.Requirements)[hr.ID]; ok && br.Status == "approved" && br.Severity == "blocking" {
			if weakened(br, hr) {
				reqs = append(reqs, br)
				seen[br.ID] = struct{}{}
				continue
			}
		}
		reqs = append(reqs, hr)
		seen[hr.ID] = struct{}{}
	}
	for _, br := range base.ApprovedBlocking() {
		if _, ok := seen[br.ID]; ok {
			continue
		}
		reqs = append(reqs, br)
		seen[br.ID] = struct{}{}
	}
	_ = reqByID

	checkByID := indexChecks(head.Checks)
	for _, br := range base.ApprovedBlocking() {
		for _, cid := range br.Verification.Checks {
			if _, ok := checkByID[cid]; ok {
				continue
			}
			if bc, ok := indexChecks(base.Checks)[cid]; ok {
				checkByID[cid] = bc
			}
		}
	}
	// Also restore base check definitions when head modified a blocking check.
	for id, bc := range indexChecks(base.Checks) {
		hc, ok := checkByID[id]
		if !ok {
			continue
		}
		if checkUsedByApprovedBlocking(base, id) && checkModified(bc, hc) {
			checkByID[id] = bc
		}
	}

	var checks []contract.Check
	for _, ch := range checkByID {
		checks = append(checks, ch)
	}
	sort.Slice(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
	sort.Slice(reqs, func(i, j int) bool { return reqs[i].ID < reqs[j].ID })

	out.Requirements = reqs
	out.Checks = checks
	out.Policy = stricterPolicy(base.Policy, head.Policy)
	return &out
}

// stricterPolicy keeps head policy but restores base blocking flags when head softens them.
func stricterPolicy(base, head contract.Policy) contract.Policy {
	p := head
	if base.BlocksOnUnknown() {
		t := true
		p.UnknownBlocks = &t
	}
	if base.BlocksOnUnverified() {
		t := true
		p.UnverifiedBlocks = &t
	}
	return p
}

func weakened(base, head contract.Requirement) bool {
	if head.Status != "approved" {
		return true
	}
	if head.Severity != "blocking" {
		return true
	}
	if base.Verification.VerificationMode() == "all" && head.Verification.VerificationMode() == "any" {
		return true
	}
	if len(missingStrings(base.Verification.Checks, head.Verification.Checks)) > 0 {
		return true
	}
	if narrowedAppliesTo(base.AppliesTo, head.AppliesTo) {
		return true
	}
	return false
}

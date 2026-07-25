package semantic

import (
	"fmt"
	"strings"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/pkg/protocol"
)

// MergeOptions controls how semantic findings affect requirement statuses.
type MergeOptions struct {
	Policy        contract.SemanticPolicy
	Contract      *contract.Contract
	SemanticModes map[string]string // requirement id → required|optional|off
}

// Apply merges provider findings into requirement results.
// Deterministic FAIL/UNKNOWN are never upgraded. Semantic never upgrades a status.
func Apply(reqs []protocol.RequirementResult, findings []Finding, opt MergeOptions) []protocol.RequirementResult {
	byID := map[string][]Finding{}
	for _, f := range findings {
		byID[f.RequirementID] = append(byID[f.RequirementID], f)
	}
	threshold := opt.Policy.ConfidenceThresholdOrDefault()
	enforcement := opt.Policy.EnforcementOrDefault()

	out := make([]protocol.RequirementResult, len(reqs))
	for i, rr := range reqs {
		out[i] = copyRequirementResult(rr)
		cur := &out[i]
		if cur.Status == protocol.ReqWaived {
			continue
		}
		semMode := semanticMode(opt, cur.ID)
		if semMode == "off" {
			continue
		}
		for _, f := range byID[cur.ID] {
			applyOne(cur, f, semMode, enforcement, threshold, opt)
		}
	}
	return out
}

// MarkUnavailable marks requirements with semantic: required as unknown when the provider cannot run.
func MarkUnavailable(reqs []protocol.RequirementResult, opt MergeOptions, reason string) []protocol.RequirementResult {
	out := make([]protocol.RequirementResult, len(reqs))
	for i, rr := range reqs {
		out[i] = copyRequirementResult(rr)
		cur := &out[i]
		if cur.Status == protocol.ReqWaived || cur.Status == protocol.ReqFail {
			continue
		}
		if semanticMode(opt, cur.ID) != "required" {
			continue
		}
		cur.Status = protocol.ReqUnknown
		cur.Reason = reason
		cur.Findings = append(cur.Findings, protocol.Finding{
			Type:    "semantic_unavailable",
			Summary: reason,
		})
		cur.Findings = appendCompletion(cur.Findings, cur.ID)
	}
	return out
}

func copyRequirementResult(rr protocol.RequirementResult) protocol.RequirementResult {
	out := rr
	if rr.AffectedBy != nil {
		out.AffectedBy = append([]string{}, rr.AffectedBy...)
	}
	if rr.Checks != nil {
		out.Checks = append([]protocol.CheckRef{}, rr.Checks...)
	}
	if rr.Evidence != nil {
		out.Evidence = append([]protocol.Evidence{}, rr.Evidence...)
	}
	if rr.Findings != nil {
		out.Findings = append([]protocol.Finding{}, rr.Findings...)
	}
	return out
}

func semanticMode(opt MergeOptions, id string) string {
	if opt.SemanticModes != nil {
		if m, ok := opt.SemanticModes[id]; ok {
			if m == "" {
				return "optional"
			}
			return m
		}
	}
	if opt.Contract != nil {
		for _, r := range opt.Contract.Requirements {
			if r.ID == id {
				if r.Verification.Semantic == "" {
					return "optional"
				}
				return r.Verification.Semantic
			}
		}
	}
	return "optional"
}

func applyOne(rr *protocol.RequirementResult, f Finding, semMode, enforcement string, threshold float64, opt MergeOptions) {
	if rr.Status == protocol.ReqFail || rr.Status == protocol.ReqUnknown {
		attachFinding(rr, f, false)
		return
	}

	switch f.Assessment {
	case AssessmentAligned, AssessmentNotAffected:
		attachFinding(rr, f, false)
		return
	case AssessmentContradiction:
		if canBlockingFail(rr, f, semMode, enforcement, threshold, opt) {
			rr.Status = protocol.ReqFail
			rr.Reason = "Semantic analysis found a contradiction with sufficient evidence."
			attachFinding(rr, f, true)
			rr.Findings = appendCompletion(rr.Findings, rr.ID)
			return
		}
		downgradeUnverified(rr, f, "semantic_contradiction")
	case AssessmentInsufficientEvidence:
		downgradeUnverified(rr, f, "semantic_insufficient_evidence")
	case AssessmentUncertain:
		downgradeUnverified(rr, f, "semantic_uncertain")
	default:
		downgradeUnverified(rr, f, "semantic_uncertain")
	}
}

func canBlockingFail(rr *protocol.RequirementResult, f Finding, semMode, enforcement string, threshold float64, opt MergeOptions) bool {
	if enforcement != "blocking" {
		return false
	}
	if semMode != "required" {
		return false
	}
	if f.Confidence < threshold {
		return false
	}
	if !hasPathEvidence(f) {
		return false
	}
	if req, ok := lookupRequirement(opt.Contract, rr.ID); ok {
		return req.Status == "approved"
	}
	// Synthetic acceptance criteria are treated as approved for the change.
	return true
}

func downgradeUnverified(rr *protocol.RequirementResult, f Finding, findingType string) {
	if rr.Status == protocol.ReqPass {
		rr.Status = protocol.ReqUnverified
		rr.Reason = "Semantic analysis reported unresolved product-level concerns."
	}
	attachTypedFinding(rr, f, findingType)
	if rr.Status != protocol.ReqPass {
		rr.Findings = appendCompletion(rr.Findings, rr.ID)
	}
}

func attachFinding(rr *protocol.RequirementResult, f Finding, asContradiction bool) {
	typ := "semantic_" + f.Assessment
	if asContradiction {
		typ = "semantic_contradiction"
	}
	attachTypedFinding(rr, f, typ)
}

func attachTypedFinding(rr *protocol.RequirementResult, f Finding, typ string) {
	summary := f.Summary
	if summary == "" {
		summary = fmt.Sprintf("Semantic assessment %s (confidence %.2f)", f.Assessment, f.Confidence)
	}
	rr.Findings = append(rr.Findings, protocol.Finding{Type: typ, Summary: summary})
	for _, e := range f.Evidence {
		if strings.TrimSpace(e.Path) == "" {
			continue
		}
		rr.Evidence = append(rr.Evidence, protocol.Evidence{
			Type:      "semantic",
			Path:      e.Path,
			LineStart: e.LineStart,
			LineEnd:   e.LineEnd,
			Summary:   summary,
		})
	}
	for _, m := range f.MissingEvidence {
		if strings.TrimSpace(m) == "" {
			continue
		}
		rr.Findings = append(rr.Findings, protocol.Finding{
			Type:    "semantic_missing_evidence",
			Summary: m,
		})
	}
}

func hasPathEvidence(f Finding) bool {
	for _, e := range f.Evidence {
		if strings.TrimSpace(e.Path) != "" {
			return true
		}
	}
	return false
}

func lookupRequirement(c *contract.Contract, id string) (contract.Requirement, bool) {
	if c == nil {
		return contract.Requirement{}, false
	}
	for _, r := range c.Requirements {
		if r.ID == id {
			return r, true
		}
	}
	return contract.Requirement{}, false
}

func appendCompletion(findings []protocol.Finding, id string) []protocol.Finding {
	for _, f := range findings {
		if f.Type == "completion_condition" {
			return findings
		}
	}
	return append(findings, protocol.Finding{
		Type:    "completion_condition",
		Summary: fmt.Sprintf("Resolve semantic findings for requirement %s.", id),
	})
}

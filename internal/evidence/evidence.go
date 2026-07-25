package evidence

import (
	"fmt"
	"strings"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/pkg/protocol"
)

// Assign builds requirement results from impact selection and check outcomes.
// waived maps requirement IDs to applied waivers (optional).
func Assign(sel impact.Selection, checkResults map[string]runner.Result, profile string, c *contract.Contract, waived map[string]protocol.Waiver) []protocol.RequirementResult {
	out := make([]protocol.RequirementResult, 0, len(sel.Requirements))
	for _, sr := range sel.Requirements {
		r := sr.Requirement
		rr := protocol.RequirementResult{
			ID:         r.ID,
			Title:      r.Title,
			Severity:   r.Severity,
			AffectedBy: sr.AffectedBy,
			Checks:     []protocol.CheckRef{},
			Evidence:   []protocol.Evidence{},
			Findings:   []protocol.Finding{},
		}
		if w, ok := waived[r.ID]; ok {
			rr.Status = protocol.ReqWaived
			rr.Reason = fmt.Sprintf("Waived by %s: %s", w.ID, w.Reason)
			out = append(out, rr)
			continue
		}

		mode := r.Verification.VerificationMode()
		var passN, failN, unknownN, missingN, skippedN int

		for _, checkID := range r.Verification.Checks {
			ch, ok := c.CheckByID(checkID)
			if !ok {
				missingN++
				rr.Findings = append(rr.Findings, protocol.Finding{
					Type:    "missing_check",
					Summary: fmt.Sprintf("Check %q is not defined.", checkID),
				})
				continue
			}
			if !ch.HasProfile(profile) {
				missingN++
				rr.Findings = append(rr.Findings, protocol.Finding{
					Type:    "missing_evidence",
					Summary: fmt.Sprintf("Required check %q is not in the %s profile; run intentci verify for full evidence.", checkID, profile),
				})
				rr.Checks = append(rr.Checks, protocol.CheckRef{
					ID:     checkID,
					Status: protocol.CheckSkipped,
				})
				continue
			}
			res, ran := checkResults[checkID]
			if !ran {
				missingN++
				rr.Findings = append(rr.Findings, protocol.Finding{
					Type:    "missing_evidence",
					Summary: fmt.Sprintf("Required check %q did not run.", checkID),
				})
				continue
			}
			rr.Checks = append(rr.Checks, protocol.CheckRef{
				ID:         checkID,
				Status:     res.Status,
				ExitCode:   res.ExitCode,
				DurationMS: res.DurationMS,
			})
			switch res.Status {
			case protocol.CheckPass:
				passN++
				rr.Evidence = append(rr.Evidence, protocol.Evidence{
					Type:    "check",
					Summary: fmt.Sprintf("Check %q passed.", checkID),
				})
			case protocol.CheckFail:
				failN++
				rr.Findings = append(rr.Findings, protocol.Finding{
					Type:    "deterministic_failure",
					Summary: fmt.Sprintf("Check %q failed: %s", checkID, firstLine(res.Reason, res.Stderr, res.Stdout)),
				})
				rr.Evidence = append(rr.Evidence, protocol.Evidence{
					Type:    "check",
					Summary: fmt.Sprintf("Check %q failed.", checkID),
				})
			case protocol.CheckUnknown:
				unknownN++
				rr.Findings = append(rr.Findings, protocol.Finding{
					Type:    "execution_uncertainty",
					Summary: fmt.Sprintf("Check %q result is unknown: %s", checkID, res.Reason),
				})
			case protocol.CheckSkipped:
				skippedN++
				rr.Findings = append(rr.Findings, protocol.Finding{
					Type:    "missing_evidence",
					Summary: fmt.Sprintf("Check %q was skipped: %s", checkID, res.Reason),
				})
			}
		}

		rr.Status, rr.Reason = resolveStatus(mode, passN, failN, unknownN, missingN, skippedN, len(r.Verification.Checks))
		if rr.Status != protocol.ReqPass {
			rr.Findings = appendCompletion(rr.Findings, r)
		}
		out = append(out, rr)
	}
	return out
}

func resolveStatus(mode string, passN, failN, unknownN, missingN, skippedN, total int) (string, string) {
	if failN > 0 {
		return protocol.ReqFail, "One or more required checks failed."
	}
	if unknownN > 0 {
		return protocol.ReqUnknown, "One or more required checks returned an unknown result."
	}
	if missingN > 0 || skippedN > 0 {
		return protocol.ReqUnverified, "Required verification evidence is missing."
	}
	if mode == "any" {
		if passN > 0 {
			return protocol.ReqPass, ""
		}
		return protocol.ReqUnverified, "No required check passed."
	}
	// mode all
	if passN == total && total > 0 {
		return protocol.ReqPass, ""
	}
	return protocol.ReqUnverified, "Not all required checks passed."
}

func appendCompletion(findings []protocol.Finding, r contract.Requirement) []protocol.Finding {
	for _, f := range findings {
		if f.Type == "completion_condition" {
			return findings
		}
	}
	return append(findings, protocol.Finding{
		Type:    "completion_condition",
		Summary: fmt.Sprintf("Satisfy requirement %s: %s", r.ID, r.Statement),
	})
}

func firstLine(parts ...string) string {
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if i := strings.IndexByte(p, '\n'); i >= 0 {
			return strings.TrimSpace(p[:i])
		}
		return p
	}
	return "see check output"
}

// Overall computes aggregate status and exit code from policy.
func Overall(reqs []protocol.RequirementResult, policy contract.Policy) (status string, exitCode int) {
	var fail, unverified, unknown bool
	for _, r := range reqs {
		if r.Severity != "blocking" {
			continue
		}
		switch r.Status {
		case protocol.ReqFail:
			fail = true
		case protocol.ReqUnverified:
			unverified = true
		case protocol.ReqUnknown:
			unknown = true
		case protocol.ReqWaived:
			// Waived blocking requirements do not block the run.
		}
	}
	switch {
	case fail:
		return protocol.StatusFail, 10
	case unknown && policy.BlocksOnUnknown():
		return protocol.StatusUnknown, 12
	case unverified && policy.BlocksOnUnverified():
		return protocol.StatusUnverified, 11
	case unknown:
		return protocol.StatusUnknown, 0
	case unverified:
		return protocol.StatusUnverified, 0
	default:
		return protocol.StatusPass, 0
	}
}

// Summarize counts statuses.
func Summarize(reqs []protocol.RequirementResult, checksExecuted int) protocol.Summary {
	var s protocol.Summary
	s.ChecksExecuted = checksExecuted
	for _, r := range reqs {
		switch r.Status {
		case protocol.ReqPass:
			s.Pass++
		case protocol.ReqFail:
			s.Fail++
		case protocol.ReqUnverified:
			s.Unverified++
		case protocol.ReqUnknown:
			s.Unknown++
		case protocol.ReqWaived:
			s.Waived++
		case protocol.ReqNotAffected:
			s.NotAffected++
		}
	}
	return s
}

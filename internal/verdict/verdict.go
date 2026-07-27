package verdict

import (
	"cmp"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
)

// Obligation verdict values.
const (
	Pass           = "pass"
	Fail           = "fail"
	Unproven       = "unproven"
	Uncertain      = "uncertain"
	Skipped        = "skipped"
	ReviewRequired = "review_required"
	Error          = "error"
)

// ObligationResult is the verdict for one obligation.
type ObligationResult struct {
	ID        string              `json:"id"`
	Statement string              `json:"statement"`
	Required  bool                `json:"required"`
	Verdict   string              `json:"verdict"`
	Reason    string              `json:"reason,omitempty"`
	Evidence  []provider.Evidence `json:"evidence,omitempty"`
}

// RequirementResult is the aggregated requirement verdict.
type RequirementResult struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Priority    string             `json:"priority"`
	Verdict     string             `json:"verdict"`
	Reason      string             `json:"reason,omitempty"`
	Obligations []ObligationResult `json:"obligations"`
}

// RunResult is the overall run.
type RunResult struct {
	Verdict      string              `json:"verdict"`
	Requirements []RequirementResult `json:"requirements"`
}

// EvidencePolicy controls whether non-deterministic evidence may satisfy an obligation.
type EvidencePolicy struct {
	Class               string
	ConfidenceThreshold *float64
}

// EvaluateNode evaluates a verify expression given leaf results keyed by provider spec id.
func EvaluateNode(n ir.VerifyNode, leaves map[string]provider.Result) (string, string, []provider.Evidence) {
	return EvaluateNodeWithPolicy(n, leaves, EvidencePolicy{})
}

// EvaluateNodeWithPolicy evaluates an expression with obligation evidence rules.
func EvaluateNodeWithPolicy(n ir.VerifyNode, leaves map[string]provider.Result, policy EvidencePolicy) (string, string, []provider.Evidence) {
	if n.Provider != nil {
		key := n.Provider.ID
		if key == "" {
			key = n.Provider.Provider
		}
		res, ok := leaves[key]
		if !ok {
			return Unproven, "missing provider evidence", nil
		}
		return leafVerdict(res, policy)
	}
	if len(n.All) > 0 {
		var ev []provider.Evidence
		worst := Pass
		reason := ""
		for _, c := range n.All {
			v, r, e := EvaluateNodeWithPolicy(c, leaves, policy)
			ev = append(ev, e...)
			worst, reason = worse(worst, reason, v, r)
		}
		return worst, reason, ev
	}
	if len(n.Any) > 0 {
		var ev []provider.Evidence
		values := make([]string, 0, len(n.Any))
		reasons := make([]string, 0, len(n.Any))
		for _, c := range n.Any {
			v, r, e := EvaluateNodeWithPolicy(c, leaves, policy)
			ev = append(ev, e...)
			values = append(values, v)
			reasons = append(reasons, r)
			if v == Pass {
				return Pass, r, ev
			}
		}
		allFailed := true
		for _, value := range values {
			if value != Fail {
				allFailed = false
			}
		}
		if allFailed {
			return Fail, "all alternatives failed", ev
		}
		value, reason := Unproven, "no alternative produced sufficient evidence"
		for index, candidate := range values {
			if candidate == Fail {
				continue
			}
			value, reason = worse(value, reason, candidate, reasons[index])
		}
		return value, reason, ev
	}
	if n.Not != nil {
		v, r, e := EvaluateNodeWithPolicy(*n.Not, leaves, policy)
		switch v {
		case Pass:
			return Fail, "negation of pass", e
		case Fail:
			return Pass, "negation of fail", e
		default:
			return v, r, e
		}
	}
	return Unproven, "empty verify expression", nil
}

func leafVerdict(res provider.Result, policy EvidencePolicy) (string, string, []provider.Evidence) {
	if res.Status == "error" {
		return Error, firstDiag(res), res.Evidence
	}
	if res.Status == "skipped" {
		return Skipped, "provider skipped", res.Evidence
	}
	if policy.Class == "human" {
		return ReviewRequired, "obligation requires human evidence", res.Evidence
	}
	if policy.Class == "informational" {
		return Unproven, "informational evidence cannot satisfy an obligation", res.Evidence
	}
	uncertainReason := ""
	for _, e := range res.Evidence {
		if e.Data != nil && e.Data["retry_superseded"] == true {
			continue
		}
		if e.Class == "manual" || e.Class == "human" || (e.Data != nil && e.Data["review_required"] == true) {
			return ReviewRequired, e.Summary, res.Evidence
		}
		if e.Class == "informational" {
			continue
		}
		if e.Class == "probabilistic" {
			if policy.Class != "probabilistic" || policy.ConfidenceThreshold == nil {
				uncertainReason = "probabilistic evidence is not explicitly permitted"
			} else if e.Confidence == nil || *e.Confidence < *policy.ConfidenceThreshold {
				uncertainReason = "probabilistic confidence is below the required threshold"
			} else if e.Passed == nil || !*e.Passed {
				uncertainReason = e.Summary
			}
		}
		if policy.Class == "deterministic" && e.Class != "deterministic" && e.Class != "informational" {
			uncertainReason = "evidence does not meet the deterministic evidence requirement"
		}
		if e.Passed != nil && !*e.Passed {
			if e.Class == "probabilistic" {
				continue
			}
			return Fail, e.Summary, res.Evidence
		}
	}
	if len(res.Evidence) == 0 {
		return Unproven, "no evidence produced", nil
	}
	for _, e := range res.Evidence {
		if e.Data != nil && e.Data["retry_superseded"] == true {
			continue
		}
		if e.Class == "informational" {
			return Unproven, "informational evidence cannot satisfy an obligation", res.Evidence
		}
		if e.Class == "probabilistic" {
			continue
		}
		if e.Passed == nil {
			return Unproven, "evidence missing pass/fail", res.Evidence
		}
	}
	if uncertainReason != "" {
		return Uncertain, uncertainReason, res.Evidence
	}
	return Pass, "all evidence passed", res.Evidence
}

func firstDiag(res provider.Result) string {
	if len(res.Diagnostics) > 0 {
		return res.Diagnostics[0]
	}
	return "provider error"
}

func worse(cur string, curReason string, next string, nextReason string) (string, string) {
	if rank(next) > rank(cur) {
		return next, nextReason
	}
	if curReason == "" {
		return cur, nextReason
	}
	return cur, curReason
}

func rank(v string) int {
	switch v {
	case Pass:
		return 0
	case Skipped:
		return 1
	case Unproven:
		return 2
	case Uncertain:
		return 3
	case ReviewRequired:
		return 4
	case Error:
		return 5
	case Fail:
		return 6
	default:
		return 2
	}
}

// AggregateRequirement combines obligation verdicts.
// Optional (required: false) obligations never block the requirement by default.
func AggregateRequirement(r ir.Requirement, obs []ObligationResult) RequirementResult {
	out := RequirementResult{
		ID: r.ID, Title: r.Title, Priority: r.Priority, Obligations: obs, Verdict: Pass,
	}
	hasRequired := false
	for index := range obs {
		o := obs[index]
		if !o.Required {
			continue
		}
		hasRequired = true
		if o.Verdict == Skipped {
			o.Verdict = Unproven
			if o.Reason == "" {
				o.Reason = "required obligation was not executed"
			}
			out.Obligations[index] = o
		}
		if rank(o.Verdict) > rank(out.Verdict) {
			out.Verdict = o.Verdict
			out.Reason = o.Reason
		}
	}
	if len(obs) == 0 {
		out.Verdict = Unproven
		out.Reason = "no obligations selected"
	} else if !hasRequired {
		out.Verdict = Pass
		out.Reason = "no required obligations"
	}
	return out
}

// AggregateRun combines requirement verdicts. Only required priority blocks by default.
func AggregateRun(reqs []RequirementResult) RunResult {
	if reqs == nil {
		reqs = []RequirementResult{}
	}
	out := RunResult{Requirements: reqs, Verdict: Pass}
	for _, r := range reqs {
		switch r.Priority {
		case "recommended", "informational":
			continue
		case "required":
			if cmp.Compare(rank(r.Verdict), rank(out.Verdict)) == 1 {
				out.Verdict = r.Verdict
			}
		default:
			out.Verdict = Error
		}
	}
	if len(reqs) == 0 {
		out.Verdict = Pass
	}
	return out
}

// ExitCode maps a run verdict to a process exit code (§17).
func ExitCode(v string) int {
	switch v {
	case Pass:
		return 0
	case Fail:
		return 1
	case Unproven:
		return 2
	case Uncertain:
		return 3
	case ReviewRequired:
		return 4
	case Error:
		return 6
	default:
		return 7
	}
}

// ExitCodeConfigured applies CI relaxation without changing the recorded verdict.
func ExitCodeConfigured(value string, failOn []string) int {
	for _, configured := range failOn {
		if configured == value {
			return ExitCode(value)
		}
	}
	return 0
}

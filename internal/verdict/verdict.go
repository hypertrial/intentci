package verdict

import (
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
	ID       string             `json:"id"`
	Statement string            `json:"statement"`
	Required bool               `json:"required"`
	Verdict  string             `json:"verdict"`
	Reason   string             `json:"reason,omitempty"`
	Evidence []provider.Evidence `json:"evidence,omitempty"`
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

// EvaluateNode evaluates a verify expression given leaf results keyed by provider spec id.
func EvaluateNode(n ir.VerifyNode, leaves map[string]provider.Result) (string, string, []provider.Evidence) {
	if n.Provider != nil {
		key := n.Provider.ID
		if key == "" {
			key = n.Provider.Provider
		}
		res, ok := leaves[key]
		if !ok {
			return Unproven, "missing provider evidence", nil
		}
		return leafVerdict(res)
	}
	if len(n.All) > 0 {
		var ev []provider.Evidence
		worst := Pass
		reason := ""
		for _, c := range n.All {
			v, r, e := EvaluateNode(c, leaves)
			ev = append(ev, e...)
			worst, reason = worse(worst, reason, v, r)
		}
		return worst, reason, ev
	}
	if len(n.Any) > 0 {
		var ev []provider.Evidence
		best := Fail
		reason := "all alternatives failed"
		anyPass := false
		for _, c := range n.Any {
			v, r, e := EvaluateNode(c, leaves)
			ev = append(ev, e...)
			if v == Pass {
				anyPass = true
				best = Pass
				reason = r
			} else if !anyPass {
				best, reason = worse(best, reason, v, r)
			}
		}
		if anyPass {
			return Pass, reason, ev
		}
		return best, reason, ev
	}
	if n.Not != nil {
		v, r, e := EvaluateNode(*n.Not, leaves)
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

func leafVerdict(res provider.Result) (string, string, []provider.Evidence) {
	if res.Status == "error" {
		return Error, firstDiag(res), res.Evidence
	}
	if res.Status == "skipped" {
		return Skipped, "provider skipped", res.Evidence
	}
	for _, e := range res.Evidence {
		if e.Class == "manual" || (e.Data != nil && e.Data["review_required"] == true) {
			return ReviewRequired, e.Summary, res.Evidence
		}
		if e.Class == "probabilistic" {
			if e.Passed != nil && !*e.Passed {
				return Uncertain, e.Summary, res.Evidence
			}
			if e.Passed == nil {
				return Uncertain, e.Summary, res.Evidence
			}
		}
		if e.Passed != nil && !*e.Passed {
			return Fail, e.Summary, res.Evidence
		}
	}
	if len(res.Evidence) == 0 {
		return Unproven, "no evidence produced", nil
	}
	for _, e := range res.Evidence {
		if e.Passed == nil {
			return Unproven, "evidence missing pass/fail", res.Evidence
		}
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
func AggregateRequirement(r ir.Requirement, obs []ObligationResult) RequirementResult {
	out := RequirementResult{
		ID: r.ID, Title: r.Title, Priority: r.Priority, Obligations: obs, Verdict: Pass,
	}
	for _, o := range obs {
		if !o.Required && o.Verdict != Fail && o.Verdict != Error {
			continue
		}
		if rank(o.Verdict) > rank(out.Verdict) {
			out.Verdict = o.Verdict
			out.Reason = o.Reason
		}
	}
	if len(obs) == 0 {
		out.Verdict = Unproven
		out.Reason = "no obligations selected"
	}
	return out
}

// AggregateRun combines requirement verdicts. Only required priority blocks by default.
func AggregateRun(reqs []RequirementResult) RunResult {
	out := RunResult{Requirements: reqs, Verdict: Pass}
	for _, r := range reqs {
		if r.Priority != "required" {
			continue
		}
		if rank(r.Verdict) > rank(out.Verdict) {
			out.Verdict = r.Verdict
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

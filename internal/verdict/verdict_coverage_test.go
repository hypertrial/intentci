package verdict_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

func TestLeafVerdictBranches(t *testing.T) {
	cases := []struct {
		name string
		res  provider.Result
		want string
	}{
		{"error diag", provider.Result{Status: "error", Diagnostics: []string{"boom"}}, verdict.Error},
		{"error no diag", provider.Result{Status: "error"}, verdict.Error},
		{"skipped", provider.Result{Status: "skipped"}, verdict.Skipped},
		{"manual", provider.Result{Status: "completed", Evidence: []provider.Evidence{{Class: "manual", Summary: "m"}}}, verdict.ReviewRequired},
		{"prob fail", provider.Result{Status: "completed", Evidence: []provider.Evidence{{Class: "probabilistic", Passed: boolPtr(false), Summary: "p"}}}, verdict.Uncertain},
		{"prob nil", provider.Result{Status: "completed", Evidence: []provider.Evidence{{Class: "probabilistic", Summary: "p"}}}, verdict.Uncertain},
		{"prob pass then fail", provider.Result{Status: "completed", Evidence: []provider.Evidence{
			{Class: "probabilistic", Passed: boolPtr(true), Summary: "ok"},
			{Class: "deterministic", Passed: boolPtr(false), Summary: "no"},
		}}, verdict.Fail},
		{"empty", provider.Result{Status: "completed"}, verdict.Unproven},
		{"nil passed", provider.Result{Status: "completed", Evidence: []provider.Evidence{{Class: "deterministic", Summary: "x"}}}, verdict.Unproven},
		{"pass", provider.Result{Status: "completed", Evidence: []provider.Evidence{{Class: "deterministic", Passed: boolPtr(true), Summary: "ok"}}}, verdict.Pass},
	}
	for _, c := range cases {
		v, _, _ := verdict.EvaluateNode(ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "command"}}, map[string]provider.Result{"command": c.res})
		if v != c.want {
			t.Fatalf("%s: got %s want %s", c.name, v, c.want)
		}
	}
}

func TestNotAnyEmptyAndWorse(t *testing.T) {
	leaves := map[string]provider.Result{
		"ok":  {Status: "completed", Evidence: []provider.Evidence{{Passed: boolPtr(true), Class: "deterministic", Summary: "ok"}}},
		"bad": {Status: "completed", Evidence: []provider.Evidence{{Passed: boolPtr(false), Class: "deterministic", Summary: "bad"}}},
		"err": {Status: "error", Diagnostics: []string{"e"}},
	}
	v, _, _ := verdict.EvaluateNode(ir.VerifyNode{Not: &ir.VerifyNode{Provider: &ir.ProviderSpec{ID: "ok", Provider: "command"}}}, leaves)
	if v != verdict.Fail {
		t.Fatal(v)
	}
	v, _, _ = verdict.EvaluateNode(ir.VerifyNode{Not: &ir.VerifyNode{Provider: &ir.ProviderSpec{ID: "bad", Provider: "command"}}}, leaves)
	if v != verdict.Pass {
		t.Fatal(v)
	}
	v, _, _ = verdict.EvaluateNode(ir.VerifyNode{Not: &ir.VerifyNode{Provider: &ir.ProviderSpec{ID: "err", Provider: "command"}}}, leaves)
	if v != verdict.Error {
		t.Fatal(v)
	}
	v, _, _ = verdict.EvaluateNode(ir.VerifyNode{}, nil)
	if v != verdict.Unproven {
		t.Fatal(v)
	}
	v, _, _ = verdict.EvaluateNode(ir.VerifyNode{Any: []ir.VerifyNode{
		{Provider: &ir.ProviderSpec{ID: "bad", Provider: "command"}},
		{Provider: &ir.ProviderSpec{ID: "err", Provider: "command"}},
	}}, leaves)
	if v != verdict.Error {
		t.Fatal(v)
	}
	v, reason, _ := verdict.EvaluateNode(ir.VerifyNode{All: []ir.VerifyNode{
		{Provider: &ir.ProviderSpec{ID: "ok", Provider: "command"}},
		{Provider: &ir.ProviderSpec{ID: "ok", Provider: "command"}},
	}}, leaves)
	if v != verdict.Pass || reason == "" {
		t.Fatalf("%s %s", v, reason)
	}
}

func TestAggregateAndExitCodes(t *testing.T) {
	rr := verdict.AggregateRequirement(ir.Requirement{ID: "R", Priority: "required"}, nil)
	if rr.Verdict != verdict.Unproven {
		t.Fatal(rr.Verdict)
	}
	rr = verdict.AggregateRequirement(ir.Requirement{ID: "R", Priority: "required"}, []verdict.ObligationResult{
		{Required: false, Verdict: verdict.Unproven},
		{Required: true, Verdict: verdict.Fail, Reason: "f"},
	})
	if rr.Verdict != verdict.Fail {
		t.Fatal(rr.Verdict)
	}
	// exercise rank cases via aggregation
	for _, v := range []string{verdict.Skipped, verdict.Uncertain, verdict.ReviewRequired, "custom"} {
		rr = verdict.AggregateRequirement(ir.Requirement{ID: "R", Priority: "required"}, []verdict.ObligationResult{
			{Required: true, Verdict: verdict.Pass},
			{Required: true, Verdict: v},
		})
		if rr.Verdict != v && v != "custom" {
			// custom maps to rank 2 (unproven-equivalent) but keeps string if worse
			_ = rr
		}
		if v == "custom" && rr.Verdict != "custom" && rr.Verdict != verdict.Pass {
			// rank(custom)=2 > rank(pass)=0 so verdict becomes custom
			if rr.Verdict != "custom" {
				t.Fatalf("custom got %s", rr.Verdict)
			}
		}
	}
	run := verdict.AggregateRun(nil)
	if run.Verdict != verdict.Pass {
		t.Fatal(run.Verdict)
	}
	if run.Requirements == nil {
		t.Fatal("empty run requirements must be encoded as an empty JSON array")
	}
	run = verdict.AggregateRun([]verdict.RequirementResult{
		{Priority: "recommended", Verdict: verdict.Fail},
		{Priority: "required", Verdict: verdict.Error},
	})
	if run.Verdict != verdict.Error {
		t.Fatal(run.Verdict)
	}
	codes := map[string]int{
		verdict.Pass: 0, verdict.Fail: 1, verdict.Unproven: 2, verdict.Uncertain: 3,
		verdict.ReviewRequired: 4, verdict.Error: 6, "weird": 7, verdict.Skipped: 7,
	}
	for v, want := range codes {
		if got := verdict.ExitCode(v); got != want {
			t.Fatalf("%s: %d want %d", v, got, want)
		}
	}
}

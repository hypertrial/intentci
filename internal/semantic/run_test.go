package semantic_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/semantic"
)

type stubProvider struct {
	resp semantic.Response
	err  error
	calls int
}

func (s *stubProvider) Analyze(ctx context.Context, req semantic.Request) (semantic.Response, error) {
	s.calls++
	return s.resp, s.err
}

func TestRun_ShowSemanticInput(t *testing.T) {
	var buf bytes.Buffer
	stub := &stubProvider{}
	out, err := semantic.Run(context.Background(), semantic.RunOptions{
		Contract: &contract.Contract{
			Product: contract.Product{Name: "n", Purpose: "p"},
			Policy:  contract.Policy{Semantic: contract.SemanticPolicy{Enabled: true, Enforcement: "advisory", Provider: &contract.SemanticProvider{Type: "local", Command: "x"}}},
		},
		Selection:         impact.Selection{},
		ShowSemanticInput: true,
		Stdout:            &buf,
		Provider:          stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.ShowedInput || stub.calls != 0 {
		t.Fatalf("showed=%v calls=%d", out.ShowedInput, stub.calls)
	}
	if !strings.Contains(buf.String(), "protocol_version") {
		t.Fatalf("buf %s", buf.String())
	}
}

func TestRun_DisabledAndSuccess(t *testing.T) {
	out, err := semantic.Run(context.Background(), semantic.RunOptions{
		Contract: &contract.Contract{Product: contract.Product{Name: "n", Purpose: "p"}},
	})
	if err != nil || out.SemanticRun == nil || out.SemanticRun.Enabled {
		t.Fatalf("%v %+v", err, out.SemanticRun)
	}

	stub := &stubProvider{resp: semantic.Response{Findings: []semantic.Finding{{
		RequirementID: "R-1", Assessment: semantic.AssessmentAligned, Confidence: 1, Summary: "ok",
	}}}}
	out, err = semantic.Run(context.Background(), semantic.RunOptions{
		Contract: &contract.Contract{
			Product: contract.Product{Name: "n", Purpose: "p"},
			Policy: contract.Policy{Semantic: contract.SemanticPolicy{
				Enabled: true, Enforcement: "advisory",
				Provider: &contract.SemanticProvider{Type: "http", URL: "https://example.invalid"},
			}},
			Requirements: []contract.Requirement{{
				ID: "R-1", Status: "approved", Severity: "blocking",
				Verification: contract.Verification{Checks: []string{"c"}, Semantic: "optional"},
			}},
		},
		Selection: impact.Selection{Requirements: []impact.SelectedRequirement{{
			Requirement: contract.Requirement{
				ID: "R-1", Status: "approved",
				Verification: contract.Verification{Semantic: "optional"},
			},
		}}},
		Provider: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.SemanticRun.FindingCount != 1 || stub.calls != 1 {
		t.Fatalf("%+v calls=%d", out.SemanticRun, stub.calls)
	}
}

func TestRun_ProviderError(t *testing.T) {
	stub := &stubProvider{err: errors.New("down")}
	out, err := semantic.Run(context.Background(), semantic.RunOptions{
		Contract: &contract.Contract{
			Product: contract.Product{Name: "n", Purpose: "p"},
			Policy: contract.Policy{Semantic: contract.SemanticPolicy{
				Enabled: true, Enforcement: "blocking",
				Provider: &contract.SemanticProvider{Type: "local", Command: "x"},
			}},
		},
		Provider:   stub,
		TrustLocal: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ProviderErr == nil || out.SemanticRun.Skipped == "" {
		t.Fatalf("%+v", out)
	}
}

func TestModesFromSelection(t *testing.T) {
	m := semantic.ModesFromSelection(impact.Selection{Requirements: []impact.SelectedRequirement{
		{Requirement: contract.Requirement{ID: "A", Verification: contract.Verification{Semantic: "required"}}},
		{Requirement: contract.Requirement{ID: "B"}},
	}})
	if m["A"] != "required" || m["B"] != "optional" {
		t.Fatalf("%v", m)
	}
}

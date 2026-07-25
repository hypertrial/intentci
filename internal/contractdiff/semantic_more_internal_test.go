package contractdiff

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestStricterSemantic_Branches(t *testing.T) {
	thrBase := 0.9
	thrHead := 0.95
	base := contract.SemanticPolicy{
		Enabled: true, Enforcement: "advisory", ConfidenceThreshold: &thrBase,
		Provider: &contract.SemanticProvider{Type: "local", Command: "./p"},
	}
	head := contract.SemanticPolicy{
		Enabled: true, Enforcement: "", ConfidenceThreshold: &thrHead,
		Provider: &contract.SemanticProvider{Type: "http", URL: "https://x"},
	}
	out := stricterSemantic(base, head)
	if out.Enforcement != "advisory" {
		t.Fatalf("%+v", out)
	}
	// head already higher threshold — keep head
	if out.ConfidenceThresholdOrDefault() != 0.95 {
		t.Fatalf("%v", out.ConfidenceThresholdOrDefault())
	}
	// provider identity changed → restore base provider
	if out.Provider == nil || out.Provider.Type != "local" || out.Provider.Command != "./p" {
		t.Fatalf("%+v", out.Provider)
	}
	// base disabled → head unchanged (still enabled from head)
	out = stricterSemantic(contract.SemanticPolicy{}, head)
	if !out.Enabled || out.Provider.Type != "http" {
		t.Fatalf("expected head preserved: %+v", out)
	}
	if providerChanged(nil, head.Provider) || providerChanged(base.Provider, nil) {
		t.Fatal("nil sides are not a change")
	}
}

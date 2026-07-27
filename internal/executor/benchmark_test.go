package executor_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/executor"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
)

type benchmarkProvider struct{}

func (benchmarkProvider) Name() string    { return "benchmark" }
func (benchmarkProvider) Version() string { return "1.0.0" }
func (benchmarkProvider) Validate(ir.ProviderSpec) []provider.Diagnostic {
	return nil
}
func (benchmarkProvider) Execute(_ context.Context, request provider.Request) provider.Result {
	passed := true
	return provider.Result{
		Provider: "benchmark", ProviderVersion: "1.0.0", Status: "completed",
		Evidence: []provider.Evidence{{
			ID: request.Spec.ID, Class: "deterministic", Summary: "passed", Passed: &passed,
		}},
	}
}

func BenchmarkV1IncrementalSchedulingOverhead(b *testing.B) {
	requirements := make([]ir.Requirement, 100)
	for index := range requirements {
		requirements[index] = ir.Requirement{
			ID: fmt.Sprintf("REQ-%03d", index), Priority: "required",
			Obligations: []ir.Obligation{{
				ID: fmt.Sprintf("OBL-%03d", index), Required: true,
				Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
					Provider: "benchmark", ID: fmt.Sprintf("check-%03d", index),
				}},
			}},
		}
	}
	cfg := config.Default()
	cfg.Verification.MaxParallel = 4
	registry := provider.NewRegistry(benchmarkProvider{})
	root := b.TempDir()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		_, results := executor.Run(context.Background(), requirements, executor.Options{
			Root: root, Config: cfg, Registry: registry, NoCache: true,
			RunID: fmt.Sprintf("run-%d", iteration), AttemptID: "attempt-001",
		})
		if len(results) != len(requirements) {
			b.Fatalf("scheduled %d requirements", len(results))
		}
	}
}

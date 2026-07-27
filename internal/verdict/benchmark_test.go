package verdict_test

import (
	"fmt"
	"testing"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/verdict"
)

func BenchmarkV1Aggregate10000TestCases(b *testing.B) {
	obligations := make([]verdict.ObligationResult, 10_000)
	for index := range obligations {
		obligations[index] = verdict.ObligationResult{
			ID: fmt.Sprintf("OBL-%05d", index), Required: true, Verdict: verdict.Pass,
		}
	}
	requirement := ir.Requirement{ID: "REQ-BENCHMARK", Priority: "required"}
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		result := verdict.AggregateRequirement(requirement, obligations)
		if result.Verdict != verdict.Pass {
			b.Fatal(result.Verdict)
		}
	}
}

package impact_test

import (
	"fmt"
	"testing"

	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/ir"
)

func BenchmarkV1ChangeImpact10000Files(b *testing.B) {
	document := &ir.Document{Requirements: make([]ir.Requirement, 100)}
	for index := range document.Requirements {
		document.Requirements[index] = ir.Requirement{
			ID: fmt.Sprintf("REQ-%03d", index), Status: "active",
			AppliesTo: ir.AppliesTo{Paths: []string{fmt.Sprintf("src/%03d/**", index)}},
		}
	}
	files := make([]string, 10_000)
	for index := range files {
		files[index] = fmt.Sprintf("src/%03d/file-%05d.go", index%100, index)
	}
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		selection := impact.Select(document, impact.Options{ChangedFiles: files})
		if len(selection.Requirements) != 100 {
			b.Fatalf("selected %d requirements", len(selection.Requirements))
		}
	}
}

package impact_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/ir"
)

func TestSelectChanged(t *testing.T) {
	doc := &ir.Document{Requirements: []ir.Requirement{
		{ID: "REQ-1", Status: "active", AppliesTo: ir.AppliesTo{Paths: []string{"src/**"}}, Obligations: []ir.Obligation{{ID: "O"}}},
		{ID: "REQ-2", Status: "active", AppliesTo: ir.AppliesTo{Paths: []string{"docs/**"}}, Obligations: []ir.Obligation{{ID: "O"}}},
		{ID: "REQ-3", Status: "draft", AppliesTo: ir.AppliesTo{Paths: []string{"**"}}, Obligations: []ir.Obligation{{ID: "O"}}},
	}}
	sel := impact.Select(doc, impact.Options{ChangedFiles: []string{"src/a.go"}})
	if len(sel.Requirements) != 1 || sel.Requirements[0].ID != "REQ-1" {
		t.Fatalf("%+v", sel)
	}
	if !impact.PathMatches([]string{"src/**"}, "src/a.go") {
		t.Fatal("path match")
	}
	all := impact.Select(doc, impact.Options{All: true})
	if len(all.Requirements) != 2 {
		t.Fatalf("%d", len(all.Requirements))
	}
}

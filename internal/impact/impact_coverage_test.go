package impact_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/ir"
)

func TestSelectRequirementObligationDepsUnmapped(t *testing.T) {
	doc := &ir.Document{Requirements: []ir.Requirement{
		{ID: "REQ-A", Status: "active", AppliesTo: ir.AppliesTo{Paths: []string{"src/**"}},
			Obligations: []ir.Obligation{{ID: "O1"}, {ID: "O2"}}, Boundaries: ir.Boundaries{Allowed: []string{"src/**"}}},
		{ID: "REQ-B", Status: "active", DependsOn: []string{"REQ-A"}, AppliesTo: ir.AppliesTo{Paths: []string{"pkg/**"}},
			Obligations: []ir.Obligation{{ID: "O1"}}},
		{ID: "REQ-C", Status: "active", AppliesTo: ir.AppliesTo{}, // any change
			Obligations: []ir.Obligation{{ID: "O1"}}},
		{ID: "REQ-D", Status: "active", DependsOn: []string{"REQ-B"}, AppliesTo: ir.AppliesTo{Paths: []string{"other/**"}},
			Obligations: []ir.Obligation{{ID: "O1"}}},
	}}

	sel := impact.Select(doc, impact.Options{RequirementID: "REQ-A", ObligationID: "O2"})
	if len(sel.Requirements) != 1 || len(sel.Requirements[0].Obligations) != 1 || sel.Requirements[0].Obligations[0].ID != "O2" {
		t.Fatalf("%+v", sel)
	}

	sel = impact.Select(doc, impact.Options{All: true, ObligationID: "O1"})
	if len(sel.Requirements) == 0 {
		t.Fatal("expected all")
	}
	for _, r := range sel.Requirements {
		if len(r.Obligations) != 1 || r.Obligations[0].ID != "O1" {
			t.Fatalf("%+v", r)
		}
	}

	sel = impact.Select(doc, impact.Options{ChangedFiles: []string{"src/a.go", "unmapped.txt"}})
	if len(sel.Unmapped) == 0 {
		t.Fatalf("expected unmapped %+v", sel)
	}
	ids := map[string]bool{}
	for _, r := range sel.Requirements {
		ids[r.ID] = true
	}
	if !ids["REQ-A"] || !ids["REQ-B"] || !ids["REQ-C"] || !ids["REQ-D"] {
		t.Fatalf("deps closure %+v", ids)
	}

	sel = impact.Select(doc, impact.Options{ChangedFiles: []string{"docs/x.md"}, ObligationID: "O1"})
	// REQ-C has empty applies_to so matches any change
	if len(sel.Requirements) == 0 {
		t.Fatal("expected REQ-C")
	}

	sel = impact.Select(doc, impact.Options{ChangedFiles: []string{"pkg/x.go"}})
	if len(sel.Requirements) == 0 {
		t.Fatal("expected match")
	}
}

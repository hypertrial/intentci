package impact_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/ir"
)

func TestV1ExplicitDependencyClosures(t *testing.T) {
	document := &ir.Document{Requirements: []ir.Requirement{
		{
			ID: "A", Status: "active", DependsOn: []string{"B"},
			Obligations: []ir.Obligation{
				{ID: "one", DependsOn: []string{"two"}},
				{ID: "two", DependsOn: []string{"three"}},
				{ID: "three"},
			},
		},
		{ID: "B", Status: "active", DependsOn: []string{"C"}, Obligations: []ir.Obligation{{ID: "one"}}},
		{ID: "C", Status: "active", Obligations: []ir.Obligation{{ID: "one"}}},
	}}
	selected := impact.Select(document, impact.Options{RequirementID: "A", ObligationID: "one"})
	if len(selected.Requirements) != 3 || len(selected.Requirements[0].Obligations) != 3 {
		t.Fatalf("%+v", selected)
	}
}

func TestV1GlobalInputsAndUnmappedSelection(t *testing.T) {
	nested := ir.VerifyNode{
		All: []ir.VerifyNode{{Provider: &ir.ProviderSpec{Provider: "command", Inputs: []string{"all/**"}}}},
		Any: []ir.VerifyNode{{Provider: &ir.ProviderSpec{Provider: "command", Inputs: []string{"any/**"}}}},
		Not: &ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "command", Inputs: []string{"not/**"}}},
	}
	document := &ir.Document{Requirements: []ir.Requirement{
		{
			ID: "mapped", Status: "active", SourcePath: "requirements/mapped.md",
			AppliesTo:   ir.AppliesTo{Paths: []string{"src/**"}},
			Obligations: []ir.Obligation{{ID: "O", Verify: nested}},
		},
		{ID: "global", Status: "active", Obligations: []ir.Obligation{{ID: "O"}}},
	}}
	for _, changed := range []string{"all/x", "any/x", "not/x", "requirements/mapped.md"} {
		selection := impact.Select(document, impact.Options{ChangedFiles: []string{changed}})
		if len(selection.Requirements) == 0 {
			t.Fatalf("%s was not mapped: %+v", changed, selection)
		}
	}
	global := impact.Select(document, impact.Options{
		ChangedFiles: []string{"config/global.yaml"}, GlobalPaths: []string{"config/**"},
	})
	if len(global.Requirements) != 2 || len(global.Unmapped) != 0 {
		t.Fatalf("%+v", global)
	}
	unmapped := impact.Select(document, impact.Options{
		ChangedFiles: []string{"other/file"}, RunUnmappedRequirements: true,
	})
	if len(unmapped.Unmapped) != 1 || len(unmapped.Requirements) != 1 ||
		unmapped.Requirements[0].ID != "global" {
		t.Fatalf("%+v", unmapped)
	}
	if impact.PathMatches([]string{"[", "src/**"}, "src/file") != true ||
		impact.PathMatches([]string{"["}, "src/file") {
		t.Fatal("path matching")
	}
}

package ir_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/ir"
)

func TestComputeHashesAndLookup(t *testing.T) {
	doc := &ir.Document{
		SchemaVersion: 1,
		Project:       "p",
		Requirements: []ir.Requirement{
			{ID: "REQ-2", Title: "b", Status: "active", Obligations: []ir.Obligation{{ID: "O"}}},
			{ID: "REQ-1", Title: "a", Status: "draft", Obligations: []ir.Obligation{{ID: "O"}}},
		},
	}
	if err := doc.ComputeHashes(); err != nil {
		t.Fatal(err)
	}
	if doc.Hash == "" || doc.Requirements[0].ID != "REQ-1" {
		t.Fatalf("%+v", doc)
	}
	if doc.RequirementByID("REQ-2") == nil {
		t.Fatal("missing")
	}
	if len(doc.ActiveRequirements()) != 1 {
		t.Fatal(len(doc.ActiveRequirements()))
	}
}

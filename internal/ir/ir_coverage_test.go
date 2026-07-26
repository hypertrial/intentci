package ir_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/ir"
)

func TestRequirementByIDMissingAndCanonical(t *testing.T) {
	doc := &ir.Document{SchemaVersion: 1, Project: "p"}
	if doc.RequirementByID("x") != nil {
		t.Fatal("expected nil")
	}
	b, err := ir.CanonicalJSON(map[string]any{"a": 1})
	if err != nil || len(b) == 0 {
		t.Fatal(err)
	}
	if ir.HashBytes(b) == "" {
		t.Fatal("hash")
	}
	doc.Requirements = []ir.Requirement{{
		ID: "REQ-1", Status: "active",
		Obligations: []ir.Obligation{{ID: "O", Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "command", Run: "true"}}}},
	}}
	if err := doc.ComputeHashes(); err != nil {
		t.Fatal(err)
	}
	if doc.Requirements[0].Hash == "" || doc.Requirements[0].Obligations[0].Hash == "" {
		t.Fatal("hashes")
	}
}

func TestComputeHashesErrors(t *testing.T) {
	doc := &ir.Document{SchemaVersion: 1, Project: "p", Requirements: []ir.Requirement{{
		ID: "R", Obligations: []ir.Obligation{{ID: "O", Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
			Extra: map[string]any{"c": make(chan int)},
		}}}},
	}}}
	if err := doc.ComputeHashes(); err == nil {
		t.Fatal("obl hash")
	}
	doc = &ir.Document{SchemaVersion: 1, Project: "p", Requirements: []ir.Requirement{{
		ID: "R", AppliesTo: ir.AppliesTo{}, Constraints: nil,
		// put chan on requirement via Boundaries? can't. Use Tags? no.
		// Obligation ok, but requirement-level: Owners is []string.
		// Provider Extra on obligation fails first.
	}}}
	doc = &ir.Document{SchemaVersion: 1, Project: "p", Requirements: []ir.Requirement{{
		ID: "R", Obligations: []ir.Obligation{{ID: "O"}},
	}}}
	// document-level: after obl+req hash, document marshal - need chan at doc level
	// Document only has Requirements - if requirement has something unmarshalable after obl hashed...
	// Actually after obl hash succeeds, req clone includes Obligations with Hash set - still has Extra chan
	doc = &ir.Document{SchemaVersion: 1, Project: "p", Requirements: []ir.Requirement{{
		ID: "R", Obligations: []ir.Obligation{{ID: "O", Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
			Extra: map[string]any{"c": make(chan int)},
		}}}},
	}}}
	if err := doc.ComputeHashes(); err == nil {
		t.Fatal("expected error")
	}
}

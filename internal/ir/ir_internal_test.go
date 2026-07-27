package ir

import (
	"errors"
	"testing"
)

func TestComputeHashesMarshalStages(t *testing.T) {
	doc := &Document{SchemaVersion: 1, Project: "p", Requirements: []Requirement{{
		ID: "R", Obligations: []Obligation{{ID: "O"}},
	}}}
	old := jsonMarshal
	defer func() { jsonMarshal = old }()
	n := 0
	jsonMarshal = func(v any) ([]byte, error) {
		n++
		if n == 1 {
			return nil, errors.New("obl")
		}
		return old(v)
	}
	if err := doc.ComputeHashes(); err == nil {
		t.Fatal("obl")
	}
	n = 0
	jsonMarshal = func(v any) ([]byte, error) {
		n++
		if n == 2 {
			return nil, errors.New("req")
		}
		return old(v)
	}
	if err := doc.ComputeHashes(); err == nil {
		t.Fatal("req")
	}
	n = 0
	jsonMarshal = func(v any) ([]byte, error) {
		n++
		if n == 3 {
			return nil, errors.New("doc")
		}
		return old(v)
	}
	if err := doc.ComputeHashes(); err == nil {
		t.Fatal("doc")
	}
}

func TestBuildVerificationPlanNormalizesAndHashes(t *testing.T) {
	document := &Document{SchemaVersion: 1, Project: "p", Hash: "ir", Requirements: []Requirement{
		{
			ID: "REQ-2", Hash: "r2",
			Obligations: []Obligation{{
				ID: "O2", Hash: "o2",
				Verify: VerifyNode{
					All: []VerifyNode{{Provider: &ProviderSpec{Provider: "command"}}},
					Any: []VerifyNode{{Provider: &ProviderSpec{Provider: "json", ID: "named"}}},
					Not: &VerifyNode{Provider: &ProviderSpec{Provider: "manual"}},
				},
			}},
		},
		{ID: "REQ-1", Hash: "r1"},
	}}
	plan, err := BuildVerificationPlan(document, document.Requirements)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash == "" || plan.Requirements[0].ID != "REQ-1" {
		t.Fatalf("%+v", plan)
	}
	node := plan.Requirements[1].Obligations[0].Verify
	if node.All[0].Provider.ID != "command#1" || node.Any[0].Provider.ID != "named" ||
		node.Not.Provider.ID != "manual#2" {
		t.Fatalf("%+v", node)
	}

	old := jsonMarshal
	jsonMarshal = func(any) ([]byte, error) { return nil, errors.New("plan") }
	defer func() { jsonMarshal = old }()
	if _, err := BuildVerificationPlan(document, document.Requirements); err == nil {
		t.Fatal("plan marshal error ignored")
	}
}

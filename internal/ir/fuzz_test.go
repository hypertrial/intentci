package ir

import (
	"reflect"
	"testing"
)

func FuzzV1LogicalExpressionNormalization(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3})
	f.Add([]byte{3, 3, 0})
	f.Fuzz(func(t *testing.T, shape []byte) {
		if len(shape) > 64 {
			t.Skip()
		}
		nodes := make([]VerifyNode, 0, len(shape))
		for _, value := range shape {
			provider := &ProviderSpec{Provider: "command"}
			if value%3 == 0 {
				provider.ID = "explicit"
			}
			nodes = append(nodes, VerifyNode{Provider: provider})
		}
		if len(nodes) == 0 {
			nodes = append(nodes, VerifyNode{Provider: &ProviderSpec{Provider: "command"}})
		}
		document := &Document{SchemaVersion: SchemaVersion, Hash: "document"}
		requirements := []Requirement{{
			ID: "R", Hash: "requirement",
			Obligations: []Obligation{{
				ID: "O", Hash: "obligation", Verify: VerifyNode{All: nodes},
			}},
		}}
		first, err := BuildVerificationPlan(document, requirements)
		if err != nil {
			t.Fatal(err)
		}
		second, err := BuildVerificationPlan(document, requirements)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("logical normalization is nondeterministic:\n%+v\n%+v", first, second)
		}
	})
}

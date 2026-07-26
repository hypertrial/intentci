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

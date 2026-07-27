package parser

import (
	"strings"
	"testing"
)

func TestV1DiagnosticLocations(t *testing.T) {
	diagnostic := Diagnostic{Path: "r.md", Line: 2, Column: 3, Message: "bad"}
	if diagnostic.Error() != "r.md:2:3: bad" {
		t.Fatal(diagnostic.Error())
	}
	located := locateDiagnostics("first\nsecond", []Diagnostic{{Line: 7, Message: "known"}})
	if located[0].Line != 7 {
		t.Fatal(located)
	}
}

func TestV1VerifyConversionEdges(t *testing.T) {
	for _, expression := range []map[string]any{
		{"all": []any{}},
		{"any": []any{}},
	} {
		if _, err := mapToVerify(expression); err == nil {
			t.Fatal(expression)
		}
	}
	nodes, err := toNodeList([]any{map[string]any{
		"all": []any{map[string]any{"provider": "command", "run": "true"}},
	}})
	if err != nil || len(nodes) != 1 || len(nodes[0].All) != 1 {
		t.Fatalf("%+v %v", nodes, err)
	}
}

func TestV1ProviderConversionEdges(t *testing.T) {
	invalid := []map[string]any{
		{},
		{"provider": "command", "run": []any{"bad"}},
		{"provider": "command", "result": "bad"},
		{"provider": "command", "inputs": "bad"},
		{"provider": "command", "inputs": []any{map[string]any{"bad": true}}},
		{"provider": "command", "environment": "bad"},
		{"provider": "command", "environment": map[string]any{"X": []any{"bad"}}},
		{"provider": "command", "retry": map[string]any{"unknown": true}},
		{"provider": "command", "exclusive": "true"},
		{"provider": "command", "unknown": true},
	}
	for _, values := range invalid {
		if _, err := toProvider(values); err == nil {
			t.Fatalf("invalid provider accepted: %#v", values)
		}
	}

	full := map[string]any{
		"provider": "command", "id": 1, "run": true, "report": "r",
		"prompt": "p", "working_directory": ".", "timeout": "1s", "evidence_class": "deterministic",
		"result": map[string]any{}, "expect": map[string]any{}, "assert": map[string]any{},
		"match": map[string]any{}, "allow": map[string]any{}, "configuration": map[string]any{},
		"inherit_environment": []any{"PATH"}, "allowed": []any{"a"}, "forbidden": []any{"b"},
		"paths": []any{"c"}, "inputs": []any{"d"}, "outputs": []any{"e"},
		"artifacts": []any{"f"}, "depends_on": []any{"other"},
		"environment": map[string]any{"BOOL": true, "COUNT": 1},
		"retry":       map[string]any{"attempts": 2, "backoff": "1s"},
		"exclusive":   true,
	}
	spec, err := toProvider(full)
	if err != nil || spec.ID != "1" || spec.Run != "true" || !spec.Exclusive ||
		spec.Retry.Attempts != 2 || spec.Environment["BOOL"] != "true" {
		t.Fatalf("%+v %v", spec, err)
	}

	if _, err := requiredStringField(map[string]any{}, "provider"); err == nil {
		t.Fatal("missing required string")
	}
	if err := optionalStringField(map[string]any{}, "x", new(string)); err != nil {
		t.Fatal(err)
	}
	if err := optionalMapField(map[string]any{}, "x", new(map[string]any)); err != nil {
		t.Fatal(err)
	}
	if err := optionalStringsField(map[string]any{}, "x", new([]string)); err != nil {
		t.Fatal(err)
	}
}

func TestV1ScalarMapAndRetryEdges(t *testing.T) {
	values, err := stringMap(map[string]any{"A": "x", "B": int64(2), "C": float64(3)})
	if err != nil || strings.Join([]string{values["A"], values["B"], values["C"]}, "") != "x23" {
		t.Fatalf("%v %v", values, err)
	}
	if isScalar(struct{}{}) {
		t.Fatal("struct treated as scalar")
	}
	if _, err := retryValue("invalid"); err == nil {
		t.Fatal("scalar retry accepted")
	}
}

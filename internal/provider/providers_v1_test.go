package provider_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
)

func TestJSONPathSubsetAndValidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "data.json")
	if err := os.WriteFile(path, []byte(`{"items":[{"value":10}],"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	jsonProvider := &provider.JSONProvider{}
	for _, assertion := range []map[string]any{
		{"path": "$.items[0].value", "equals": 10},
		{"path": "$.items[0].value", "not_equals": 11},
		{"path": "$.items[0].value", "gt": 9},
		{"path": "$.items[0].value", "gte": 10},
		{"path": "$.items[0].value", "lt": 11},
		{"path": "$.items[0].value", "lte": 10},
		{"path": "$.missing", "exists": false},
		{"name": "demo"},
	} {
		spec := ir.ProviderSpec{ID: "json", Report: "data.json", Assert: assertion}
		if diagnostics := jsonProvider.Validate(spec); len(diagnostics) != 0 {
			t.Fatalf("%v: %v", assertion, diagnostics)
		}
		result := jsonProvider.Execute(context.Background(), provider.Request{Root: root, Spec: spec})
		if result.Status != "completed" || result.Evidence[0].Passed == nil || !*result.Evidence[0].Passed {
			t.Fatalf("%v: %+v", assertion, result)
		}
	}
	for _, assertion := range []map[string]any{
		{"path": "items", "equals": 1},
		{"path": "$..items", "equals": 1},
		{"path": "$.items[-1]", "equals": 1},
		{"path": "$.name", "gt": 1},
		{"path": "$.name", "exists": "yes"},
		{"path": "$.name", "equals": "demo", "not_equals": "x"},
	} {
		spec := ir.ProviderSpec{Report: "data.json", Assert: assertion}
		diagnostics := jsonProvider.Validate(spec)
		result := jsonProvider.Execute(context.Background(), provider.Request{Root: root, Spec: spec})
		if len(diagnostics) == 0 && result.Status != "error" {
			t.Fatalf("invalid assertion passed: %v %+v", assertion, result)
		}
	}
	for _, spec := range []ir.ProviderSpec{
		{Report: "data.json"},
		{Report: "../outside.json", Assert: map[string]any{"x": true}},
		{Report: "missing.json", Assert: map[string]any{"x": true}},
	} {
		result := jsonProvider.Execute(context.Background(), provider.Request{Root: root, Spec: spec})
		if result.Status != "error" && len(jsonProvider.Validate(spec)) == 0 {
			t.Fatalf("%+v", result)
		}
	}
}

func TestJUnitDetailsAndSARIFSubset(t *testing.T) {
	root := t.TempDir()
	junit := `<testsuite name="suite" tests="3" failures="1" errors="1">
<testcase name="failure" time="0.1"><failure message="bad"/></testcase>
<testcase name="error" time="0.2"><error>boom</error></testcase>
<testcase name="skip"><skipped message="later"/></testcase>
</testsuite>`
	if err := os.WriteFile(filepath.Join(root, "junit.xml"), []byte(junit), 0o644); err != nil {
		t.Fatal(err)
	}
	result := (&provider.JUnitProvider{}).Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: "junit.xml"},
	})
	if result.Status != "completed" || *result.Evidence[0].Passed {
		t.Fatalf("%+v", result)
	}
	data := result.Evidence[0].Data
	if data["skipped"] != 1 || len(data["failure_messages"].([]string)) != 2 {
		t.Fatalf("%v", data)
	}
	if err := os.WriteFile(filepath.Join(root, "empty.xml"), []byte(`<testsuite/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := (&provider.JUnitProvider{}).Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: "empty.xml"},
	})
	if empty.Evidence[0].Passed != nil {
		t.Fatal(empty)
	}
	if err := os.WriteFile(filepath.Join(root, "wrong.xml"), []byte(`<other/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	wrong := (&provider.JUnitProvider{}).Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: "wrong.xml"},
	})
	if wrong.Status != "error" {
		t.Fatal(wrong)
	}

	sarif := `{
  "version":"2.1.0",
  "runs":[{"results":[
    {"ruleId":"R1","level":"error","baselineState":"new","properties":{"security-severity":"9"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"src/a.go"}}}]},
    {"ruleId":"R2","level":"warning","baselineState":"unchanged","locations":[]}
  ]}]
}`
	if err := os.WriteFile(filepath.Join(root, "result.sarif"), []byte(sarif), 0o644); err != nil {
		t.Fatal(err)
	}
	sarifProvider := &provider.SARIFProvider{}
	spec := ir.ProviderSpec{
		Report: "result.sarif",
		Match: map[string]any{
			"rule_id": "R1", "result_level": "error", "baseline_state": "new",
			"severity": "9", "path": "src/**",
		},
		Allow: map[string]any{"max_findings": 1, "levels": []any{"error"}},
	}
	if diagnostics := sarifProvider.Validate(spec); len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	sarifResult := sarifProvider.Execute(context.Background(), provider.Request{Root: root, Spec: spec})
	if sarifResult.Status != "completed" || !*sarifResult.Evidence[0].Passed {
		t.Fatalf("%+v", sarifResult)
	}
	for _, invalid := range []ir.ProviderSpec{
		{Report: "x", Match: map[string]any{"unknown": true}},
		{Report: "x", Allow: map[string]any{"unknown": true}},
		{Report: "x", Allow: map[string]any{"max_findings": -1}},
		{Report: "x", Allow: map[string]any{"levels": []any{}}},
	} {
		if len(sarifProvider.Validate(invalid)) == 0 {
			t.Fatalf("invalid SARIF spec passed: %+v", invalid)
		}
	}
}

func TestCommandMatchersAndValidation(t *testing.T) {
	command := &provider.CommandProvider{}
	for _, spec := range []ir.ProviderSpec{
		{Run: "true", Result: map[string]any{"unknown": true}},
		{Run: "true", Result: map[string]any{"type": "signal"}},
		{Run: "true", Result: map[string]any{"equals": 1.5}},
		{Run: "true", Result: map[string]any{"equals": "zero"}},
		{Run: "true", Result: map[string]any{"stdout": "x"}},
		{Run: "true", Result: map[string]any{"stdout": map[string]any{"unknown": "x"}}},
		{Run: "true", Result: map[string]any{"stderr": map[string]any{"matches": "["}}},
	} {
		if len(command.Validate(spec)) == 0 {
			t.Fatalf("invalid command spec passed: %+v", spec)
		}
	}
	valid := ir.ProviderSpec{
		Run: `printf hello; printf problem >&2`,
		Result: map[string]any{
			"type": "exit_code", "equals": 0,
			"stdout": map[string]any{"equals": "hello", "contains": "ell", "matches": "^h"},
			"stderr": map[string]any{"contains": "problem"},
		},
	}
	if diagnostics := command.Validate(valid); len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	result := command.Execute(context.Background(), provider.Request{
		Root: t.TempDir(), RetainStdout: true, RetainStderr: true, Spec: valid,
	})
	if result.Status != "completed" || !*result.Evidence[0].Passed {
		t.Fatal(result)
	}
	for _, matcher := range []map[string]any{
		{"equals": "different"}, {"contains": "missing"}, {"matches": "^z"},
	} {
		spec := valid
		spec.Result = map[string]any{"equals": 0, "stdout": matcher}
		result := command.Execute(context.Background(), provider.Request{Root: t.TempDir(), Spec: spec})
		if *result.Evidence[0].Passed {
			t.Fatalf("%v: %+v", matcher, result)
		}
	}
}

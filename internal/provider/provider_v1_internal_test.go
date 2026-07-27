package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/ir"
)

func TestV1CommandAndExternalEdges(t *testing.T) {
	command := (&CommandProvider{}).Execute(context.Background(), Request{
		Root: t.TempDir(),
		Spec: ir.ProviderSpec{
			Run: `printf bad >&2`,
			Result: map[string]any{
				"equals": 0, "stderr": map[string]any{"equals": "good"},
			},
		},
	})
	if *command.Evidence[0].Passed {
		t.Fatal(command)
	}
	if ok, _ := matchOutput("stdout", "", "not-a-map"); ok {
		t.Fatal("scalar output expectation accepted")
	}

	external := (&ExternalProvider{ProviderName: "custom", Path: "/bin/true"}).Execute(
		context.Background(),
		Request{
			Root: t.TempDir(), Timeout: time.Second,
			Spec: ir.ProviderSpec{
				WorkingDirectory: ".",
				Configuration:    map[string]any{"unsupported": make(chan int)},
			},
		},
	)
	if external.Status != "error" || !strings.Contains(external.Diagnostics[0], "unsupported") {
		t.Fatal(external)
	}
	if validProviderName("") || validProviderName("Upper") || !validProviderName("lower-1") {
		t.Fatal("provider name validation")
	}
}

func TestV1GitDiffExpectations(t *testing.T) {
	changes := []Change{
		{Path: "renamed.go", Status: "renamed", Additions: 5, Deletions: 2},
		{Path: "deleted.go", Status: "deleted", Deletions: 4},
		{Path: "binary.bin", Status: "modified", Binary: true},
	}
	if got := changeForPath(changes, "renamed.go"); got.Status != "renamed" {
		t.Fatal(got)
	}
	if got := changeForPath(changes, "missing.go"); got.Status != "modified" {
		t.Fatal(got)
	}
	for _, testCase := range []struct {
		expect map[string]any
		pass   bool
	}{
		{nil, true},
		{map[string]any{"status": "renamed"}, false},
		{map[string]any{"status": []string{"renamed", "deleted", "modified"}}, true},
		{map[string]any{"renamed": true, "deleted": true, "binary": true}, true},
		{map[string]any{"renamed": false}, false},
		{map[string]any{"max_additions": 4}, false},
		{map[string]any{"max_deletions": 5}, false},
	} {
		pass, _ := evaluateChangeExpectations(testCase.expect, changes)
		if pass != testCase.pass {
			t.Fatalf("%v: %t", testCase.expect, pass)
		}
	}
	result := (&GitDiffProvider{}).Execute(context.Background(), Request{
		ChangedFiles: []string{"renamed.go"}, Changes: changes,
		Spec: ir.ProviderSpec{
			Paths: []string{"*.go"}, Expect: map[string]any{"changed": true, "renamed": false},
		},
	})
	if *result.Evidence[0].Passed || !strings.Contains(result.Evidence[0].Summary, "renamed") {
		t.Fatal(result)
	}
}

func TestV1JSONHelperEdges(t *testing.T) {
	if err := validateJSONAssert(nil); err != nil {
		t.Fatal(err)
	}
	if pass, _, err := evaluateJSONAssert(map[string]any{"x": 1}, map[string]any{"x": 2}); err != nil || pass {
		t.Fatalf("pass=%t err=%v", pass, err)
	}
	if pass, _, err := evaluateJSONAssert(map[string]any{}, map[string]any{"path": "$.missing", "equals": 1}); err != nil || pass {
		t.Fatalf("pass=%t err=%v", pass, err)
	}
	for _, expression := range []string{"$[", "$[x]", "$x"} {
		if _, _, err := jsonPath(nil, expression, false); err == nil {
			t.Fatal(expression)
		}
	}
	if _, exists, err := jsonPath(map[string]any{"x": "not-array"}, "$.x[0]", false); err != nil || exists {
		t.Fatalf("exists=%t err=%v", exists, err)
	}
	if _, exists, err := jsonPath(map[string]any{"x": []any{}}, "$.x[0]", false); err != nil || exists {
		t.Fatalf("exists=%t err=%v", exists, err)
	}
	if _, exists := lookupMember("not-map", "x"); exists {
		t.Fatal("member found in scalar")
	}
	if _, err := compareJSON(1, 1, "unsupported"); err == nil {
		t.Fatal("unsupported comparison")
	}
	for _, value := range []any{float32(1), int64(1)} {
		if number, ok := number(value); !ok || number != 1 {
			t.Fatalf("%T %v", value, value)
		}
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := (&JSONProvider{}).Execute(context.Background(), Request{
		Root: root, Spec: ir.ProviderSpec{Report: "bad.json", Assert: map[string]any{"x": 1}},
	})
	if result.Status != "error" {
		t.Fatal(result)
	}
}

func TestV1JUnitExecutionEdges(t *testing.T) {
	root := t.TempDir()
	provider := &JUnitProvider{}
	if result := provider.Execute(context.Background(), Request{
		Root: root, Spec: ir.ProviderSpec{Report: "../escape"},
	}); !result.SecurityViolation {
		t.Fatal(result)
	}
	nonempty := filepath.Join(root, "nonempty")
	if err := os.Mkdir(nonempty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nonempty, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if result := provider.Execute(context.Background(), Request{
		Root: root, Spec: ir.ProviderSpec{Report: "nonempty", Run: "true"},
	}); result.Status != "error" || !strings.Contains(result.Diagnostics[0], "remove stale") {
		t.Fatal(result)
	}
	timed := provider.Execute(context.Background(), Request{
		Root: root, Timeout: time.Millisecond, RetainStderr: true,
		Spec: ir.ProviderSpec{Report: "timed.xml", Run: "sleep 1"},
	})
	if timed.Status != "error" || !strings.Contains(timed.Diagnostics[0], "timed out") {
		t.Fatal(timed)
	}
	allSkipped := `<testsuite tests="1"><testcase><skipped/></testcase></testsuite>`
	if err := os.WriteFile(filepath.Join(root, "skipped.xml"), []byte(allSkipped), 0o644); err != nil {
		t.Fatal(err)
	}
	skipped := provider.Execute(context.Background(), Request{
		Root: root, Spec: ir.ProviderSpec{Report: "skipped.xml"},
	})
	if skipped.Evidence[0].Passed != nil {
		t.Fatal(skipped)
	}
	failures, total, err := parseJUnit([]byte(`<testsuite><testcase><failure>x</failure><error>y</error></testcase></testsuite>`))
	if err != nil || failures != 2 || total != 1 {
		t.Fatalf("%d/%d %v", failures, total, err)
	}
}

func TestV1SARIFExecutionAndMatchingEdges(t *testing.T) {
	root := t.TempDir()
	provider := &SARIFProvider{}
	if result := provider.Execute(context.Background(), Request{
		Root: root, Spec: ir.ProviderSpec{Report: "../escape"},
	}); !result.SecurityViolation {
		t.Fatal(result)
	}
	nonempty := filepath.Join(root, "nonempty")
	if err := os.Mkdir(nonempty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nonempty, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if result := provider.Execute(context.Background(), Request{
		Root: root, Spec: ir.ProviderSpec{Report: "nonempty", Run: "true"},
	}); result.Status != "error" || !strings.Contains(result.Diagnostics[0], "remove stale") {
		t.Fatal(result)
	}
	timed := provider.Execute(context.Background(), Request{
		Root: root, Timeout: time.Millisecond, RetainStderr: true,
		Spec: ir.ProviderSpec{Report: "timed.sarif", Run: "sleep 1"},
	})
	if timed.Status != "error" || !strings.Contains(timed.Diagnostics[0], "timed out") {
		t.Fatal(timed)
	}
	if count, err := countSARIF([]byte(`{"runs":[{"results":[{},{}]}]}`)); err != nil || count != 2 {
		t.Fatalf("%d %v", count, err)
	}
	if _, err := countSARIF([]byte("{")); err == nil {
		t.Fatal("malformed SARIF counted")
	}
	if _, _, err := evaluateSARIF([]byte(`{"version":"2.1.0","runs":[]}`),
		ir.ProviderSpec{Allow: map[string]any{"max_findings": "bad"}}); err == nil {
		t.Fatal("invalid maximum accepted")
	}
	if _, _, err := evaluateSARIF([]byte(`{"runs":[]}`), ir.ProviderSpec{}); err == nil {
		t.Fatal("missing SARIF version accepted")
	}
	filtered := `{"version":"2.1.0","runs":[{"results":[{"ruleId":"other"}]}]}`
	if findings, _, err := evaluateSARIF([]byte(filtered),
		ir.ProviderSpec{Match: map[string]any{"rule_id": "wanted"}}); err != nil || findings != 0 {
		t.Fatalf("findings=%d err=%v", findings, err)
	}

	base := sarifResult{RuleID: "R", Level: "error", BaselineState: "new", Properties: map[string]any{"security-severity": "9"}}
	for _, match := range []map[string]any{
		{"rule_id": "other"}, {"result_level": "warning"}, {"baseline_state": "old"},
		{"severity": "1"}, {"path": "src/**"},
	} {
		if matchesSARIF(base, match) {
			t.Fatal(match)
		}
	}
	if _, ok := integer(1.5); ok {
		t.Fatal("fractional integer accepted")
	}
	if values := stringValues("bad"); values != nil {
		t.Fatal(values)
	}
	if containsString([]string{"a"}, "b") {
		t.Fatal("missing string found")
	}
}

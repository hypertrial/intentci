package compiler

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/ir"
)

func TestV1ValidationBranches(t *testing.T) {
	confidenceHigh := 1.1
	confidenceValid := 0.8
	document := &ir.Document{Requirements: []ir.Requirement{
		{
			ID: "A", Status: "active", SourcePath: "a.md",
			DependsOn:  []string{"B", "missing"},
			AppliesTo:  ir.AppliesTo{Paths: []string{"../escape"}},
			Boundaries: ir.Boundaries{Allowed: []string{"[", "same"}, Forbidden: []string{"same"}},
			Timeout:    "never",
			Obligations: []ir.Obligation{
				{
					ID: "one", DependsOn: []string{"two", "missing"}, EvidenceClass: "bogus",
					Severity: "bogus", ConfidenceThreshold: &confidenceHigh, Timeout: "never",
					Retry: ir.Retry{Attempts: -1}, Platforms: []string{"windows"},
				},
				{
					ID: "two", DependsOn: []string{"one"}, EvidenceClass: "deterministic",
					ConfidenceThreshold: &confidenceValid, Retry: ir.Retry{Backoff: "never"},
				},
				{ID: "duplicate"},
				{ID: "duplicate"},
			},
		},
		{ID: "B", DependsOn: []string{"A"}, SourcePath: "b.md"},
	}}
	if got := validateGraph(document); len(got) < 5 {
		t.Fatalf("graph diagnostics: %+v", got)
	}
	if got := validateRequirements(document); len(got) < 10 {
		t.Fatalf("requirement diagnostics: %+v", got)
	}
	if got := validateBoundaries(document); len(got) != 1 {
		t.Fatalf("boundary diagnostics: %+v", got)
	}
	if obligationCycle([]ir.Obligation{{ID: "ok"}, {ID: "other", DependsOn: []string{"ok"}}}) != "" {
		t.Fatal("acyclic obligations rejected")
	}
	if !validLocalID("a-B_2.0") || validLocalID("") || validLocalID("../bad") {
		t.Fatal("local id validation")
	}
	for _, value := range []string{"ok/**", "x"} {
		if !validPattern(value) || !validRelativePath(value) {
			t.Fatal(value)
		}
	}
	for _, value := range []string{"", "../x", "/x", "["} {
		if validPattern(value) {
			t.Fatal(value)
		}
	}
	if len(validateRetry("p", "o", ir.Retry{})) != 0 {
		t.Fatal("empty retry")
	}
	if !oneOf("x", "a", "x") || oneOf("x", "a", "b") {
		t.Fatal("oneOf")
	}
}

func TestV1ProviderValidationBranches(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "report.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldLookPath := lookPath
	lookPath = func(name string) (string, error) {
		if name == "intentci-provider-found" {
			return "/bin/true", nil
		}
		return "", errors.New("missing")
	}
	defer func() { lookPath = oldLookPath }()

	providerNode := func(spec ir.ProviderSpec) ir.VerifyNode {
		return ir.VerifyNode{Provider: &spec}
	}
	requirement := ir.Requirement{
		ID: "REQ", SourcePath: "r.md",
		Obligations: []ir.Obligation{
			{ID: "bad", Verify: ir.VerifyNode{All: []ir.VerifyNode{
				providerNode(ir.ProviderSpec{
					Provider: "../bad", ID: "../bad", Allowed: []string{"["},
					WorkingDirectory: "../bad", Report: "/absolute",
					InheritEnv: []string{"["}, Environment: map[string]string{"BAD=NAME": "x"},
					Timeout: "never", Retry: ir.Retry{Attempts: -1},
				}),
				providerNode(ir.ProviderSpec{
					Provider: "command", ID: "same", WorkingDirectory: "missing",
					Report: "missing.xml", InheritEnv: []string{"PATH"},
					Environment: map[string]string{"": "x"}, Retry: ir.Retry{Backoff: "never"},
				}),
				providerNode(ir.ProviderSpec{Provider: "command", ID: "same", Run: "different"}),
				providerNode(ir.ProviderSpec{Provider: "found", ID: "found", WorkingDirectory: "work"}),
				providerNode(ir.ProviderSpec{Provider: "missing", ID: "missing"}),
			}}},
			{ID: "deps", Verify: ir.VerifyNode{Any: []ir.VerifyNode{
				providerNode(ir.ProviderSpec{Provider: "command", ID: "cycle-a", Run: "true", DependsOn: []string{"cycle-b", "absent"}}),
				providerNode(ir.ProviderSpec{Provider: "command", ID: "cycle-b", Run: "true", DependsOn: []string{"cycle-a"}}),
			}}},
			{ID: "not", Verify: ir.VerifyNode{Not: ptrNode(providerNode(ir.ProviderSpec{
				Provider: "json", ID: "json", Report: "report.json",
				Assert: map[string]any{"path": "$.x", "operator": "exists"},
			}))}},
		},
	}
	document := &ir.Document{Requirements: []ir.Requirement{requirement}}
	diagnostics := validateProviders(root, document)
	if len(diagnostics) < 12 {
		t.Fatalf("provider diagnostics: %+v", diagnostics)
	}

	var ids []string
	collectProviderIDs(document.Requirements[0].Obligations[0].Verify, func(id string) {
		ids = append(ids, id)
	})
	if len(ids) != 5 {
		t.Fatalf("%v", ids)
	}
	paths := providerPaths(ir.ProviderSpec{
		Allowed: []string{"a"}, Forbidden: []string{"b"}, Paths: []string{"c"},
		Inputs: []string{"d"}, Outputs: []string{"e"}, Artifacts: []string{"f"},
	})
	if strings.Join(paths, "") != "abcdef" {
		t.Fatalf("%v", paths)
	}
}

func TestV1CompilerWarnings(t *testing.T) {
	cfg := config.Default()
	cfg.Verification.DefaultTimeout = ""
	document := &ir.Document{Requirements: []ir.Requirement{
		{
			ID: "disabled", Status: "disabled",
		},
		{
			ID: "active", Status: "active", SourcePath: "r.md", DependsOn: []string{"disabled"},
			Boundaries: ir.Boundaries{
				Allowed:   []string{"**", "tests/**"},
				Forbidden: []string{"**/*"},
			},
			Obligations: []ir.Obligation{
				{ID: "empty"},
				{ID: "prob", EvidenceClass: "probabilistic", Verify: ir.VerifyNode{
					Provider: &ir.ProviderSpec{
						Provider: "command", Run: "TOKEN=literal git add x > y",
						Environment: map[string]string{"API_KEY": "literal"},
					},
				}},
				{ID: "expected", Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
					Provider: "command", Run: "true", Result: map[string]any{"stdout": map[string]any{"contains": "x"}},
				}}},
			},
		},
	}}
	diagnostics := compilerWarnings(document, cfg)
	messages := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != "warning" {
			t.Fatalf("%+v", diagnostic)
		}
		messages = append(messages, diagnostic.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{
		"no applies_to", "no owner", "broad file boundary", "test path",
		"disabled requirement", "only probabilistic", "only on command exit",
		"no timeout", "literal credential", "may modify tracked",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
	if environmentCredential(map[string]string{"PATH": "/bin"}) ||
		!environmentCredential(map[string]string{"PASSWORD": "x"}) {
		t.Fatal("environment credential")
	}
	for _, value := range []string{"token=$TOKEN", `token="[REDACTED]"`, "plain"} {
		if literalCredential(value) {
			t.Fatal(value)
		}
	}
	if !literalCredential("password: visible") ||
		!containsPattern([]string{"a", "b"}, "b") ||
		containsPattern([]string{"a"}, "b") ||
		firstNonEmpty("", "x", "y") != "x" || firstNonEmpty("", "") != "" ||
		hasOutputExpectation(nil) || hasOutputExpectation(map[string]any{}) ||
		!hasOutputExpectation(map[string]any{"stderr": "x"}) {
		t.Fatal("warning helpers")
	}
}

func TestV1CompileWorkingDirectoryAndWriteValidation(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	cfg.Project.Name = "p"
	cfg.Verification.WorkingDirectory = "missing"
	result, err := Compile(Options{Root: root, Config: cfg})
	if err == nil || len(result.Diagnostics) == 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}

	acyclic := ir.Requirement{SourcePath: "r.md", Obligations: []ir.Obligation{{
		Verify: ir.VerifyNode{All: []ir.VerifyNode{
			{Provider: &ir.ProviderSpec{Provider: "command", ID: "a", DependsOn: []string{"leaf"}}},
			{Provider: &ir.ProviderSpec{Provider: "command", ID: "b", DependsOn: []string{"leaf"}}},
			{Provider: &ir.ProviderSpec{Provider: "command", ID: "leaf"}},
		}},
	}}}
	if diagnostics := validateVerifierGraph(acyclic); len(diagnostics) != 0 {
		t.Fatalf("%+v", diagnostics)
	}

	invalid := &ir.Document{SchemaVersion: 2, Project: "p", Hash: "set", Requirements: []ir.Requirement{}}
	if err := WriteIR(invalid, filepath.Join(root, "invalid.json")); err == nil {
		t.Fatal("schema-invalid IR written")
	}
}

func ptrNode(node ir.VerifyNode) *ir.VerifyNode { return &node }

package compiler

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/initcmd"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/parser"
)

func TestMutationSensitiveCompilationSelectionAndSourcePath(t *testing.T) {
	root := t.TempDir()
	if err := initcmd.Run(initcmd.Options{Root: root}); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(root, ".intentci", "requirements", "REQ-001.md")
	second := strings.ReplaceAll(string(readTestFile(t, first)), "REQ-001", "REQ-002")
	if err := os.WriteFile(filepath.Join(filepath.Dir(first), "REQ-002.md"), []byte(second), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(Options{Root: root, RequirementID: "REQ-002"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Document.Requirements) != 1 ||
		result.Document.Requirements[0].ID != "REQ-002" ||
		result.Document.Requirements[0].SourcePath != ".intentci/requirements/REQ-002.md" {
		t.Fatalf("filtered compilation = %#v", result.Document.Requirements)
	}
}

func TestMutationSensitiveRequirementMetadataValidation(t *testing.T) {
	zero, one, below, above, half := 0.0, 1.0, -0.1, 1.1, 0.5
	document := &ir.Document{Requirements: []ir.Requirement{{
		ID: "REQ", SourcePath: "requirement.md", Timeout: "invalid",
		Obligations: []ir.Obligation{
			{ID: "ZERO", EvidenceClass: "probabilistic", ConfidenceThreshold: &zero},
			{ID: "ONE", EvidenceClass: "probabilistic", ConfidenceThreshold: &one},
			{ID: "BELOW", EvidenceClass: "probabilistic", ConfidenceThreshold: &below},
			{ID: "ABOVE", EvidenceClass: "probabilistic", ConfidenceThreshold: &above},
			{ID: "CLASS", EvidenceClass: "deterministic", ConfidenceThreshold: &half},
			{ID: "TIMEOUT", Timeout: "invalid"},
			{ID: "RETRY", Retry: ir.Retry{Backoff: "invalid"}},
		},
	}}}
	diagnostics := validateRequirements(document)
	for _, expected := range []string{
		"timeout:", "BELOW: confidence_threshold", "ABOVE: confidence_threshold",
		"CLASS: confidence_threshold requires", "TIMEOUT: timeout:", "RETRY: retry.backoff:",
	} {
		if !diagnosticsContain(diagnostics, expected) {
			t.Fatalf("missing %q in %#v", expected, diagnostics)
		}
	}
	for _, unexpected := range []string{"ZERO:", "ONE:"} {
		if diagnosticsContain(diagnostics, unexpected) {
			t.Fatalf("valid confidence boundary rejected: %#v", diagnostics)
		}
	}
	if countDiagnostics(diagnostics, "timeout:") != 2 {
		t.Fatalf("requirement and obligation timeout diagnostics = %#v", diagnostics)
	}
}

func TestMutationSensitiveProviderValidation(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "report.xml"), []byte("<testsuites/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(name string) (string, error) {
		if name == "intentci-provider-custom" {
			return "/bin/true", nil
		}
		return "", errors.New("missing")
	}
	valid := ir.ProviderSpec{
		Provider: "custom", ID: "aZ09-_.", WorkingDirectory: "work",
		InheritEnv: []string{"PATH"}, Environment: map[string]string{"VALID": "value"},
		Timeout: "1s",
	}
	validReport := ir.ProviderSpec{Provider: "junit", ID: "report", Report: "report.xml"}
	document := providerDocument(valid, validReport)
	if diagnostics := validateProviders(root, document); len(diagnostics) != 0 {
		t.Fatalf("valid providers rejected: %#v", diagnostics)
	}

	if err := os.WriteFile(filepath.Join(root, "not-a-directory"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "report-directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	invalid := []ir.ProviderSpec{
		{Provider: "missing", ID: "bad/"},
		{Provider: "command", ID: "missing-work", WorkingDirectory: "missing"},
		{Provider: "command", ID: "file-work", WorkingDirectory: "not-a-directory"},
		{Provider: "junit", ID: "missing-report", Report: "missing.xml"},
		{Provider: "junit", ID: "directory-report", Report: "report-directory"},
		{Provider: "command", ID: "inherit", InheritEnv: []string{"["}},
		{Provider: "command", ID: "environment", Environment: map[string]string{"": "x"}},
		{Provider: "command", ID: "timeout", Timeout: "invalid"},
	}
	document = providerDocument(invalid...)
	diagnostics := validateProviders(root, document)
	for _, expected := range []string{
		"unsupported provider", "invalid verifier id", "working_directory does not exist",
		"referenced report does not exist", "invalid inherited environment",
		"invalid environment name", "timeout:",
	} {
		if !diagnosticsContain(diagnostics, expected) {
			t.Fatalf("missing %q in %#v", expected, diagnostics)
		}
	}
}

func TestMutationSensitiveVerifierIdentifiersAndGraphs(t *testing.T) {
	for _, value := range []string{"a", "z", "A", "Z", "0", "9", "-", "_", ".", "aZ09-_."} {
		if !validLocalID(value) {
			t.Fatalf("valid id %q rejected", value)
		}
	}
	for _, value := range []string{"", "`", "{", "@", "[", "/", "="} {
		if validLocalID(value) {
			t.Fatalf("invalid id %q accepted", value)
		}
	}
	requirement := ir.Requirement{
		SourcePath: "requirement.md",
		Obligations: []ir.Obligation{
			{Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "command", ID: "same", Run: "true"}}},
			{Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "command", ID: "same", Run: "false"}}},
		},
	}
	if diagnostics := validateVerifierGraph(requirement); !diagnosticsContain(diagnostics, "incompatible configuration") {
		t.Fatalf("incompatible verifier reuse accepted: %#v", diagnostics)
	}
	requirement.Obligations[1].Verify.Provider.Run = "true"
	if diagnostics := validateVerifierGraph(requirement); len(diagnostics) != 0 {
		t.Fatalf("identical verifier reuse rejected: %#v", diagnostics)
	}
	document := &ir.Document{Requirements: []ir.Requirement{{
		ID: "REQ", SourcePath: "requirement.md", Obligations: []ir.Obligation{{
			ID: "OBL", Verify: ir.VerifyNode{All: []ir.VerifyNode{
				{Provider: &ir.ProviderSpec{Provider: "command", ID: "duplicate", Run: "true"}},
				{Provider: &ir.ProviderSpec{Provider: "command", ID: "duplicate", Run: "true"}},
			}},
		}},
	}}}
	if diagnostics := validateProviders(t.TempDir(), document); !diagnosticsContain(diagnostics, "duplicate verifier id") {
		t.Fatalf("duplicate verifier id not diagnosed: %#v", diagnostics)
	}
}

func TestMutationSensitiveRetryBoundaryAndWarnings(t *testing.T) {
	if diagnostics := validateRetry("r.md", "OBL", ir.Retry{Backoff: "1s"}); len(diagnostics) != 0 {
		t.Fatalf("valid retry rejected: %#v", diagnostics)
	}
	if diagnostics := validateRetry("r.md", "OBL", ir.Retry{Backoff: "bad"}); !diagnosticsContain(diagnostics, "retry.backoff") {
		t.Fatalf("invalid retry accepted: %#v", diagnostics)
	}
	document := &ir.Document{Requirements: []ir.Requirement{{
		ID: "REQ", SourcePath: "r.md",
		Boundaries: ir.Boundaries{Allowed: []string{"src/**"}, Forbidden: []string{"src/**"}},
	}}}
	if diagnostics := validateBoundaries(document); !diagnosticsContain(diagnostics, "contradictory boundary") {
		t.Fatalf("contradictory boundary accepted: %#v", diagnostics)
	}
	document.Requirements[0].Boundaries = ir.Boundaries{
		Allowed: []string{"**", "**/*"}, Forbidden: []string{"other/**"},
	}
	warnings := compilerWarnings(document, config.Default())
	if countDiagnostics(warnings, "broad file boundary") != 2 {
		t.Fatalf("broad boundary warnings = %#v", warnings)
	}
	if !diagnosticsContain(warnings, `broad file boundary "**"`) ||
		!diagnosticsContain(warnings, `broad file boundary "**/*"`) ||
		diagnosticsContain(warnings, `broad file boundary "other/**"`) {
		t.Fatalf("broad boundary identities = %#v", warnings)
	}
}

func TestMutationSensitiveWriteIRComputesHash(t *testing.T) {
	document := &ir.Document{
		SchemaVersion: 1, Project: "project", Requirements: []ir.Requirement{},
	}
	path := filepath.Join(t.TempDir(), "ir.json")
	if err := WriteIR(document, path); err != nil {
		t.Fatal(err)
	}
	if document.Hash == "" {
		t.Fatal("WriteIR did not compute the document hash")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	bad := &ir.Document{
		SchemaVersion: 1, Project: "project",
		Requirements: []ir.Requirement{{
			ID: "REQ", Obligations: []ir.Obligation{{
				ID: "OBL", Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
					Provider: "command", Extra: map[string]any{"bad": make(chan int)},
				}},
			}},
		}},
	}
	if err := WriteIR(bad, filepath.Join(t.TempDir(), "bad.json")); err == nil {
		t.Fatal("hash computation failure ignored")
	}
}

func providerDocument(specs ...ir.ProviderSpec) *ir.Document {
	obligations := make([]ir.Obligation, 0, len(specs))
	for index := range specs {
		spec := specs[index]
		obligations = append(obligations, ir.Obligation{
			ID: "OBL", Verify: ir.VerifyNode{Provider: &spec},
		})
	}
	return &ir.Document{Requirements: []ir.Requirement{{
		ID: "REQ", SourcePath: "requirement.md", Obligations: obligations,
	}}}
}

func diagnosticsContain(diagnostics []parser.Diagnostic, want string) bool {
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, want) {
			return true
		}
	}
	return false
}

func countDiagnostics(diagnostics []parser.Diagnostic, want string) int {
	count := 0
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, want) {
			count++
		}
	}
	return count
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

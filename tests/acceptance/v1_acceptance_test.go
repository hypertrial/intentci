package acceptance_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

var repositoryRoot string
var intentciBinary string

func TestMain(m *testing.M) {
	_, file, _, _ := runtime.Caller(0)
	repositoryRoot = filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	buildDir, err := os.MkdirTemp("", "intentci-acceptance-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	intentciBinary = filepath.Join(buildDir, "intentci")
	command := exec.Command("go", "build", "-trimpath", "-o", intentciBinary, "./cmd/intentci")
	command.Dir = repositoryRoot
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		_ = os.RemoveAll(buildDir)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(buildDir)
	os.Exit(code)
}

type acceptanceResult struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Test        string `json:"test"`
}

type acceptanceMatrix struct {
	SchemaVersion string             `json:"schema_version"`
	Specification string             `json:"specification"`
	GeneratedAt   time.Time          `json:"generated_at"`
	Commit        string             `json:"commit,omitempty"`
	Platform      string             `json:"platform"`
	AllPassed     bool               `json:"all_passed"`
	Criteria      []acceptanceResult `json:"criteria"`
}

type acceptanceSuite struct {
	workspace         string
	passingRepository string
	passingBundle     *evidence.Bundle
	repairRepository  string
	repairBundle      *evidence.Bundle
}

func TestV1Acceptance(t *testing.T) {
	workspace, err := os.MkdirTemp("", "intentci-v1-suite-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(workspace) })
	suite := &acceptanceSuite{workspace: workspace}
	cases := []struct {
		id          string
		description string
		run         func(*testing.T, *acceptanceSuite)
	}{
		{"AC-01", "A user can initialize IntentCI in an existing repository.", acceptInitialize},
		{"AC-02", "Requirements can be authored in human-readable Markdown.", acceptMarkdown},
		{"AC-03", "Requirement files compile into stable canonical JSON.", acceptCanonicalCompilation},
		{"AC-04", "Invalid requirement graphs fail with actionable diagnostics.", acceptInvalidGraph},
		{"AC-05", "Existing test commands can verify obligations.", acceptCommandVerification},
		{"AC-06", "JUnit and SARIF reports can be mapped to obligations.", acceptReportProviders},
		{"AC-07", "File-boundary violations are detected.", acceptBoundaryViolation},
		{"AC-08", "Changed files can select affected requirements.", acceptChangedSelection},
		{"AC-09", "Evidence is tied to repository state and contract hashes.", acceptEvidenceProvenance},
		{"AC-10", "Every required obligation receives an explicit verdict.", acceptExplicitVerdicts},
		{"AC-11", "Missing evidence cannot produce a passing requirement.", acceptMissingEvidence},
		{"AC-12", "A failed requirement produces a structured repair packet.", acceptRepairPacket},
		{"AC-13", "An external coding agent can be invoked for bounded repair attempts.", acceptRepairAgent},
		{"AC-14", "The agent cannot silently modify protected contracts.", acceptProtectedContract},
		{"AC-15", "Repeated ineffective attempts are stopped.", acceptRepeatedFailure},
		{"AC-16", "Terminal, JSON, and JUnit reports are generated.", acceptReports},
		{"AC-17", "GitHub Actions can use the CLI without a custom service.", acceptGitHubActions},
		{"AC-18", "Linux and macOS are supported.", acceptPlatforms},
		{"AC-19", "No telemetry is sent by default.", acceptTelemetryDefault},
		{"AC-20", "The complete end-to-end workflow is covered by automated tests.", acceptCompleteWorkflow},
	}
	matrix := acceptanceMatrix{
		SchemaVersion: "1.0", Specification: "v1.md#38", GeneratedAt: time.Now().UTC(),
		Commit: gitOutput(repositoryRoot, "rev-parse", "HEAD"), Platform: runtime.GOOS + "/" + runtime.GOARCH,
		AllPassed: true,
	}
	defer func() {
		path := os.Getenv("INTENTCI_ACCEPTANCE_OUTPUT")
		if path == "" {
			return
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Errorf("create acceptance output directory: %v", err)
			return
		}
		raw, err := json.MarshalIndent(matrix, "", "  ")
		if err != nil {
			t.Errorf("marshal acceptance matrix: %v", err)
			return
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Errorf("write acceptance matrix: %v", err)
		}
	}()
	for _, item := range cases {
		item := item
		passed := t.Run(item.id, func(t *testing.T) {
			item.run(t, suite)
		})
		status := "passed"
		if !passed {
			status = "failed"
			matrix.AllPassed = false
		}
		matrix.Criteria = append(matrix.Criteria, acceptanceResult{
			ID: item.id, Description: item.description, Status: status,
			Test: "tests/acceptance/v1_acceptance_test.go::TestV1Acceptance/" + item.id,
		})
	}
}

func acceptInitialize(t *testing.T, suite *acceptanceSuite) {
	root := filepath.Join(suite.workspace, "passing")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/acceptance\n\ngo 1.23\n")
	writeFile(t, filepath.Join(root, "calculator.go"), "package calculator\n\nfunc Add(a, b int) int { return a + b }\n")
	writeFile(t, filepath.Join(root, "calculator_test.go"), `package calculator

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Fatal("wrong sum")
	}
}
`)
	runCLI(t, root, 0, "init", "--language", "go")
	for _, relative := range []string{
		".intentci/config.yaml",
		".intentci/requirements/REQ-001.md",
	} {
		if _, err := os.Stat(filepath.Join(root, relative)); err != nil {
			t.Fatalf("%s: %v", relative, err)
		}
	}
	initializeGit(t, root)
	suite.passingRepository = root
}

func acceptMarkdown(t *testing.T, suite *acceptanceSuite) {
	raw := readFile(t, filepath.Join(suite.passingRepository, ".intentci", "requirements", "REQ-001.md"))
	for _, expected := range []string{"---", "# Intent", "# Obligations", "```yaml"} {
		if !strings.Contains(string(raw), expected) {
			t.Fatalf("generated requirement missing %q", expected)
		}
	}
}

func acceptCanonicalCompilation(t *testing.T, suite *acceptanceSuite) {
	first := filepath.Join(t.TempDir(), "first.json")
	second := filepath.Join(t.TempDir(), "second.json")
	runCLI(t, suite.passingRepository, 0, "compile", "--strict", "--output", first)
	runCLI(t, suite.passingRepository, 0, "compile", "--strict", "--output", second)
	firstRaw, secondRaw := readFile(t, first), readFile(t, second)
	if string(firstRaw) != string(secondRaw) {
		t.Fatal("repeated compilation was not byte-identical")
	}
	var document ir.Document
	if err := json.Unmarshal(firstRaw, &document); err != nil || document.Hash == "" {
		t.Fatalf("canonical IR is invalid: hash=%q err=%v", document.Hash, err)
	}
}

func acceptInvalidGraph(t *testing.T, _ *acceptanceSuite) {
	root := t.TempDir()
	runCLI(t, root, 0, "init")
	path := filepath.Join(root, ".intentci", "requirements", "REQ-001.md")
	body := string(readFile(t, path))
	body = strings.Replace(body, "depends_on: []", "depends_on:\n  - REQ-MISSING", 1)
	writeFile(t, path, body)
	_, stderr := runCLI(t, root, 5, "compile", "--strict")
	if !strings.Contains(stderr, "REQ-MISSING") ||
		!strings.Contains(stderr, ".intentci/requirements/REQ-001.md") {
		t.Fatalf("diagnostic is not actionable:\n%s", stderr)
	}
}

func acceptCommandVerification(t *testing.T, suite *acceptanceSuite) {
	reportPath := filepath.Join(t.TempDir(), "report.json")
	runCLI(t, suite.passingRepository, 0,
		"verify", "--all", "--base", "HEAD", "--no-cache", "--format", "json", "--output", reportPath)
	var bundle evidence.Bundle
	if err := json.Unmarshal(readFile(t, reportPath), &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.Run.Verdict != verdict.Pass {
		t.Fatalf("run verdict = %s", bundle.Run.Verdict)
	}
	store, err := evidence.NewStore(suite.passingRepository, ".intentci/runs")
	if err != nil {
		t.Fatal(err)
	}
	suite.passingBundle, err = store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	runCLI(t, suite.passingRepository, 0, "explain", "REQ-001", "--show-evidence")
}

func acceptReportProviders(t *testing.T, _ *acceptanceSuite) {
	root := t.TempDir()
	copyFile(t, filepath.Join(repositoryRoot, "fixtures", "reports", "junit-failure.xml"), filepath.Join(root, "junit.xml"))
	copyFile(t, filepath.Join(repositoryRoot, "fixtures", "reports", "sarif-error.json"), filepath.Join(root, "sarif.json"))
	junit := (&provider.JUnitProvider{}).Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{ID: "junit", Report: "junit.xml"},
	})
	sarif := (&provider.SARIFProvider{}).Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{
			ID: "sarif", Report: "sarif.json", Allow: map[string]any{"max_findings": 0},
		},
	})
	for name, result := range map[string]provider.Result{"junit": junit, "sarif": sarif} {
		if result.Status != "completed" || len(result.Evidence) != 1 ||
			result.Evidence[0].Passed == nil || *result.Evidence[0].Passed {
			t.Fatalf("%s did not map a failing report: %#v", name, result)
		}
	}
}

func acceptBoundaryViolation(t *testing.T, _ *acceptanceSuite) {
	result := (&provider.BoundaryProvider{}).Execute(context.Background(), provider.Request{
		ChangedFiles: []string{"src/allowed.go", "secrets/token.txt"},
		Spec: ir.ProviderSpec{
			ID: "boundary", Allowed: []string{"src/**"}, Forbidden: []string{"secrets/**"},
		},
	})
	if !result.SecurityViolation || len(result.Evidence) != 1 ||
		result.Evidence[0].Passed == nil || *result.Evidence[0].Passed {
		t.Fatalf("boundary violation was not detected: %#v", result)
	}
}

func acceptChangedSelection(t *testing.T, _ *acceptanceSuite) {
	document := &ir.Document{Requirements: []ir.Requirement{
		{ID: "REQ-A", Status: "active", AppliesTo: ir.AppliesTo{Paths: []string{"src/a/**"}}},
		{ID: "REQ-B", Status: "active", AppliesTo: ir.AppliesTo{Paths: []string{"src/b/**"}}},
	}}
	selection := impact.Select(document, impact.Options{ChangedFiles: []string{"src/a/file.go"}})
	if len(selection.Requirements) != 1 || selection.Requirements[0].ID != "REQ-A" {
		t.Fatalf("unexpected impact selection: %#v", selection)
	}
}

func acceptEvidenceProvenance(t *testing.T, suite *acceptanceSuite) {
	bundle := suite.passingBundle
	if bundle == nil {
		t.Fatal("passing verification bundle is unavailable")
	}
	if bundle.IRHash == "" || bundle.HeadCommit == "" ||
		bundle.RepositoryState == nil || bundle.RepositoryState.DiffHash == "" {
		t.Fatalf("bundle provenance incomplete: %#v", bundle)
	}
	for _, result := range bundle.ProviderLogs {
		for _, record := range result.Evidence {
			if record.RequirementHash == "" || record.ObligationHash == "" ||
				record.PlanHash == "" || record.RepositoryCommit == "" {
				t.Fatalf("evidence provenance incomplete: %#v", record)
			}
		}
	}
}

func acceptExplicitVerdicts(t *testing.T, suite *acceptanceSuite) {
	if suite.passingBundle == nil {
		t.Fatal("passing verification bundle is unavailable")
	}
	for _, requirement := range suite.passingBundle.Run.Requirements {
		for _, obligation := range requirement.Obligations {
			if obligation.Required && obligation.Verdict == "" {
				t.Fatalf("%s/%s has no verdict", requirement.ID, obligation.ID)
			}
		}
	}
}

func acceptMissingEvidence(t *testing.T, _ *acceptanceSuite) {
	node := ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "external", ID: "missing"}}
	got, _, _ := verdict.EvaluateNode(node, nil)
	if got == verdict.Pass {
		t.Fatal("missing provider evidence produced pass")
	}
}

func acceptRepairPacket(t *testing.T, suite *acceptanceSuite) {
	root := copyRepairFixture(t)
	_, stderr := runCLI(t, root, 9, "repair", "--dry-run", "--max-attempts", "1")
	if !strings.Contains(stderr, "max_attempts") {
		t.Fatalf("repair stop reason missing: %s", stderr)
	}
	store, err := evidence.NewStore(root, ".intentci/runs")
	if err != nil {
		t.Fatal(err)
	}
	runID := strings.TrimSpace(string(readFile(t, filepath.Join(store.Root, "latest"))))
	packetPath := filepath.Join(store.Dir(runID), "attempts", "attempt-001", "repair-packet.json")
	var packet map[string]any
	if err := json.Unmarshal(readFile(t, packetPath), &packet); err != nil {
		t.Fatal(err)
	}
	if packet["run_id"] != runID || len(packet["failures"].([]any)) == 0 {
		t.Fatalf("repair packet is incomplete: %#v", packet)
	}
	suite.repairRepository = root
}

func acceptRepairAgent(t *testing.T, suite *acceptanceSuite) {
	root := filepath.Join(suite.workspace, "repair-success")
	copyTree(t, filepath.Join(repositoryRoot, "fixtures", "repair-go"), root)
	initializeGit(t, root)
	runCLI(t, root, 0,
		"repair", "--agent-command", "sh repair/fake-agent.sh {packet}", "--max-attempts", "2")
	store, err := evidence.NewStore(root, ".intentci/runs")
	if err != nil {
		t.Fatal(err)
	}
	suite.repairBundle, err = store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	runCLI(t, root, 0, "verify", "--all", "--base", "HEAD", "--no-cache")
	suite.repairRepository = root
	if suite.repairBundle.Run.Verdict != verdict.Pass {
		t.Fatalf("repaired verdict = %s", suite.repairBundle.Run.Verdict)
	}
}

func acceptProtectedContract(t *testing.T, _ *acceptanceSuite) {
	root := copyRepairFixture(t)
	command := "printf '\\nmalicious contract edit\\n' >> .intentci/requirements/REQ-REPAIR-001.md"
	_, stderr := runCLI(t, root, 10, "repair", "--agent-command", command, "--max-attempts", "2")
	if !strings.Contains(stderr, "protected_path") {
		t.Fatalf("protected-path stop reason missing: %s", stderr)
	}
}

func acceptRepeatedFailure(t *testing.T, _ *acceptanceSuite) {
	root := copyRepairFixture(t)
	_, stderr := runCLI(t, root, 9, "repair", "--agent-command", "true", "--max-attempts", "3")
	if !strings.Contains(stderr, "repeated_failure") {
		t.Fatalf("repeated-failure stop reason missing: %s", stderr)
	}
}

func acceptReports(t *testing.T, suite *acceptanceSuite) {
	if suite.repairBundle == nil {
		t.Fatal("successful repair bundle is unavailable")
	}
	runDir := filepath.Join(suite.repairRepository, ".intentci", "runs", suite.repairBundle.RunID)
	for _, relative := range []string{"report.txt", "report.json", "report.junit.xml"} {
		if info, err := os.Stat(filepath.Join(runDir, relative)); err != nil || info.Size() == 0 {
			t.Fatalf("%s: size=%v err=%v", relative, sizeOf(info), err)
		}
	}
}

func acceptGitHubActions(t *testing.T, _ *acceptanceSuite) {
	raw := string(readFile(t, filepath.Join(repositoryRoot, "examples", "github-actions", "intentci.yml")))
	if !strings.Contains(raw, "intentci verify") || strings.Contains(raw, "curl ") {
		t.Fatalf("workflow does not use the standalone CLI:\n%s", raw)
	}
}

func acceptPlatforms(t *testing.T, _ *acceptanceSuite) {
	for _, target := range []string{"linux/amd64", "darwin/amd64", "darwin/arm64"} {
		parts := strings.Split(target, "/")
		output := filepath.Join(t.TempDir(), "intentci-"+parts[0]+"-"+parts[1])
		command := exec.Command("go", "build", "-trimpath", "-o", output, "./cmd/intentci")
		command.Dir = repositoryRoot
		command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+parts[0], "GOARCH="+parts[1])
		if raw, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", target, err, raw)
		}
	}
}

func acceptTelemetryDefault(t *testing.T, _ *acceptanceSuite) {
	if config.Default().Telemetry.Enabled {
		t.Fatal("telemetry is enabled by default")
	}
}

func acceptCompleteWorkflow(t *testing.T, suite *acceptanceSuite) {
	if suite.passingBundle == nil || suite.repairBundle == nil {
		t.Fatal("init/compile/verify/explain or fail/repair/pass workflow did not complete")
	}
	runDir := filepath.Join(suite.repairRepository, ".intentci", "runs", suite.repairBundle.RunID)
	for _, relative := range []string{
		"compiled-intent.json", "verification-plan.json", "repository-state.json", "diff.patch",
		"attempts/attempt-001/evidence.json", "attempts/attempt-001/verdict.json",
		"attempts/attempt-002/evidence.json", "attempts/attempt-002/verdict.json",
		"manifest.json", "final-verdict.json",
	} {
		if _, err := os.Stat(filepath.Join(runDir, relative)); err != nil {
			t.Fatalf("%s: %v", relative, err)
		}
	}
	var manifest evidence.Manifest
	manifestRaw := readFile(t, filepath.Join(runDir, "manifest.json"))
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, artifact := range manifest.Artifacts {
		raw := readFile(t, filepath.Join(runDir, filepath.FromSlash(artifact.Path)))
		sum := sha256.Sum256(raw)
		if artifact.SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("manifest hash mismatch for %s", artifact.Path)
		}
	}
	var final evidence.FinalVerdict
	if err := json.Unmarshal(readFile(t, filepath.Join(runDir, "final-verdict.json")), &final); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(manifestRaw)
	if final.ManifestHash != hex.EncodeToString(sum[:]) {
		t.Fatal("final verdict does not reference the manifest hash")
	}
}

func runCLI(t *testing.T, root string, wantCode int, arguments ...string) (string, string) {
	t.Helper()
	command := exec.Command(intentciBinary, arguments...)
	command.Dir = root
	command.Env = os.Environ()
	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	code := 0
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			t.Fatalf("intentci %v: %v", arguments, err)
		}
		code = exitError.ExitCode()
	}
	if code != wantCode {
		t.Fatalf("intentci %v returned %d, want %d\nstdout:\n%s\nstderr:\n%s",
			arguments, code, wantCode, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

func copyRepairFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "fixtures", "repair-go"), root)
	initializeGit(t, root)
	return root
}

func initializeGit(t *testing.T, root string) {
	t.Helper()
	for _, arguments := range [][]string{
		{"init"},
		{"config", "user.email", "acceptance@intentci.test"},
		{"config", "user.name", "IntentCI Acceptance"},
		{"add", "."},
		{"commit", "-m", "fixture"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if raw, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, raw)
		}
	}
}

func copyTree(t *testing.T, source, target string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, raw, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, source, target string) {
	t.Helper()
	raw := readFile(t, source)
	if err := os.WriteFile(target, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func gitOutput(root string, arguments ...string) string {
	command := exec.Command("git", arguments...)
	command.Dir = root
	raw, _ := command.Output()
	return strings.TrimSpace(string(raw))
}

func sizeOf(info os.FileInfo) int64 {
	if info == nil {
		return 0
	}
	return info.Size()
}

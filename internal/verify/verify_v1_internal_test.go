package verify

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/executor"
	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/verdict"
)

func emptyDocument(t *testing.T) *ir.Document {
	t.Helper()
	document := &ir.Document{SchemaVersion: 1, Project: "project", Requirements: []ir.Requirement{}}
	if err := document.ComputeHashes(); err != nil {
		t.Fatal(err)
	}
	return document
}

func verificationConfig(directory string) *config.Config {
	cfg := config.Default()
	cfg.Evidence.Directory = directory
	return cfg
}

func gitVerificationRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, arguments := range [][]string{
		{"init"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", "."}, {"commit", "-m", "base"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	return root
}

func TestPinnedDocumentNoGitOverridesAndInterruption(t *testing.T) {
	root := t.TempDir()
	cfg := verificationConfig("runs")
	outcome, err := Run(context.Background(), Options{
		Root: root, Config: cfg, Document: emptyDocument(t), All: true, NoGit: true,
		MaxParallel: 2, MaxParallelSet: true, FailFast: true, FailFastSet: true,
		AttemptOnly: true, RunID: "run", AttemptID: "attempt",
	})
	if err != nil || outcome.ExitCode != exitcode.Pass ||
		cfg.Verification.MaxParallel != 2 || !cfg.Verification.FailFast {
		t.Fatalf("%+v %v", outcome, err)
	}
	if _, err := Run(context.Background(), Options{
		Root: root, Config: cfg, Document: emptyDocument(t), Changed: true, NoGit: true,
	}); err == nil {
		t.Fatal("changed mode accepted with --no-git")
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	outcome, err = Run(cancelled, Options{
		Root: t.TempDir(), Config: verificationConfig("runs"), Document: emptyDocument(t),
		All: true, NoGit: true, AttemptOnly: true, RunID: "cancelled", AttemptID: "attempt",
	})
	if err != nil || outcome.ExitCode != exitcode.VerifierError ||
		!outcome.Bundle.Interrupted || outcome.Bundle.Run.Verdict != verdict.Error {
		t.Fatalf("%+v %v", outcome, err)
	}
}

func TestPinnedDocumentAndSelectorFailures(t *testing.T) {
	invalid := &ir.Document{SchemaVersion: 2, Project: "project", Hash: "hash", Requirements: []ir.Requirement{}}
	outcome, err := Run(context.Background(), Options{
		Root: t.TempDir(), Config: verificationConfig("runs"), Document: invalid, All: true, NoGit: true,
	})
	if err == nil || outcome.ExitCode != exitcode.CompileFailed {
		t.Fatalf("%+v %v", outcome, err)
	}
	document := emptyDocument(t)
	for _, options := range []Options{
		{RequirementID: "missing"},
		{ObligationID: "missing"},
		{ProviderID: "missing"},
	} {
		options.Root = t.TempDir()
		options.Config = verificationConfig("runs")
		options.Document = document
		options.All = true
		options.NoGit = true
		outcome, err := Run(context.Background(), options)
		if err == nil || outcome.ExitCode != exitcode.Usage {
			t.Fatalf("%+v %v", outcome, err)
		}
	}
}

func TestCleanWorktreeAndFailOnUnmapped(t *testing.T) {
	root := gitVerificationRepo(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := verificationConfig("runs")
	cfg.Verification.RequireCleanWorktree = true
	outcome, err := Run(context.Background(), Options{
		Root: root, Config: cfg, Document: emptyDocument(t), All: true, Base: "HEAD",
	})
	if err == nil || outcome.ExitCode != exitcode.VerifierError {
		t.Fatalf("%+v %v", outcome, err)
	}

	cfg.Verification.RequireCleanWorktree = false
	cfg.ChangeImpact.FailOnUnmapped = true
	document := &ir.Document{
		SchemaVersion: 1, Project: "project",
		Requirements: []ir.Requirement{{
			ID: "REQ-1", Title: "Requirement", Status: "active", Priority: "required",
			AppliesTo: ir.AppliesTo{Paths: []string{"src/**"}}, Intent: "intent", SourcePath: "requirement.md",
			Obligations: []ir.Obligation{{
				ID: "OBL", Statement: "statement", Required: true,
				Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
					Provider: "command", ID: "command", Run: "true",
				}},
			}},
		}},
	}
	if err := document.ComputeHashes(); err != nil {
		t.Fatal(err)
	}
	outcome, err = Run(context.Background(), Options{
		Root: root, Config: cfg, Document: document, Changed: true, Base: "HEAD",
		RunID: "unmapped", AttemptID: "attempt",
	})
	if err != nil || outcome.Bundle.Run.Verdict != verdict.Fail || len(outcome.Bundle.Unmapped) == 0 {
		t.Fatalf("%+v %v", outcome, err)
	}
}

func TestSecurityViolationAndConversionHelpers(t *testing.T) {
	document := &ir.Document{
		SchemaVersion: 1, Project: "project",
		Requirements: []ir.Requirement{{
			ID: "REQ-1", Title: "Requirement", Status: "active", Priority: "required",
			Intent: "intent", SourcePath: "requirement.md",
			Obligations: []ir.Obligation{{
				ID: "OBL", Statement: "statement", Required: true,
				Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
					Provider: "json", ID: "json", Report: "../escape",
					Assert: map[string]any{"value": true},
				}},
			}},
		}},
	}
	if err := document.ComputeHashes(); err != nil {
		t.Fatal(err)
	}
	outcome, err := Run(context.Background(), Options{
		Root: t.TempDir(), Config: verificationConfig("runs"), Document: document,
		All: true, NoGit: true, RunID: "security", AttemptID: "attempt",
	})
	if err != nil || outcome.ExitCode != exitcode.SecurityBoundary {
		t.Fatalf("%+v %v", outcome, err)
	}

	changes := providerChanges([]git.Change{{
		Path: "new", OldPath: "old", Status: "renamed", Additions: 1, Deletions: 2,
		Binary: true, OldMode: "1", NewMode: "2",
	}})
	if len(changes) != 1 || changes[0].OldPath != "old" || !changes[0].Binary {
		t.Fatal(changes)
	}
	flattened := flattenProviderResults(map[string]executor.LeafResult{
		"REQ": {"OBL/provider": {Provider: "p"}},
	})
	if flattened["REQ/OBL/provider"].Provider != "p" {
		t.Fatal(flattened)
	}
	if !hasSecurityViolation(map[string]executor.LeafResult{
		"REQ": {"x": {SecurityViolation: true}},
	}) || hasSecurityViolation(map[string]executor.LeafResult{"REQ": {"x": {}}}) {
		t.Fatal("security aggregation")
	}
	if hashConfig(config.Default()) == "" {
		t.Fatal("config hash")
	}
}

func TestSelectorTraversal(t *testing.T) {
	document := &ir.Document{Requirements: []ir.Requirement{
		{ID: "OTHER"},
		{
			ID: "REQ", Obligations: []ir.Obligation{{
				ID: "OBL", Verify: ir.VerifyNode{
					All: []ir.VerifyNode{{Provider: &ir.ProviderSpec{ID: "all"}}},
					Any: []ir.VerifyNode{{Provider: &ir.ProviderSpec{ID: "any"}}},
					Not: &ir.VerifyNode{Provider: &ir.ProviderSpec{ID: "not"}},
				},
			}},
		},
	}}
	for _, providerID := range []string{"all", "any", "not"} {
		if err := validateSelectors(document, Options{
			RequirementID: "REQ", ObligationID: "OBL", ProviderID: providerID,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, options := range []Options{
		{RequirementID: "missing"},
		{RequirementID: "REQ", ObligationID: "missing"},
		{RequirementID: "REQ", ObligationID: "OBL", ProviderID: "missing"},
	} {
		if err := validateSelectors(document, options); err == nil {
			t.Fatalf("%+v", options)
		}
	}
}

func TestFinalizeBundleFailureStages(t *testing.T) {
	root := t.TempDir()
	store, err := evidence.NewStore(root, "runs")
	if err != nil {
		t.Fatal(err)
	}
	invalid := &evidence.Bundle{RunID: "invalid", Run: verdict.RunResult{Verdict: "invalid"}}
	if err := FinalizeBundle(store, invalid); err == nil {
		t.Fatal("invalid report finalized")
	}

	finalized := &evidence.Bundle{RunID: "finalized", Run: verdict.RunResult{Verdict: verdict.Pass}}
	if err := store.WriteBundle(finalized); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeBundle(store, finalized); err == nil {
		t.Fatal("finalized run reports overwritten")
	}

	symlinked := &evidence.Bundle{RunID: "symlinked", Run: verdict.RunResult{Verdict: verdict.Pass}}
	runDir := store.Dir("symlinked")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(runDir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeBundle(store, symlinked); err == nil {
		t.Fatal("symlinked evidence finalized")
	}

	badRunDir := filepath.Join(store.Root, "bad-write")
	if err := os.WriteFile(badRunDir, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := persistRunBundle(store, &evidence.Bundle{
		RunID: "bad-write", Run: verdict.RunResult{Verdict: verdict.Pass},
	}); err == nil {
		t.Fatal("attempt write error ignored")
	}
}

func TestGitFallbackAndErrorModes(t *testing.T) {
	document := emptyDocument(t)
	outcome, err := Run(context.Background(), Options{
		Root: t.TempDir(), Config: verificationConfig("runs"), Document: document,
		All: true, RunID: "fallback",
	})
	if err != nil || outcome.ExitCode != exitcode.Pass || outcome.Bundle.RepositoryState.HeadCommit != "unknown" {
		t.Fatalf("%+v %v", outcome, err)
	}
	outcome, err = Run(context.Background(), Options{
		Root: t.TempDir(), Config: verificationConfig("runs"), Document: document, Changed: true,
	})
	if err == nil || outcome.ExitCode != exitcode.VerifierError ||
		!strings.Contains(err.Error(), "git") {
		t.Fatalf("%+v %v", outcome, err)
	}
}

func TestPersistHookReturnsBundle(t *testing.T) {
	root := t.TempDir()
	oldPersist := persistBundle
	defer func() { persistBundle = oldPersist }()
	persistBundle = func(*evidence.Store, *evidence.Bundle) error { return errors.New("persist") }
	outcome, err := Run(context.Background(), Options{
		Root: root, Config: verificationConfig("runs"), Document: emptyDocument(t),
		All: true, NoGit: true,
	})
	if err == nil || outcome.Bundle == nil || outcome.ExitCode != exitcode.Internal {
		t.Fatalf("%+v %v", outcome, err)
	}
}

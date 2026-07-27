package evidence_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/repair"
	"github.com/hypertrial/intentci/internal/verdict"
)

func TestCompleteBundleManifestRedactionAndImmutability(t *testing.T) {
	root := t.TempDir()
	store, err := evidence.NewStore(root, ".intentci/runs")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("API_TOKEN", "literal-secret")
	store.RedactPatterns = []string{"*TOKEN*"}
	document := &ir.Document{
		SchemaVersion: 1, Project: "demo",
		Requirements: []ir.Requirement{{
			ID: "REQ-001", Title: "Requirement", Status: "active", Priority: "required",
			Intent: "Keep behavior correct.", SourcePath: ".intentci/requirements/REQ-001.md",
			Obligations: []ir.Obligation{{
				ID: "OBL-001", Statement: "It passes.", Required: true,
				Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "command", ID: "check", Run: "true"}},
			}},
		}},
	}
	if err := document.ComputeHashes(); err != nil {
		t.Fatal(err)
	}
	plan, err := ir.BuildVerificationPlan(document, document.Requirements)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	passed := true
	record := provider.Evidence{
		SchemaVersion: "1.0", ID: "evidence", RunID: "run", AttemptID: "attempt-001",
		RequirementID: "REQ-001", ObligationID: "OBL-001", VerifierID: "check",
		Provider: "command", ProviderVersion: "1.0.0", Class: "deterministic",
		Status: "passed", Summary: "literal-secret passed", Passed: &passed,
		RepositoryCommit: "head", BaseCommit: "base", DiffHash: strings.Repeat("d", 64),
		RequirementHash: document.Requirements[0].Hash,
		ObligationHash:  document.Requirements[0].Obligations[0].Hash,
		PlanHash:        plan.Hash, StartedAt: now, CompletedAt: now,
	}
	bundle := &evidence.Bundle{
		RunID: "run", AttemptID: "attempt-001", CreatedAt: now, Root: root,
		HeadCommit: "head", BaseCommit: "base", IRHash: document.Hash,
		Document: document, VerificationPlan: plan,
		RepositoryState: &git.State{
			Root: root, BaseCommit: "base", HeadCommit: "head",
			DiffHash: strings.Repeat("d", 64), DiffPatch: "literal-secret diff",
		},
		ProviderLogs: map[string]provider.Result{
			"REQ-001/OBL-001/check": {
				Provider: "command", ProviderVersion: "1.0.0", Status: "completed",
				Stdout: "literal-secret stdout", Stderr: "API_TOKEN=visible", Evidence: []provider.Evidence{record},
			},
		},
		Run: verdict.RunResult{
			Verdict: verdict.Pass,
			Requirements: []verdict.RequirementResult{{
				ID: "REQ-001", Title: "Requirement", Priority: "required", Verdict: verdict.Pass,
				Obligations: []verdict.ObligationResult{{
					ID: "OBL-001", Statement: "It passes.", Required: true,
					Verdict: verdict.Pass, Evidence: []provider.Evidence{record},
				}},
			}},
		},
	}
	if err := store.WriteAttempt(bundle); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"report.txt":       "literal-secret report\n",
		"report.json":      `{"run":"run"}`,
		"report.junit.xml": `<testsuites/>`,
	} {
		if err := store.WriteReport("run", name, []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Finalize(bundle); err != nil {
		t.Fatal(err)
	}

	runDir := store.Dir("run")
	for _, relative := range []string{
		"compiled-intent.json", "verification-plan.json", "repository-state.json", "diff.patch",
		"attempts/attempt-001/evidence.json", "attempts/attempt-001/verdict.json",
		"attempts/attempt-001/logs/REQ-001_OBL-001_check.stdout",
		"attempts/attempt-001/logs/REQ-001_OBL-001_check.stderr",
		"final-verdict.json", "manifest.json", "report.txt", "report.json", "report.junit.xml",
	} {
		data, err := os.ReadFile(filepath.Join(runDir, relative))
		if err != nil {
			t.Fatalf("%s: %v", relative, err)
		}
		if strings.Contains(string(data), "literal-secret") || strings.Contains(string(data), "visible") {
			t.Fatalf("%s was not redacted: %s", relative, data)
		}
	}
	manifestRaw, err := os.ReadFile(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest evidence.Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, artifact := range manifest.Artifacts {
		if artifact.Path == "manifest.json" || artifact.Path == "final-verdict.json" {
			t.Fatalf("hash cycle: %+v", artifact)
		}
		content, err := os.ReadFile(filepath.Join(runDir, filepath.FromSlash(artifact.Path)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(content)
		if artifact.SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("%s hash mismatch", artifact.Path)
		}
	}
	sum := sha256.Sum256(manifestRaw)
	if bundle.ManifestHash != hex.EncodeToString(sum[:]) {
		t.Fatal("final manifest hash mismatch")
	}
	if err := store.WriteAgentLog("run", "attempt-001", "stdout", []byte("late")); err == nil {
		t.Fatal("finalized run was mutable")
	}
}

func TestRepairArtifactsAndUnsafeEvidencePaths(t *testing.T) {
	root := t.TempDir()
	store, err := evidence.NewStore(root, ".intentci/runs")
	if err != nil {
		t.Fatal(err)
	}
	packet := repair.Packet{
		RunID: "run", Verdict: verdict.Fail, Summary: "failed",
		Failures: []repair.Failure{{Requirement: "REQ", Obligation: "OBL", Verdict: verdict.Fail, Reason: "x"}},
		Attempt:  1, MaxAttempts: 2,
	}
	path, err := store.WriteRepairPacketForAttempt("run", "attempt-001", packet)
	if err != nil || !strings.HasSuffix(path, "repair-packet.json") {
		t.Fatalf("%s %v", path, err)
	}
	for _, stream := range []string{"stdout", "stderr"} {
		if err := store.WriteAgentLog("run", "attempt-001", stream, []byte(stream)); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"patch-before.diff", "patch-after.diff", "agent-exit.json"} {
		if err := store.WriteRepairArtifact("run", "attempt-001", name, []byte(name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.WriteRepairPacketForAttempt("../run", "attempt-001", packet); err == nil {
		t.Fatal("unsafe run id accepted")
	}
	if err := store.WriteAgentLog("run", "../attempt", "stdout", nil); err == nil {
		t.Fatal("unsafe attempt id accepted")
	}
	if err := store.WriteAgentLog("run", "attempt-001", "other", nil); err == nil {
		t.Fatal("unsafe stream accepted")
	}
	if err := store.WriteRepairArtifact("run", "attempt-001", "other", nil); err == nil {
		t.Fatal("unsafe artifact accepted")
	}
	if err := store.WriteReport("run", "other", nil); err == nil {
		t.Fatal("unsafe report accepted")
	}

	outside := t.TempDir()
	link := filepath.Join(root, ".intentci", "linked-runs")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := evidence.NewStore(root, ".intentci/linked-runs"); err == nil {
		t.Fatal("symlink evidence escape accepted")
	}
}

package evidence

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	repogit "github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

func validTestBundle(t *testing.T, runID string) *Bundle {
	t.Helper()
	document := &ir.Document{SchemaVersion: 1, Project: "project", Requirements: []ir.Requirement{}}
	if err := document.ComputeHashes(); err != nil {
		t.Fatal(err)
	}
	plan, err := ir.BuildVerificationPlan(document, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &Bundle{
		RunID: runID, AttemptID: "attempt-001", CreatedAt: time.Now().UTC(),
		Document: document, VerificationPlan: plan,
		RepositoryState: &repogit.State{DiffPatch: "patch"},
		ProviderLogs: map[string]provider.Result{
			"provider": {Stdout: "out", Stderr: "err"},
		},
		Run: verdict.RunResult{Verdict: verdict.Pass},
	}
}

func TestNewStoreAllFailures(t *testing.T) {
	if _, err := NewStore(t.TempDir(), ""); err == nil {
		t.Fatal("empty evidence directory accepted")
	}
	oldMkdir, oldAbs, oldEval := mkdirAll, absolutePath, evaluateSymlinks
	defer func() {
		mkdirAll, absolutePath, evaluateSymlinks = oldMkdir, oldAbs, oldEval
	}()
	root := t.TempDir()
	mkdirAll = func(string, os.FileMode) error { return errors.New("mkdir") }
	if _, err := NewStore(root, "runs"); err == nil {
		t.Fatal("relative mkdir failure ignored")
	}
	if _, err := NewStore(root, filepath.Join(root, "absolute")); err == nil {
		t.Fatal("absolute mkdir failure ignored")
	}
	mkdirAll = oldMkdir
	absolutePath = func(string) (string, error) { return "", errors.New("absolute") }
	if _, err := NewStore(root, filepath.Join(root, "absolute")); err == nil {
		t.Fatal("absolute path failure ignored")
	}
	absolutePath = oldAbs
	evaluateSymlinks = func(string) (string, error) { return "", errors.New("symlinks") }
	if _, err := NewStore(root, filepath.Join(root, "absolute")); err == nil {
		t.Fatal("symlink evaluation failure ignored")
	}
}

func TestWriteAttemptStageFailures(t *testing.T) {
	store, err := NewStore(t.TempDir(), "runs")
	if err != nil {
		t.Fatal(err)
	}
	oldWrite := writeFile
	defer func() { writeFile = oldWrite }()
	for failure := 1; failure <= 9; failure++ {
		calls := 0
		writeFile = func(path string, content []byte, mode os.FileMode) error {
			calls++
			if calls == failure {
				return errors.New("stage")
			}
			return oldWrite(path, content, mode)
		}
		bundle := validTestBundle(t, "stage-"+string(rune('a'+failure)))
		if err := store.WriteAttempt(bundle); err == nil {
			t.Fatalf("write stage %d failure ignored", failure)
		}
	}
	writeFile = oldWrite

	oldMkdir := mkdirAll
	defer func() { mkdirAll = oldMkdir }()
	for index, suffix := range []string{"run-mkdir", filepath.Join("logs"), filepath.Join("artifacts")} {
		runID := "mkdir-case-" + string(rune('a'+index))
		mkdirAll = func(path string, mode os.FileMode) error {
			if (suffix == "run-mkdir" && filepath.Base(path) == runID) ||
				(suffix != "run-mkdir" && strings.HasSuffix(path, suffix)) {
				return errors.New("mkdir")
			}
			return oldMkdir(path, mode)
		}
		if err := store.WriteAttempt(&Bundle{
			RunID: runID, AttemptID: "attempt", Run: verdict.RunResult{Verdict: verdict.Pass},
		}); err == nil {
			t.Fatalf("%s mkdir failure ignored", suffix)
		}
	}
}

func TestWriteAttemptValidationFailures(t *testing.T) {
	store, err := NewStore(t.TempDir(), "runs")
	if err != nil {
		t.Fatal(err)
	}
	for _, bundle := range []*Bundle{
		{RunID: "../run", Run: verdict.RunResult{Verdict: verdict.Pass}},
		{RunID: "run", AttemptID: "../attempt", Run: verdict.RunResult{Verdict: verdict.Pass}},
		{RunID: "bad-run", Run: verdict.RunResult{Verdict: "invalid"}},
		{RunID: "bad-requirement", Run: verdict.RunResult{Verdict: verdict.Pass, Requirements: []verdict.RequirementResult{{ID: "R", Verdict: "invalid"}}}},
		{RunID: "bad-obligation", Run: verdict.RunResult{Verdict: verdict.Pass, Requirements: []verdict.RequirementResult{{
			ID: "R", Verdict: verdict.Pass, Obligations: []verdict.ObligationResult{{ID: "O", Verdict: "invalid"}},
		}}}},
		{RunID: "bad-document", Document: &ir.Document{SchemaVersion: 2, Project: "p", Hash: "set"}, Run: verdict.RunResult{Verdict: verdict.Pass}},
		{RunID: "bad-plan", VerificationPlan: &ir.VerificationPlan{SchemaVersion: 2}, Run: verdict.RunResult{Verdict: verdict.Pass}},
		{RunID: "bad-evidence", ProviderLogs: map[string]provider.Result{"p": {Evidence: []provider.Evidence{{ID: "bad"}}}}, Run: verdict.RunResult{Verdict: verdict.Pass}},
	} {
		if err := store.WriteAttempt(bundle); err == nil {
			t.Fatalf("invalid bundle accepted: %+v", bundle)
		}
	}
	badHash := &ir.Document{SchemaVersion: 1, Project: "p", Requirements: []ir.Requirement{{
		ID: "R", Obligations: []ir.Obligation{{Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
			Extra: map[string]any{"bad": make(chan int)},
		}}}},
	}}}
	if err := store.WriteAttempt(&Bundle{
		RunID: "bad-hash", Document: badHash, Run: verdict.RunResult{Verdict: verdict.Pass},
	}); err == nil {
		t.Fatal("document hash failure ignored")
	}
	if err := store.WriteAttempt(&Bundle{
		RunID: "bad-marshal", ProviderLogs: map[string]provider.Result{
			"p": {Extra: map[string]any{"bad": make(chan int)}},
		}, Run: verdict.RunResult{Verdict: verdict.Pass},
	}); err == nil {
		t.Fatal("bundle marshal failure ignored")
	}
}

func TestReportFinalizeLoadAndControllerErrors(t *testing.T) {
	store, err := NewStore(t.TempDir(), "runs")
	if err != nil {
		t.Fatal(err)
	}
	for _, runID := range []string{"../run", ""} {
		if err := store.WriteReport(runID, "report.txt", nil); err == nil {
			t.Fatal("invalid report run id")
		}
		if err := store.Finalize(&Bundle{RunID: runID}); err == nil {
			t.Fatal("invalid finalize run id")
		}
		if _, err := store.Load(runID); err == nil {
			t.Fatal("invalid load run id")
		}
		if err := store.WriteRepairPacket(runID, nil); err == nil {
			t.Fatal("invalid packet run id")
		}
		if _, err := store.WriteRepairPacketForAttempt(runID, "attempt", nil); err == nil {
			t.Fatal("invalid attempt packet run id")
		}
		if err := store.WriteAgentLog(runID, "attempt", "stdout", nil); err == nil {
			t.Fatal("invalid agent log run id")
		}
		if err := store.WriteRepairArtifact(runID, "attempt", "agent-exit.json", nil); err == nil {
			t.Fatal("invalid repair artifact run id")
		}
	}
	if _, err := store.WriteRepairPacketForAttempt("run", "../attempt", nil); err == nil {
		t.Fatal("invalid packet attempt id")
	}
	if err := store.WriteRepairArtifact("run", "../attempt", "agent-exit.json", nil); err == nil {
		t.Fatal("invalid repair artifact attempt id")
	}
	if _, err := store.WriteRepairPacketForAttempt("run", "attempt", map[string]any{"bad": true}); err == nil {
		t.Fatal("invalid repair packet schema")
	}
	packet := map[string]any{
		"run_id": "run", "verdict": "fail", "failures": []any{},
		"attempt": 1, "max_attempts": 2,
	}
	packetPath := filepath.Join(store.Dir("packet-conflict"), "attempts", "attempt", "repair-packet.json")
	if err := os.MkdirAll(filepath.Dir(packetPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, []byte("different"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.WriteRepairPacketForAttempt("packet-conflict", "attempt", packet); err == nil {
		t.Fatal("repair packet conflict ignored")
	}

	finalized := validTestBundle(t, "finalized")
	if err := store.WriteBundle(finalized); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []func() error{
		func() error { return store.WriteReport("finalized", "report.txt", nil) },
		func() error { return store.WriteRepairPacket("finalized", nil) },
		func() error {
			_, err := store.WriteRepairPacketForAttempt("finalized", "attempt", nil)
			return err
		},
		func() error { return store.WriteAgentLog("finalized", "attempt", "stdout", nil) },
		func() error { return store.WriteRepairArtifact("finalized", "attempt", "agent-exit.json", nil) },
	} {
		if err := operation(); err == nil {
			t.Fatal("finalized run mutated")
		}
	}
}

func TestLowLevelAtomicAndPathFailures(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, "runs")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.writeJSONImmutable(filepath.Join(store.Root, "bad.json"), make(chan int)); err == nil {
		t.Fatal("immutable JSON marshal failure ignored")
	}
	if err := store.writeJSONAtomic(filepath.Join(store.Root, "bad-atomic.json"), make(chan int)); err == nil {
		t.Fatal("atomic JSON marshal failure ignored")
	}
	outside := filepath.Join(root, "outside")
	if err := store.writeImmutable(outside, nil); err == nil {
		t.Fatal("immutable escape accepted")
	}
	if err := store.writeAtomic(outside, nil); err == nil {
		t.Fatal("atomic escape accepted")
	}
	path := filepath.Join(store.Root, "same")
	if err := store.writeImmutable(path, []byte("same")); err != nil {
		t.Fatal(err)
	}
	if err := store.writeImmutable(path, []byte("same")); err != nil {
		t.Fatal(err)
	}
	if err := store.writeImmutable(path, []byte("different")); err == nil {
		t.Fatal("immutable replacement accepted")
	}

	oldRead, oldMkdir, oldWrite, oldRename := readFile, mkdirAll, writeFile, renameFile
	defer func() {
		readFile, mkdirAll, writeFile, renameFile = oldRead, oldMkdir, oldWrite, oldRename
	}()
	readFile = func(string) ([]byte, error) { return nil, errors.New("read") }
	if err := store.writeImmutable(filepath.Join(store.Root, "read-error"), nil); err == nil {
		t.Fatal("read error ignored")
	}
	readFile = oldRead
	mkdirAll = func(string, os.FileMode) error { return errors.New("mkdir") }
	if err := store.writeAtomic(filepath.Join(store.Root, "mkdir-error"), nil); err == nil {
		t.Fatal("mkdir error ignored")
	}
	mkdirAll = oldMkdir
	writeFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	if err := store.writeAtomic(filepath.Join(store.Root, "write-error"), nil); err == nil {
		t.Fatal("write error ignored")
	}
	writeFile = oldWrite
	renameFile = func(string, string) error { return errors.New("rename") }
	if err := store.writeAtomic(filepath.Join(store.Root, "rename-error"), nil); err == nil {
		t.Fatal("rename error ignored")
	}
}

func TestJSONRedactionPreservesNonStringValues(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, "runs")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("BOOLEAN_TOKEN", "true")
	store.RedactPatterns = []string{"*TOKEN*"}
	path := filepath.Join(store.Root, "structured.json")
	value := map[string]any{
		"boolean": true,
		"number":  42,
		"secret":  "true",
		"nested":  []any{false, map[string]any{"value": "prefix true suffix"}},
	}
	if err := store.writeJSONAtomic(path, value); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("redaction corrupted JSON: %v\n%s", err, raw)
	}
	if decoded["boolean"] != true || decoded["number"] != float64(42) {
		t.Fatalf("non-string JSON values changed: %#v", decoded)
	}
	if decoded["secret"] != "[REDACTED]" ||
		decoded["nested"].([]any)[1].(map[string]any)["value"] != "prefix [REDACTED] suffix" {
		t.Fatalf("JSON strings were not redacted: %#v", decoded)
	}
}

func TestSafePathAndManifestFailures(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, filepath.Join(root, "absolute-runs"))
	if err != nil {
		t.Fatal(err)
	}
	oldRel, oldEval, oldStat, oldRead := relativePath, evaluateSymlinks, statFile, readFile
	defer func() {
		relativePath, evaluateSymlinks, statFile, readFile = oldRel, oldEval, oldStat, oldRead
	}()
	relativePath = func(string, string) (string, error) { return "", errors.New("relative") }
	if _, err := store.safePath(filepath.Join(store.Root, "x")); err == nil {
		t.Fatal("relative error ignored")
	}
	relativePath = oldRel
	relativeStore := &Store{
		Root: filepath.Join(root, "relative-runs"), repoRoot: root, relativeRoot: "../escape",
	}
	if err := os.MkdirAll(relativeStore.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := relativeStore.safePath(filepath.Join(relativeStore.Root, "x")); err == nil {
		t.Fatal("unsafe relative store root accepted")
	}
	evaluateSymlinks = func(string) (string, error) { return "", errors.New("symlink") }
	if _, err := store.safePath(filepath.Join(store.Root, "x")); err == nil {
		t.Fatal("root symlink error ignored")
	}
	evaluateSymlinks = oldEval

	statFile = func(string) (os.FileInfo, error) { return nil, errors.New("stat") }
	if err := store.ensureOpen("run"); err == nil {
		t.Fatal("stat error ignored")
	}
	statFile = oldStat

	missing := filepath.Join(root, "missing")
	if _, err := hashArtifacts(missing); err == nil {
		t.Fatal("walk error ignored")
	}
	manifestRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(manifestRoot, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b", "manifest.json", "final-verdict.json", ".x.tmp-y"} {
		if err := os.WriteFile(filepath.Join(manifestRoot, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("a", filepath.Join(manifestRoot, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := hashArtifacts(manifestRoot); err == nil {
		t.Fatal("artifact symlink accepted")
	}
	if err := os.Remove(filepath.Join(manifestRoot, "link")); err != nil {
		t.Fatal(err)
	}
	artifacts, err := hashArtifacts(manifestRoot)
	if err != nil || len(artifacts) != 2 || artifacts[0].Path != "a" {
		t.Fatalf("%+v %v", artifacts, err)
	}
	readFile = func(path string) ([]byte, error) {
		if filepath.Base(path) == "a" {
			return nil, errors.New("read")
		}
		return oldRead(path)
	}
	if _, err := hashArtifacts(manifestRoot); err == nil {
		t.Fatal("artifact read error ignored")
	}
}

func TestOperationSafePathFailures(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, "runs")
	if err != nil {
		t.Fatal(err)
	}
	oldRel := relativePath
	defer func() { relativePath = oldRel }()
	calls := 0
	relativePath = func(base, target string) (string, error) {
		calls++
		if calls == 2 {
			return "", errors.New("second safe path")
		}
		return oldRel(base, target)
	}
	if err := store.WriteAttempt(&Bundle{
		RunID: "run", Run: verdict.RunResult{Verdict: verdict.Pass},
	}); err == nil {
		t.Fatal("run directory safe-path failure ignored")
	}

	relativePath = func(string, string) (string, error) { return "", errors.New("safe path") }
	if err := store.Finalize(&Bundle{RunID: "run"}); err == nil {
		t.Fatal("finalize safe-path failure ignored")
	}
	if _, err := store.LoadLatest(); err == nil {
		t.Fatal("latest safe-path failure ignored")
	}
	if _, err := store.Load("run"); err == nil {
		t.Fatal("load safe-path failure ignored")
	}
}

func TestFinalizeArtifactAndWriteFailures(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, "runs")
	if err != nil {
		t.Fatal(err)
	}
	runDir := store.Dir("symlink-run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(runDir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := store.Finalize(&Bundle{RunID: "symlink-run"}); err == nil {
		t.Fatal("manifest symlink accepted")
	}

	conflictDir := store.Dir("manifest-conflict")
	if err := os.MkdirAll(conflictDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, "manifest.json"), []byte("different"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.Finalize(&Bundle{RunID: "manifest-conflict"}); err == nil {
		t.Fatal("manifest conflict ignored")
	}

	oldWrite := writeFile
	defer func() { writeFile = oldWrite }()
	calls := 0
	writeFile = func(path string, content []byte, mode os.FileMode) error {
		calls++
		if calls == 2 {
			return errors.New("final")
		}
		return oldWrite(path, content, mode)
	}
	if err := store.Finalize(&Bundle{RunID: "final-write"}); err == nil {
		t.Fatal("final verdict write failure ignored")
	}
}

func TestNamesAndIdentifiers(t *testing.T) {
	for _, value := range []string{"", ".", "..", "a/b", `a\b`} {
		if validateRunID(value) == nil || validateAttemptID(value) == nil {
			t.Fatal(value)
		}
	}
	if validateRunID("run") != nil || validateAttemptID("attempt") != nil {
		t.Fatal("valid identifiers rejected")
	}
	if safeName("") != "provider" || safeName("a/b") != "a_b" {
		t.Fatal("safe name")
	}
	keys := sortedProviderKeys(map[string]provider.Result{"b": {}, "a": {}})
	if strings.Join(keys, "") != "ab" {
		t.Fatal(keys)
	}
}

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/exitcode"
	repogit "github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/initcmd"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
	"github.com/hypertrial/intentci/internal/verify"
)

func TestV1CLIUsageValidationAndAdapterResolution(t *testing.T) {
	root := t.TempDir()
	if err := initcmd.Run(initcmd.Options{Root: root}); err != nil {
		t.Fatal(err)
	}
	initializeTestGit(t, root)
	oldGetwd := getwd
	defer func() { getwd = oldGetwd }()
	getwd = func() (string, error) { return root, nil }

	usageCases := [][]string{
		{"compile", "--format", "yaml"},
		{"compile", "--requirement", " "},
		{"compile", "--requirement", "MISSING"},
		{"verify", "--all", "--changed"},
		{"verify", "--all", "--max-parallel", "-1"},
		{"verify", "--all", "--requirement", " "},
		{"verify", "--all", "--obligation", " "},
		{"verify", "--all", "--provider", " "},
		{"explain", "REQ-001", "--format", "yaml"},
		{"repair", "--agent", "one", "--agent-command", "true"},
		{"repair", "--max-attempts", "0"},
		{"repair", "--max-attempts", "-1"},
		{"repair", "--requirement", " "},
		{"repair", "--agent", " "},
		{"repair", "--agent-command", " "},
		{"repair", "--agent", "Bad"},
		{"repair", "--agent", "missing"},
	}
	for _, args := range usageCases {
		var stdout, stderr bytes.Buffer
		if code := RunMain(args, &stdout, &stderr); code != exitcode.Usage {
			t.Fatalf("%v returned %d, want usage; stderr=%s", args, code, stderr.String())
		}
	}

	adapterDir := t.TempDir()
	adapter := filepath.Join(adapterDir, "intentci-agent-good")
	if err := os.WriteFile(adapter, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", adapterDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stdout, stderr bytes.Buffer
	if code := RunMain([]string{"repair", "--agent", "good", "--dry-run", "--max-attempts", "1"}, &stdout, &stderr); code != exitcode.Pass {
		t.Fatalf("resolved adapter returned %d: %s", code, stderr.String())
	}
	if code := RunMain([]string{"repair", "--agent-command", "true", "--max-attempts", "1"}, &stdout, &stderr); code != exitcode.Pass {
		t.Fatalf("host warning path returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "not a sandbox") {
		t.Fatalf("host execution warning missing: %s", stderr.String())
	}

	for input, want := range map[string]bool{
		"": false, "a": true, "a-0": true, "A": false, "_": false,
	} {
		if got := validAdapterName(input); got != want {
			t.Fatalf("validAdapterName(%q)=%t, want %t", input, got, want)
		}
	}
}

func initializeTestGit(t *testing.T, root string) {
	t.Helper()
	for _, arguments := range [][]string{
		{"init"},
		{"config", "user.email", "intentci@example.test"},
		{"config", "user.name", "IntentCI Test"},
		{"add", "."},
		{"commit", "-m", "initial"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
}

type fakeReportFile struct {
	name       string
	writeErr   error
	closeErr   error
	shortWrite bool
}

func (f *fakeReportFile) Name() string { return f.name }
func (f *fakeReportFile) Write(content []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	if f.shortWrite {
		return len(content) - 1, nil
	}
	return len(content), nil
}
func (f *fakeReportFile) Close() error { return f.closeErr }

func TestWriteReportFileAtomicFailures(t *testing.T) {
	root := t.TempDir()
	if err := writeReportFile(root, "reports/result.txt", []byte("ok")); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(root, "reports", "result.txt")); err != nil || string(content) != "ok" {
		t.Fatalf("report=%q err=%v", content, err)
	}
	if err := writeReportFile(root, "../escape", nil); err == nil {
		t.Fatal("report traversal accepted")
	}

	oldMkdir, oldCreate := makeReportDirs, createReportTemp
	oldRemove, oldRename := removeReportFile, renameReportFile
	defer func() {
		makeReportDirs, createReportTemp = oldMkdir, oldCreate
		removeReportFile, renameReportFile = oldRemove, oldRename
	}()
	makeReportDirs = func(string, os.FileMode) error { return errors.New("mkdir") }
	if err := writeReportFile(root, filepath.Join(root, "mkdir"), nil); err == nil {
		t.Fatal("mkdir failure ignored")
	}
	makeReportDirs = oldMkdir
	createReportTemp = func(string, string) (reportTempFile, error) {
		return nil, errors.New("create")
	}
	if err := writeReportFile(root, filepath.Join(root, "create"), nil); err == nil {
		t.Fatal("create failure ignored")
	}
	createReportTemp = func(string, string) (reportTempFile, error) {
		return &fakeReportFile{name: filepath.Join(root, "write"), writeErr: errors.New("write")}, nil
	}
	if err := writeReportFile(root, filepath.Join(root, "write-target"), nil); err == nil {
		t.Fatal("write failure ignored")
	}
	createReportTemp = func(string, string) (reportTempFile, error) {
		return &fakeReportFile{name: filepath.Join(root, "short"), shortWrite: true}, nil
	}
	if err := writeReportFile(root, filepath.Join(root, "short-target"), []byte("content")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write returned %v", err)
	}
	createReportTemp = func(string, string) (reportTempFile, error) {
		return &fakeReportFile{name: filepath.Join(root, "close"), closeErr: errors.New("close")}, nil
	}
	if err := writeReportFile(root, filepath.Join(root, "close-target"), nil); err == nil {
		t.Fatal("close failure ignored")
	}
	createReportTemp = func(string, string) (reportTempFile, error) {
		return &fakeReportFile{name: filepath.Join(root, "rename")}, nil
	}
	renameReportFile = func(string, string) error { return errors.New("rename") }
	if err := writeReportFile(root, filepath.Join(root, "rename-target"), nil); err == nil {
		t.Fatal("rename failure ignored")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("writer") }

func TestVerifyOutputSecurityAndWriterFailure(t *testing.T) {
	root := t.TempDir()
	if err := initcmd.Run(initcmd.Options{Root: root}); err != nil {
		t.Fatal(err)
	}
	oldGetwd := getwd
	defer func() { getwd = oldGetwd }()
	getwd = func() (string, error) { return root, nil }

	var stdout, stderr bytes.Buffer
	if code := RunMain([]string{"verify", "--all", "--no-git", "--output", "../escape"}, &stdout, &stderr); code != exitcode.SecurityBoundary {
		t.Fatalf("report escape returned %d: %s", code, stderr.String())
	}
	cmd := newVerifyCmd()
	cmd.SetArgs([]string{"--all", "--no-git"})
	cmd.SetOut(failingWriter{})
	cmd.SetErr(io.Discard)
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("report writer failure ignored")
	}

	oldRunVerification := runVerification
	defer func() { runVerification = oldRunVerification }()
	runVerification = func(context.Context, verify.Options) (*verify.Outcome, error) {
		return &verify.Outcome{
			Bundle: &evidence.Bundle{
				RunID: "failed", CreatedAt: time.Now().UTC(),
				Run: verdict.RunResult{Verdict: verdict.Fail, Requirements: []verdict.RequirementResult{}},
			},
			ExitCode: exitcode.Fail,
		}, nil
	}
	stdout.Reset()
	stderr.Reset()
	if code := RunMain([]string{"verify", "--all", "--no-git"}, &stdout, &stderr); code != exitcode.Fail {
		t.Fatalf("failed verdict returned %d: %s", code, stderr.String())
	}
}

func TestStatusAllVerdictsAndCompileFailure(t *testing.T) {
	root := t.TempDir()
	if err := initcmd.Run(initcmd.Options{Root: root, NoExample: true}); err != nil {
		t.Fatal(err)
	}
	statuses := []string{
		verdict.Pass, verdict.Fail, verdict.Unproven, verdict.Uncertain,
		verdict.ReviewRequired, verdict.Error, verdict.Skipped, "",
	}
	results := make([]verdict.RequirementResult, 0, len(statuses)-1)
	for index, status := range statuses {
		id := fmt.Sprintf("REQ-%03d", index+1)
		requirement := fmt.Sprintf(`---
id: %s
title: %s
status: active
priority: required
---
# Intent
Intent.
# Obligations
`+"```yaml"+`
- id: OBL-001
  statement: Pass.
  required: true
  verify:
    provider: command
    run: "true"
    result: {equals: 0}
`+"```"+`
`, id, id)
		if err := os.WriteFile(filepath.Join(root, ".intentci", "requirements", id+".md"), []byte(requirement), 0o644); err != nil {
			t.Fatal(err)
		}
		if status != "" {
			results = append(results, verdict.RequirementResult{
				ID: id, Title: id, Priority: "required", Verdict: status,
				Obligations: []verdict.ObligationResult{},
			})
		}
	}
	store, err := evidence.NewStore(root, ".intentci/runs")
	if err != nil {
		t.Fatal(err)
	}
	bundle := &evidence.Bundle{
		RunID: "status-run", CreatedAt: time.Now().UTC(), HeadCommit: "123456789",
		RepositoryState: &repogit.State{
			BaseCommit: "987654321", WorkingTreeDirty: true, ChangedFiles: []string{"a", "b"},
		},
		Run: verdict.RunResult{Verdict: verdict.Error, Requirements: results},
	}
	if err := store.WriteBundle(bundle); err != nil {
		t.Fatal(err)
	}
	oldGetwd := getwd
	defer func() { getwd = oldGetwd }()
	getwd = func() (string, error) { return root, nil }
	var stdout, stderr bytes.Buffer
	if code := RunMain([]string{"status"}, &stdout, &stderr); code != exitcode.Pass {
		t.Fatalf("status returned %d: %s", code, stderr.String())
	}
	for _, expected := range []string{
		"Requirements: 8 active", "Verified:     1", "Failed:        1",
		"Unproven:      2", "Uncertain:     1", "Review:        1",
		"Errors:        1", "Skipped:       1", "Base:", "Dirty:         true",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("status missing %q:\n%s", expected, stdout.String())
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".intentci", "requirements", "REQ-001.md"), []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := RunMain([]string{"status"}, &stdout, &stderr); code != exitcode.CompileFailed {
		t.Fatalf("invalid status contract returned %d: %s", code, stderr.String())
	}
}

func TestSchemaAndExplainJSONIdentifiers(t *testing.T) {
	for _, name := range []string{"report", "plan"} {
		var stdout, stderr bytes.Buffer
		if code := RunMain([]string{"schema", name}, &stdout, &stderr); code != exitcode.Pass || stdout.Len() == 0 {
			t.Fatalf("schema %s returned %d: %s", name, code, stderr.String())
		}
	}
	record := provider.Evidence{ID: "EVIDENCE", VerifierID: "verifier"}
	bundle := &evidence.Bundle{
		RunID: "run",
		Run: verdict.RunResult{Requirements: []verdict.RequirementResult{{
			ID: "REQ", Obligations: []verdict.ObligationResult{{
				ID: "OBL", Evidence: []provider.Evidence{record},
			}},
		}}},
		ProviderLogs: map[string]provider.Result{
			"REQ/OBL/log": {Status: "completed"},
			"direct":      {Status: "completed"},
		},
	}
	for _, id := range []string{"run", "REQ", "OBL", "EVIDENCE", "verifier", "log", "direct"} {
		if _, found := explainJSONValue(bundle, id); !found {
			t.Fatalf("identifier %q not found", id)
		}
	}
	if _, found := explainJSONValue(bundle, "missing"); found {
		t.Fatal("missing identifier found")
	}
}

func TestRepairVerificationErrorReturnsInternal(t *testing.T) {
	root := t.TempDir()
	if err := initcmd.Run(initcmd.Options{Root: root}); err != nil {
		t.Fatal(err)
	}
	initializeTestGit(t, root)
	oldGetwd := getwd
	oldRunVerification := runVerification
	defer func() {
		getwd = oldGetwd
		runVerification = oldRunVerification
	}()
	getwd = func() (string, error) { return root, nil }
	runVerification = func(context.Context, verify.Options) (*verify.Outcome, error) {
		return &verify.Outcome{ExitCode: exitcode.Internal}, errors.New("verification")
	}
	var stdout, stderr bytes.Buffer
	if code := RunMain([]string{"repair", "--changed", "--dry-run", "--max-attempts", "1"}, &stdout, &stderr); code != exitcode.Internal {
		t.Fatalf("repair verification error returned %d: %s", code, stderr.String())
	}
}

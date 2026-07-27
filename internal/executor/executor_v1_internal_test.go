package executor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

type sequenceProvider struct {
	mutex   sync.Mutex
	results []provider.Result
	calls   int
}

func (p *sequenceProvider) Name() string                                   { return "sequence" }
func (p *sequenceProvider) Version() string                                { return "1.2.3" }
func (p *sequenceProvider) Validate(ir.ProviderSpec) []provider.Diagnostic { return nil }
func (p *sequenceProvider) Execute(context.Context, provider.Request) provider.Result {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	index := p.calls
	p.calls++
	if index >= len(p.results) {
		index = len(p.results) - 1
	}
	return p.results[index]
}

func TestSchedulerRetryDependenciesAndManualReview(t *testing.T) {
	failed := false
	passed := true
	implementation := &sequenceProvider{results: []provider.Result{
		{Provider: "sequence", ProviderVersion: "1.2.3", Status: "error", Stdout: "first", Stderr: "bad", Evidence: []provider.Evidence{{ID: "e", Class: "deterministic", Summary: "retry", Passed: &failed}}},
		{Provider: "sequence", ProviderVersion: "1.2.3", Status: "completed", Stdout: "second", Evidence: []provider.Evidence{{ID: "e", Class: "deterministic", Summary: "ok", Passed: &passed}}},
	}}
	registry := provider.NewRegistry(implementation)
	requirements := []ir.Requirement{{
		ID: "REQ", Priority: "required", Hash: strings.Repeat("a", 64),
		Obligations: []ir.Obligation{
			{
				ID: "FIRST", Required: true, Hash: strings.Repeat("b", 64),
				Retry:  ir.Retry{Attempts: 2},
				Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "sequence", ID: "retry"}},
			},
			{
				ID: "SECOND", Required: true, DependsOn: []string{"FIRST"},
				Hash: strings.Repeat("c", 64), ManualReview: true,
				Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "sequence", ID: "review"}},
			},
		},
	}}
	leaves, results := Run(context.Background(), requirements, Options{
		Root: t.TempDir(), Config: config.Default(), Registry: registry,
		RunID: "run", AttemptID: "attempt-001", HeadCommit: "head", BaseCommit: "base",
		DiffHash: "diff", IRHash: "ir", PlanHash: "plan", NoCache: true,
	})
	if implementation.calls != 3 || results[0].Verdict != verdict.ReviewRequired {
		t.Fatalf("calls=%d results=%+v", implementation.calls, results)
	}
	retry := leaves["REQ"]["FIRST/retry"]
	if len(retry.Evidence) != 2 || !strings.Contains(retry.Stdout, "provider attempt 1") ||
		retry.Evidence[0].ID == retry.Evidence[1].ID {
		t.Fatalf("%+v", retry)
	}
}

func TestSchedulerCancellationConflictsAndSelectors(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	requirement := ir.Requirement{
		ID: "REQ", Priority: "required",
		Obligations: []ir.Obligation{{
			ID: "O", Required: true,
			Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "command", ID: "c", Run: "true"}},
		}},
	}
	_, results := Run(cancelled, []ir.Requirement{requirement}, Options{
		Root: t.TempDir(), Config: config.Default(), RunID: "run", NoCache: true,
	})
	if results[0].Verdict != verdict.Error {
		t.Fatal(results)
	}

	if !outputPatternsConflict("build/**", "build/file") ||
		!outputPatternsConflict("same", "same") ||
		outputPatternsConflict("build/**", "dist/**") {
		t.Fatal("output conflict detection")
	}
	jobs := []job{
		{fullKey: "a", spec: ir.ProviderSpec{Outputs: []string{"build/**"}}},
		{fullKey: "b", spec: ir.ProviderSpec{Outputs: []string{"build/file"}}},
		{fullKey: "c", spec: ir.ProviderSpec{Exclusive: true}},
	}
	if batch := compatibleBatch(jobs, 3); len(batch) != 1 || batch[0].fullKey != "a" {
		t.Fatalf("%+v", batch)
	}
	if batch := compatibleBatch(jobs[2:], 1); len(batch) != 1 || batch[0].fullKey != "c" {
		t.Fatalf("%+v", batch)
	}

	otherPlatform := "linux"
	if runtime.GOOS == "linux" {
		otherPlatform = "darwin"
	}
	requirement.Obligations[0].Platforms = []string{otherPlatform}
	_, results = Run(context.Background(), []ir.Requirement{requirement}, Options{
		Root: t.TempDir(), Config: config.Default(), ProviderID: "other", RunID: "run", NoCache: true,
	})
	if results[0].Verdict != verdict.Unproven {
		t.Fatal(results)
	}
}

func TestArtifactsOutputsAndCacheReuse(t *testing.T) {
	passed := true
	root := t.TempDir()
	evidenceDir := filepath.Join(root, ".intentci", "runs", "run", "attempts", "attempt-001", "artifacts")
	requirement := ir.Requirement{
		ID: "REQ", Priority: "required", Hash: strings.Repeat("a", 64),
		Obligations: []ir.Obligation{{
			ID: "O", Required: true, Hash: strings.Repeat("b", 64),
			Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
				Provider: "command", ID: "build",
				Run:     "mkdir -p build && printf artifact > build/out.txt && printf ok",
				Result:  map[string]any{"equals": 0, "stdout": map[string]any{"contains": "ok"}},
				Outputs: []string{"build/out.txt"}, Artifacts: []string{"build/*.txt"},
			}},
		}},
	}
	leaves, results := Run(context.Background(), []ir.Requirement{requirement}, Options{
		Root: root, Config: config.Default(), RunID: "run", AttemptID: "attempt-001",
		EvidenceDir: evidenceDir, HeadCommit: "head", BaseCommit: "base", DiffHash: "diff",
		IRHash: "ir", PlanHash: "plan", CacheDir: filepath.Join(root, "cache"),
	})
	record := leaves["REQ"]["O/build"].Evidence[0]
	if results[0].Verdict != verdict.Pass || len(record.Artifacts) != 1 {
		t.Fatalf("%+v %+v", results, record)
	}
	if _, err := os.Stat(filepath.Join(evidenceDir, "REQ", "O", "build", "build", "out.txt")); err != nil {
		t.Fatal(err)
	}

	requirement.Obligations[0].Verify.Provider.Outputs = []string{"missing.txt"}
	requirement.Obligations[0].Verify.Provider.Artifacts = nil
	_, results = Run(context.Background(), []ir.Requirement{requirement}, Options{
		Root: root, Config: config.Default(), RunID: "missing", NoCache: true,
	})
	if results[0].Verdict != verdict.Fail {
		t.Fatal(results)
	}

	cacheProvider := &sequenceProvider{results: []provider.Result{{
		Provider: "sequence", ProviderVersion: "1.2.3", Status: "completed",
		Evidence: []provider.Evidence{{ID: "cache", Class: "deterministic", Summary: "ok", Passed: &passed}},
	}}}
	cacheRequirement := requirement
	cacheRequirement.Obligations[0].Verify.Provider = &ir.ProviderSpec{Provider: "sequence", ID: "cache"}
	cacheDir := filepath.Join(root, "cache-reuse")
	options := Options{
		Root: root, Config: config.Default(), Registry: provider.NewRegistry(cacheProvider),
		RunID: "one", AttemptID: "attempt-001", HeadCommit: "head", DiffHash: "diff",
		IRHash: "ir", PlanHash: "plan", CacheDir: cacheDir,
	}
	_, _ = Run(context.Background(), []ir.Requirement{cacheRequirement}, options)
	options.RunID = "two"
	cached, cachedResults := Run(context.Background(), []ir.Requirement{cacheRequirement}, options)
	got := cached["REQ"]["O/cache"]
	if cacheProvider.calls != 1 || !got.FromCache || got.SourceEvidenceHash == "" ||
		cachedResults[0].Verdict != verdict.Pass {
		t.Fatalf("calls=%d result=%+v verdict=%+v", cacheProvider.calls, got, cachedResults)
	}
}

func TestExecutorHelperFailures(t *testing.T) {
	root := t.TempDir()
	if _, err := hashInputs(root, []string{"missing/**"}, nil, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadCache("", "x"); ok {
		t.Fatal("empty cache hit")
	}
	if _, ok := loadCache(root, "missing"); ok {
		t.Fatal("missing cache hit")
	}
	if err := saveCache("", "x", provider.Result{}); err != nil {
		t.Fatal(err)
	}
	if safeSegment("../") != "artifact" || safeSegment("normal") != "normal" {
		t.Fatal("safe segment")
	}
	var log strings.Builder
	appendAttemptLog(&log, 1, "")
	if log.Len() != 0 {
		t.Fatal(log.String())
	}
	if redactResult(provider.Result{Extra: map[string]any{"bad": make(chan int)}}, nil).Extra == nil {
		t.Fatal("marshal failure should preserve result")
	}
	t.Setenv("TOKEN", "secret")
	redacted := redactResult(provider.Result{Stdout: "secret"}, []string{"TOKEN"})
	if redacted.Stdout != "[REDACTED]" {
		t.Fatal(redacted)
	}
	if _, err := copyArtifact(filepath.Join(root, "missing"), filepath.Join(root, "out")); err == nil {
		t.Fatal("missing artifact source accepted")
	}
}

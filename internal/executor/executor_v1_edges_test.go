package executor

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/security"
	"github.com/hypertrial/intentci/internal/verdict"
)

type staticProvider struct {
	name    string
	version string
	result  provider.Result
	execute func(context.Context, provider.Request) provider.Result
}

func (p *staticProvider) Name() string    { return p.name }
func (p *staticProvider) Version() string { return p.version }
func (p *staticProvider) Validate(ir.ProviderSpec) []provider.Diagnostic {
	return nil
}
func (p *staticProvider) Execute(ctx context.Context, request provider.Request) provider.Result {
	if p.execute != nil {
		return p.execute(ctx, request)
	}
	return p.result
}

func edgeJob(id string) job {
	return job{
		req: ir.Requirement{ID: "REQ", Hash: "requirement"},
		obl: ir.Obligation{ID: "OBL", Hash: "obligation", Required: true},
		key: id, fullKey: "REQ/OBL/" + id,
		spec: ir.ProviderSpec{Provider: "static", ID: id},
	}
}

func TestRunDependenciesAndDefaultOptions(t *testing.T) {
	_, empty := Run(context.Background(), nil, Options{Root: t.TempDir(), NoCache: true})
	if len(empty) != 0 {
		t.Fatal(empty)
	}
	failed, passed := false, true
	implementation := &staticProvider{
		name: "static", version: "1",
		result: provider.Result{Status: "completed", Evidence: []provider.Evidence{{
			Class: "deterministic", Passed: &passed,
		}}},
	}
	registry := provider.NewRegistry(implementation)
	requirements := []ir.Requirement{
		{
			ID: "BASE", Priority: "required", Obligations: []ir.Obligation{{
				ID: "FAIL", Required: true, Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
					Provider: "static", ID: "base",
				}},
			}},
		},
		{
			ID: "DEPENDENT", Priority: "required", DependsOn: []string{"BASE"},
			Obligations: []ir.Obligation{
				{ID: "FIRST", Required: true, Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{Provider: "static", ID: "first"}}},
				{ID: "SECOND", Required: true, DependsOn: []string{"FIRST"}, Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
					Provider: "static", ID: "second", DependsOn: []string{"first"},
				}}},
			},
		},
	}
	implementation.execute = func(_ context.Context, request provider.Request) provider.Result {
		value := &passed
		if request.RequirementID == "BASE" || request.ObligationID == "FIRST" {
			value = &failed
		}
		return provider.Result{Status: "completed", Evidence: []provider.Evidence{{Class: "deterministic", Passed: value}}}
	}
	_, results := Run(context.Background(), requirements, Options{
		Root: t.TempDir(), Config: config.Default(), Registry: registry, NoCache: true,
	})
	if results[0].Verdict != verdict.Fail || results[1].Verdict != verdict.Fail ||
		results[1].Obligations[1].Verdict != verdict.Unproven {
		t.Fatalf("%+v", results)
	}

	direct := applyRequirementDependencies(
		[]ir.Requirement{{ID: "A", DependsOn: []string{"missing"}}, {ID: "B", DependsOn: []string{"A"}}},
		[]verdict.RequirementResult{{ID: "A", Verdict: verdict.Pass}, {ID: "B", Verdict: verdict.Pass}},
	)
	if direct[0].Verdict != verdict.Unproven || direct[1].Verdict != verdict.Unproven {
		t.Fatalf("%+v", direct)
	}
}

func TestNormalizeResultsAllStatuses(t *testing.T) {
	passed, failed := true, false
	confidence := 2.0
	implementation := &staticProvider{name: "static", version: "1"}
	registry := provider.NewRegistry(implementation)
	jobs := []job{}
	results := map[string]provider.Result{"unknown": {}}
	add := func(id, status string, evidence provider.Evidence) {
		current := edgeJob(id)
		jobs = append(jobs, current)
		results[current.fullKey] = provider.Result{Status: status, Evidence: []provider.Evidence{evidence}}
	}
	add("invalid-status", "bogus", provider.Evidence{Class: "deterministic", Passed: &passed})
	add("invalid-class", "completed", provider.Evidence{Class: "bogus", Passed: &passed})
	add("invalid-confidence", "completed", provider.Evidence{Class: "deterministic", Confidence: &confidence, Passed: &passed})
	add("error", "error", provider.Evidence{Class: "deterministic"})
	add("skipped", "skipped", provider.Evidence{Class: "deterministic"})
	add("unknown-status", "completed", provider.Evidence{Class: "deterministic"})
	add("passed", "completed", provider.Evidence{Class: "deterministic", Passed: &passed})
	add("failed", "completed", provider.Evidence{Class: "deterministic", Passed: &failed})
	normalizeResults(results, jobs, Options{Registry: registry, RunID: "run"})
	for _, id := range []string{"invalid-status", "invalid-class", "invalid-confidence"} {
		if results["REQ/OBL/"+id].Status != "error" {
			t.Fatalf("%s: %+v", id, results["REQ/OBL/"+id])
		}
	}
	for id, want := range map[string]string{
		"error": "error", "skipped": "skipped", "unknown-status": "unknown",
		"passed": "passed", "failed": "failed",
	} {
		record := results["REQ/OBL/"+id].Evidence[0]
		if record.ID != id || record.Status != want || record.ProviderVersion != "1" ||
			record.StartedAt.IsZero() || record.CompletedAt.IsZero() {
			t.Fatalf("%s: %+v", id, record)
		}
	}
}

func TestExecuteJobsDependencyFailFastAndDirectStops(t *testing.T) {
	cfg := config.Default()
	cfg.Verification.MaxParallel = 1
	passed, failed := true, false
	implementation := &staticProvider{
		name: "static", version: "1",
		result: provider.Result{Status: "completed", Evidence: []provider.Evidence{{
			Class: "deterministic", Passed: &passed,
		}}},
	}
	opt := Options{
		Root: t.TempDir(), Config: cfg, Registry: provider.NewRegistry(implementation), NoCache: true,
	}
	blocked := edgeJob("blocked")
	blocked.dependsOn = []string{"missing"}
	if result := executeJobs(context.Background(), []job{blocked}, opt)[blocked.fullKey]; result.Status != "skipped" {
		t.Fatal(result)
	}

	cfg.Verification.FailFast = true
	first, second := edgeJob("a"), edgeJob("b")
	implementation.result = provider.Result{Status: "completed", Evidence: []provider.Evidence{{
		Class: "deterministic", Passed: &failed,
	}}}
	results := executeJobs(context.Background(), []job{second, first}, opt)
	if results[second.fullKey].Status != "skipped" {
		t.Fatalf("%+v", results)
	}

	if batch := compatibleBatch([]job{first}, 0); len(batch) != 1 {
		t.Fatal(batch)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result := executeJob(cancelled, first, opt); result.Status != "error" {
		t.Fatal(result)
	}
	other := "linux"
	if runtime.GOOS == "linux" {
		other = "darwin"
	}
	first.obl.Platforms = []string{other}
	if result := executeJob(context.Background(), first, opt); result.Status != "skipped" {
		t.Fatal(result)
	}
}

func TestRetryBackoffCancellationAndArtifactFailure(t *testing.T) {
	failed := false
	cfg := config.Default()
	current := edgeJob("retry")
	current.spec.Retry = ir.Retry{Attempts: 2, Backoff: "1ms"}
	implementation := &staticProvider{
		name: "static", version: "1",
		result: provider.Result{
			Status: "completed", Stdout: "line\n",
			Evidence: []provider.Evidence{{Class: "deterministic", Passed: &failed}},
		},
	}
	opt := Options{
		Root: t.TempDir(), Config: cfg, Registry: provider.NewRegistry(implementation), NoCache: true,
	}
	if result := executeJob(context.Background(), current, opt); len(result.Evidence) != 2 {
		t.Fatal(result)
	}

	ctx, cancel := context.WithCancel(context.Background())
	current.spec.Retry.Backoff = time.Second.String()
	implementation.execute = func(context.Context, provider.Request) provider.Result {
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		return implementation.result
	}
	if result := executeJob(ctx, current, opt); len(result.Evidence) == 0 {
		t.Fatal(result)
	}

	implementation.execute = nil
	current.spec.Retry = ir.Retry{}
	current.spec.Artifacts = []string{"["}
	current.spec.Outputs = []string{"missing"}
	current.spec.Exclusive = true
	result := executeJob(context.Background(), current, opt)
	if result.Status != "error" || len(result.Diagnostics) == 0 {
		t.Fatal(result)
	}
}

func TestEvidenceOutputRedactionAndCacheHelpers(t *testing.T) {
	passed, failed := true, false
	implementation := &staticProvider{name: "static", version: "1"}
	threshold := 0.8
	for _, testCase := range []struct {
		status string
		pass   *bool
		want   string
	}{
		{"error", &failed, "error"},
		{"skipped", nil, "skipped"},
		{"completed", nil, "unknown"},
		{"completed", &passed, "passed"},
		{"completed", &failed, "failed"},
	} {
		result := provider.Result{Status: testCase.status, Evidence: []provider.Evidence{{}}}
		result.Evidence[0].Passed = testCase.pass
		enrichEvidence(&result, provider.Request{
			Spec:          ir.ProviderSpec{Provider: "static", ID: "id"},
			EvidenceClass: "probabilistic", ConfidenceThreshold: &threshold,
		}, implementation)
		if result.Evidence[0].Status != testCase.want ||
			result.Evidence[0].Class != "probabilistic" ||
			result.Evidence[0].Confidence == nil {
			t.Fatalf("%+v", result)
		}
	}
	resultWithVersion := provider.Result{
		Status: "completed", ProviderVersion: "custom",
		Evidence: []provider.Evidence{{Class: "deterministic", Passed: &passed}},
	}
	enrichEvidence(&resultWithVersion, provider.Request{}, implementation)
	if resultWithVersion.Evidence[0].ProviderVersion != "custom" {
		t.Fatal(resultWithVersion)
	}

	outputResult := provider.Result{Status: "error"}
	validateOutputs(provider.Request{Spec: ir.ProviderSpec{Outputs: []string{"missing"}}}, &outputResult)
	if len(outputResult.Evidence) != 0 {
		t.Fatal(outputResult)
	}
	var log strings.Builder
	appendAttemptLog(&log, 1, "line\n")
	if strings.Count(log.String(), "\n") != 2 {
		t.Fatal(log.String())
	}
	t.Setenv("SECRET_VALUE", "true")
	redacted := redactResult(provider.Result{
		Status: "completed", Stdout: "true", Extra: map[string]any{"flag": true, "items": []any{"true"}},
	}, []string{"SECRET_VALUE"})
	if redacted.Stdout != "[REDACTED]" || redacted.Extra["flag"] != true {
		t.Fatalf("%+v", redacted)
	}
	if effectiveTimeout("bad", "") != 10*time.Minute {
		t.Fatal("default timeout")
	}
	if attempts, _ := retrySettings(ir.Retry{}, ir.Retry{}); attempts != 1 {
		t.Fatal(attempts)
	}
	if resultFailed(provider.Result{
		Status: "completed", Evidence: []provider.Evidence{
			{Data: map[string]any{"retry_superseded": true}},
			{Class: "deterministic", Passed: &passed},
		},
	}) {
		t.Fatal("superseded retry failed result")
	}
	if !cacheSuccess(provider.Result{
		Status: "completed", Evidence: []provider.Evidence{
			{Data: map[string]any{"retry_superseded": true}},
			{Class: "deterministic", Passed: &passed},
		},
	}) {
		t.Fatal("superseded retry invalidated cache")
	}
	for _, value := range []provider.Result{
		{},
		{Status: "completed"},
		{Status: "completed", Evidence: []provider.Evidence{{Class: "probabilistic", Passed: &passed}}},
	} {
		if cacheSuccess(value) {
			t.Fatal(value)
		}
	}
}

func TestArtifactCollectionEdges(t *testing.T) {
	root := t.TempDir()
	evidenceRoot := filepath.Join(root, "evidence")
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	request := provider.Request{
		Root: root, EvidenceDir: evidenceRoot, RequirementID: "../REQ", ObligationID: "../OBL",
		ExecutionAttempt: 2,
		Spec:             ir.ProviderSpec{Provider: "static", ID: "../id", Artifacts: []string{"file"}},
	}
	result := provider.Result{Evidence: []provider.Evidence{{}}}
	if err := collectArtifacts(request, &result); err != nil ||
		!strings.Contains(result.Evidence[0].Artifacts[0].Path, "try-002") {
		t.Fatalf("%+v %v", result, err)
	}
	request.Spec.Artifacts = []string{"directory"}
	if err := collectArtifacts(request, &result); err == nil {
		t.Fatal("directory collected as artifact")
	}
	request.Spec.Artifacts = []string{"["}
	if err := collectArtifacts(request, &result); err == nil {
		t.Fatal("invalid artifact glob accepted")
	}
	request.Spec.Artifacts = []string{"file"}
	request.EvidenceDir = filepath.Join(root, "evidence-file")
	if err := os.WriteFile(request.EvidenceDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := collectArtifacts(request, &result); err == nil {
		t.Fatal("evidence directory file accepted")
	}
	request.EvidenceDir = evidenceRoot
	oldLstat, oldOpen := lstatPath, openFile
	lstatPath = func(string) (os.FileInfo, error) { return nil, errors.New("lstat") }
	if err := collectArtifacts(request, &result); err == nil {
		t.Fatal("artifact lstat failure ignored")
	}
	lstatPath = oldLstat
	openFile = func(string) (*os.File, error) { return nil, errors.New("open") }
	if err := collectArtifacts(request, &result); err == nil {
		t.Fatal("artifact copy failure ignored")
	}
	openFile = oldOpen

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "outside-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "artifact"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	request.EvidenceDir = evidenceRoot
	request.Spec.Artifacts = []string{"outside-link/*"}
	if err := collectArtifacts(request, &result); err == nil || !security.IsPathViolation(errors.Unwrap(err)) && !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("%v", err)
	}

	destinationLink := filepath.Join(evidenceRoot, "REQ")
	if err := os.MkdirAll(evidenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, destinationLink); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}
	request.RequirementID = "REQ"
	request.ExecutionAttempt = 1
	request.Spec.Artifacts = []string{"file"}
	if err := collectArtifacts(request, &result); err == nil {
		t.Fatal("artifact destination symlink accepted")
	}
}

type failingTemporary struct {
	name       string
	writeError error
	closeError error
}

func (f *failingTemporary) Name() string { return f.name }
func (f *failingTemporary) Close() error { return f.closeError }
func (f *failingTemporary) Write(value []byte) (int, error) {
	if f.writeError != nil {
		return 0, f.writeError
	}
	return len(value), nil
}

func TestFilesystemAndCacheFailures(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.WriteFile(source, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldMkdir, oldCreate, oldRename := makeDirectories, createTemporary, renamePath
	oldLstat, oldReadlink, oldRead := lstatPath, readlinkPath, readPath
	defer func() {
		makeDirectories, createTemporary, renamePath = oldMkdir, oldCreate, oldRename
		lstatPath, readlinkPath, readPath = oldLstat, oldReadlink, oldRead
	}()

	makeDirectories = func(string, os.FileMode) error { return errors.New("mkdir") }
	if _, err := copyArtifact(source, filepath.Join(root, "out")); err == nil {
		t.Fatal("artifact mkdir failure ignored")
	}
	makeDirectories = oldMkdir
	createTemporary = func(string, string) (temporaryFile, error) { return nil, errors.New("create") }
	if _, err := copyArtifact(source, filepath.Join(root, "out")); err == nil {
		t.Fatal("artifact temp failure ignored")
	}
	createTemporary = func(string, string) (temporaryFile, error) {
		return &failingTemporary{name: filepath.Join(root, "tmp"), writeError: errors.New("write")}, nil
	}
	if _, err := copyArtifact(source, filepath.Join(root, "out")); err == nil {
		t.Fatal("artifact write failure ignored")
	}
	createTemporary = func(string, string) (temporaryFile, error) {
		return &failingTemporary{name: filepath.Join(root, "tmp"), closeError: errors.New("close")}, nil
	}
	if _, err := copyArtifact(source, filepath.Join(root, "out")); err == nil {
		t.Fatal("artifact close failure ignored")
	}
	createTemporary = oldCreate
	renamePath = func(string, string) error { return errors.New("rename") }
	if _, err := copyArtifact(source, filepath.Join(root, "out")); err == nil {
		t.Fatal("artifact rename failure ignored")
	}
	renamePath = oldRename

	current := edgeJob("cache")
	current.spec.Inputs = []string{"["}
	if _, ok := cacheKey(provider.Request{}, current, "1", Options{Root: root}); ok {
		t.Fatal("invalid input glob produced cache key")
	}
	current.spec.Inputs = nil
	current.spec.Extra = map[string]any{"bad": make(chan int)}
	if _, ok := cacheKey(provider.Request{Spec: current.spec}, current, "1", Options{Root: root}); ok {
		t.Fatal("unmarshalable provider spec produced cache key")
	}

	directory := filepath.Join(root, "directory")
	link := filepath.Join(root, "link")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("source", link); err != nil {
		t.Fatal(err)
	}
	if _, err := hashInputs(root, []string{"directory", "link", "source"}, nil, "", ""); err != nil {
		t.Fatal(err)
	}
	lstatPath = func(string) (os.FileInfo, error) { return nil, errors.New("lstat") }
	if _, err := hashInputs(root, []string{"source"}, nil, "", ""); err == nil {
		t.Fatal("lstat failure ignored")
	}
	lstatPath = oldLstat
	readlinkPath = func(string) (string, error) { return "", errors.New("readlink") }
	if _, err := hashInputs(root, []string{"link"}, nil, "", ""); err == nil {
		t.Fatal("readlink failure ignored")
	}
	readlinkPath = oldReadlink
	readPath = func(string) ([]byte, error) { return nil, errors.New("read") }
	if _, err := hashInputs(root, []string{"source"}, nil, "", ""); err == nil {
		t.Fatal("read failure ignored")
	}
	readPath = oldRead

	if err := saveCache(root, "bad", provider.Result{Extra: map[string]any{"bad": make(chan int)}}); err == nil {
		t.Fatal("cache marshal failure ignored")
	}
	cacheFile := filepath.Join(root, "cache-file")
	if err := os.WriteFile(cacheFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := saveCache(cacheFile, "key", provider.Result{}); err == nil {
		t.Fatal("cache mkdir failure ignored")
	}
	createTemporary = func(string, string) (temporaryFile, error) { return nil, errors.New("create") }
	if err := saveCache(root, "key", provider.Result{}); err == nil {
		t.Fatal("cache create failure ignored")
	}
	createTemporary = func(string, string) (temporaryFile, error) {
		return &failingTemporary{name: filepath.Join(root, "tmp"), writeError: errors.New("write")}, nil
	}
	if err := saveCache(root, "key", provider.Result{}); err == nil {
		t.Fatal("cache write failure ignored")
	}
	createTemporary = func(string, string) (temporaryFile, error) {
		return &failingTemporary{name: filepath.Join(root, "tmp"), closeError: errors.New("close")}, nil
	}
	if err := saveCache(root, "key", provider.Result{}); err == nil {
		t.Fatal("cache close failure ignored")
	}
	createTemporary = oldCreate
	renamePath = func(string, string) error { return errors.New("rename") }
	if err := saveCache(root, "key", provider.Result{}); err == nil {
		t.Fatal("cache rename failure ignored")
	}
}

func TestRemainingHelpers(t *testing.T) {
	if !outputPatternsConflict("build/file", "build/**") ||
		!outputPatternsConflict("build/file", "build/file?") ||
		wildcardIndex("plain") != len("plain") ||
		wildcardIndex("a[b]") != 1 {
		t.Fatal("pattern helpers")
	}
	if strings.Join(jobWritePatterns(ir.ProviderSpec{Outputs: []string{"a"}, Artifacts: []string{"b"}}), "") != "ab" {
		t.Fatal("write patterns")
	}
	if contains([]string{"a"}, "b") || firstNonEmpty("", "") != "" {
		t.Fatal("list helpers")
	}
	if safeSegment(".") != "artifact" || safeSegment("/") != "artifact" {
		t.Fatal("safe segments")
	}
	if _, err := io.Copy(io.Discard, strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
}

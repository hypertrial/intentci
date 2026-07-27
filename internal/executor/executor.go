package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/security"
	"github.com/hypertrial/intentci/internal/verdict"
)

type temporaryFile interface {
	io.Writer
	Close() error
	Name() string
}

var makeDirectories = os.MkdirAll
var openFile = os.Open
var createTemporary = func(directory, pattern string) (temporaryFile, error) {
	return os.CreateTemp(directory, pattern)
}
var renamePath = os.Rename
var lstatPath = os.Lstat
var readlinkPath = os.Readlink
var readPath = os.ReadFile

// Options configures execution.
type Options struct {
	Root         string
	Config       *config.Config
	Registry     *provider.Registry
	BaseCommit   string
	HeadCommit   string
	DiffHash     string
	ChangedFiles []string
	Changes      []provider.Change
	RunID        string
	AttemptID    string
	EvidenceDir  string
	IRHash       string
	PlanHash     string
	ProviderID   string
	NoCache      bool
	CacheDir     string
}

// LeafResult maps provider leaf id -> result.
type LeafResult map[string]provider.Result

type job struct {
	req       ir.Requirement
	obl       ir.Obligation
	key       string
	fullKey   string
	spec      ir.ProviderSpec
	dependsOn []string
}

type prepared struct {
	req    ir.Requirement
	obl    ir.Obligation
	verify ir.VerifyNode
	jobs   []job
}

// Run executes all provider leaves for selected requirements.
func Run(ctx context.Context, reqs []ir.Requirement, opt Options) (map[string]LeafResult, []verdict.RequirementResult) {
	if opt.Config == nil {
		opt.Config = config.Default()
	}
	if opt.Registry == nil {
		opt.Registry = provider.DefaultRegistry()
	}
	var preparedObs []prepared
	var jobs []job
	providerIDs := map[string]string{}
	obligationJobKeys := map[string][]string{}
	for _, requirement := range reqs {
		for _, obligation := range requirement.Obligations {
			counter := 0
			verify := assignLeafIDs(obligation.Verify, &counter)
			var obligationJobs []job
			collectLeaves(verify, func(spec ir.ProviderSpec) {
				fullKey := requirement.ID + "/" + obligation.ID + "/" + spec.ID
				current := job{
					req: requirement, obl: obligation, key: spec.ID,
					fullKey: fullKey, spec: spec,
				}
				obligationJobs = append(obligationJobs, current)
				jobs = append(jobs, current)
				providerIDs[requirement.ID+"/"+spec.ID] = fullKey
			})
			preparedObs = append(preparedObs, prepared{
				req: requirement, obl: obligation, verify: verify, jobs: obligationJobs,
			})
			for _, current := range obligationJobs {
				obligationJobKeys[requirement.ID+"/"+obligation.ID] = append(
					obligationJobKeys[requirement.ID+"/"+obligation.ID], current.fullKey,
				)
			}
		}
	}
	for index := range jobs {
		for _, dependency := range jobs[index].spec.DependsOn {
			if key := providerIDs[jobs[index].req.ID+"/"+dependency]; key != "" {
				jobs[index].dependsOn = append(jobs[index].dependsOn, key)
			}
		}
		for _, dependency := range jobs[index].obl.DependsOn {
			jobs[index].dependsOn = append(
				jobs[index].dependsOn, obligationJobKeys[jobs[index].req.ID+"/"+dependency]...,
			)
		}
	}

	results := executeJobs(ctx, jobs, opt)
	normalizeResults(results, jobs, opt)
	if mutateResults != nil {
		mutateResults(results)
	}

	byReq := map[string]LeafResult{}
	obligationResults := map[string]map[string]verdict.ObligationResult{}
	for _, item := range preparedObs {
		leaves := LeafResult{}
		for _, current := range item.jobs {
			if result, ok := results[current.fullKey]; ok {
				leaves[current.key] = result
			}
		}
		if byReq[item.req.ID] == nil {
			byReq[item.req.ID] = LeafResult{}
		}
		for key, result := range leaves {
			byReq[item.req.ID][item.obl.ID+"/"+key] = result
		}
		value, reason, evidence := verdict.EvaluateNodeWithPolicy(item.verify, leaves, verdict.EvidencePolicy{
			Class: item.obl.EvidenceClass, ConfidenceThreshold: item.obl.ConfidenceThreshold,
		})
		if item.obl.ManualReview && value != verdict.Fail && value != verdict.Error {
			value = verdict.ReviewRequired
			reason = "obligation requires manual review"
		}
		if obligationResults[item.req.ID] == nil {
			obligationResults[item.req.ID] = map[string]verdict.ObligationResult{}
		}
		obligationResults[item.req.ID][item.obl.ID] = verdict.ObligationResult{
			ID: item.obl.ID, Statement: item.obl.Statement, Required: item.obl.Required,
			Verdict: value, Reason: reason, Evidence: evidence,
		}
	}
	for _, item := range preparedObs {
		result := obligationResults[item.req.ID][item.obl.ID]
		for _, dependency := range item.obl.DependsOn {
			if prior, ok := obligationResults[item.req.ID][dependency]; ok && prior.Verdict != verdict.Pass {
				result.Verdict = verdict.Unproven
				result.Reason = "dependency " + dependency + " did not pass"
				break
			}
		}
		obligationResults[item.req.ID][item.obl.ID] = result
	}

	var requirementResults []verdict.RequirementResult
	for _, requirement := range reqs {
		obligations := make([]verdict.ObligationResult, 0, len(requirement.Obligations))
		for _, obligation := range requirement.Obligations {
			if result, ok := obligationResults[requirement.ID][obligation.ID]; ok {
				obligations = append(obligations, result)
			}
		}
		requirementResults = append(requirementResults, verdict.AggregateRequirement(requirement, obligations))
	}
	requirementResults = applyRequirementDependencies(reqs, requirementResults)
	return byReq, requirementResults
}

func applyRequirementDependencies(requirements []ir.Requirement, results []verdict.RequirementResult) []verdict.RequirementResult {
	byID := make(map[string]verdict.RequirementResult, len(results))
	for _, result := range results {
		byID[result.ID] = result
	}
	for changed := true; changed; {
		changed = false
		for index, requirement := range requirements {
			for _, dependency := range requirement.DependsOn {
				dependencyResult, ok := byID[dependency]
				if !ok || dependencyResult.Verdict != verdict.Pass {
					if results[index].Verdict == verdict.Pass {
						results[index].Verdict = verdict.Unproven
						results[index].Reason = "requirement dependency " + dependency + " did not pass"
						byID[results[index].ID] = results[index]
						changed = true
					}
					break
				}
			}
		}
	}
	return results
}

func normalizeResults(results map[string]provider.Result, jobs []job, opt Options) {
	byKey := make(map[string]job, len(jobs))
	for _, current := range jobs {
		byKey[current.fullKey] = current
	}
	for key, result := range results {
		current, ok := byKey[key]
		if !ok {
			continue
		}
		version := result.ProviderVersion
		if version == "" {
			if implementation, found := opt.Registry.Get(current.spec.Provider); found {
				version = implementation.Version()
			}
		}
		if result.Status != "completed" && result.Status != "error" && result.Status != "skipped" {
			result.Status = "error"
			result.Diagnostics = append(result.Diagnostics, "provider returned invalid status")
		}
		for _, record := range result.Evidence {
			if !contains([]string{"deterministic", "probabilistic", "human", "informational"}, record.Class) ||
				(record.Confidence != nil && (*record.Confidence < 0 || *record.Confidence > 1)) {
				result.Status = "error"
				result.Diagnostics = append(result.Diagnostics, "provider returned invalid evidence")
				result.Evidence = []provider.Evidence{{
					ID: current.key, Class: "deterministic", Status: "error",
					Summary: "provider returned invalid evidence", Passed: boolPtr(false),
				}}
				break
			}
		}
		now := time.Now().UTC()
		for index := range result.Evidence {
			record := &result.Evidence[index]
			if record.ID == "" {
				record.ID = current.key
			}
			record.SchemaVersion = "1.0"
			record.RunID = opt.RunID
			record.AttemptID = opt.AttemptID
			record.RequirementID = current.req.ID
			record.ObligationID = current.obl.ID
			record.VerifierID = current.spec.ID
			record.Provider = current.spec.Provider
			record.ProviderVersion = version
			record.RepositoryCommit = firstNonEmpty(opt.HeadCommit, "unknown")
			record.BaseCommit = firstNonEmpty(opt.BaseCommit, "unknown")
			record.DiffHash = firstNonEmpty(opt.DiffHash, ir.HashBytes(nil))
			record.RequirementHash = current.req.Hash
			record.ObligationHash = current.obl.Hash
			record.PlanHash = firstNonEmpty(opt.PlanHash, opt.IRHash)
			if record.StartedAt.IsZero() {
				record.StartedAt = now
			}
			if record.CompletedAt.IsZero() {
				record.CompletedAt = now
			}
			if record.Status == "" {
				switch {
				case result.Status == "error":
					record.Status = "error"
				case result.Status == "skipped":
					record.Status = "skipped"
				case record.Passed == nil:
					record.Status = "unknown"
				case *record.Passed:
					record.Status = "passed"
				default:
					record.Status = "failed"
				}
			}
		}
		results[key] = result
	}
}

func executeJobs(ctx context.Context, jobs []job, opt Options) map[string]provider.Result {
	pending := append([]job{}, jobs...)
	results := map[string]provider.Result{}
	maximum := opt.Config.MaxParallelOr(4)
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			for _, current := range pending {
				results[current.fullKey] = provider.Result{
					Provider: current.spec.Provider, Status: "error", Diagnostics: []string{err.Error()},
					Evidence: []provider.Evidence{{
						ID: current.key, Class: "deterministic", Status: "error",
						Summary: "verification interrupted: " + err.Error(), Passed: boolPtr(false),
					}},
				}
			}
			break
		}
		var ready []job
		for _, current := range pending {
			dependenciesDone := true
			for _, dependency := range current.dependsOn {
				if _, ok := results[dependency]; !ok {
					dependenciesDone = false
				}
			}
			if dependenciesDone {
				ready = append(ready, current)
			}
		}
		if len(ready) == 0 {
			for _, current := range pending {
				results[current.fullKey] = skippedResult(current, "unresolved verifier dependency")
			}
			break
		}
		sort.Slice(ready, func(i, j int) bool { return ready[i].fullKey < ready[j].fullKey })
		batch := compatibleBatch(ready, maximum)
		var mutex sync.Mutex
		var wait sync.WaitGroup
		for _, current := range batch {
			current := current
			wait.Add(1)
			go func() {
				defer wait.Done()
				result := executeJob(ctx, current, opt)
				mutex.Lock()
				results[current.fullKey] = result
				mutex.Unlock()
			}()
		}
		wait.Wait()
		ran := map[string]bool{}
		stop := false
		for _, current := range batch {
			ran[current.fullKey] = true
			if opt.Config.Verification.FailFast && resultFailed(results[current.fullKey]) {
				stop = true
			}
		}
		next := pending[:0]
		for _, current := range pending {
			if !ran[current.fullKey] {
				next = append(next, current)
			}
		}
		pending = next
		if stop {
			for _, current := range pending {
				results[current.fullKey] = skippedResult(current, "fail-fast cancellation")
			}
			break
		}
	}
	return results
}

func compatibleBatch(ready []job, maximum int) []job {
	var batch []job
	outputs := map[string]bool{}
	for _, current := range ready {
		if len(batch) >= maximum {
			break
		}
		if current.spec.Exclusive {
			if len(batch) == 0 {
				return []job{current}
			}
			continue
		}
		conflict := false
		for _, output := range jobWritePatterns(current.spec) {
			for scheduled := range outputs {
				if outputPatternsConflict(output, scheduled) {
					conflict = true
					break
				}
			}
		}
		if conflict {
			continue
		}
		batch = append(batch, current)
		for _, output := range jobWritePatterns(current.spec) {
			outputs[output] = true
		}
	}
	if len(batch) == 0 {
		return ready[:1]
	}
	return batch
}

func jobWritePatterns(spec ir.ProviderSpec) []string {
	output := append([]string{}, spec.Outputs...)
	return append(output, spec.Artifacts...)
}

func outputPatternsConflict(left, right string) bool {
	if left == right {
		return true
	}
	if matched, _ := doublestar.Match(left, right); matched {
		return true
	}
	if matched, _ := doublestar.Match(right, left); matched {
		return true
	}
	leftPrefix := left[:wildcardIndex(left)]
	rightPrefix := right[:wildcardIndex(right)]
	return strings.HasPrefix(leftPrefix, rightPrefix) || strings.HasPrefix(rightPrefix, leftPrefix)
}

func wildcardIndex(value string) int {
	index := len(value)
	for _, token := range []string{"*", "?", "["} {
		if found := strings.Index(value, token); found >= 0 && found < index {
			index = found
		}
	}
	return index
}

func executeJob(ctx context.Context, current job, opt Options) provider.Result {
	if err := ctx.Err(); err != nil {
		return provider.Result{
			Provider: current.spec.Provider, Status: "error", Diagnostics: []string{err.Error()},
			Evidence: []provider.Evidence{{
				ID: current.key, Class: "deterministic", Status: "error",
				Summary: "verification interrupted: " + err.Error(), Passed: boolPtr(false),
			}},
		}
	}
	if opt.ProviderID != "" && current.spec.ID != opt.ProviderID {
		return skippedResult(current, "not selected by --provider")
	}
	if len(current.obl.Platforms) > 0 && !contains(current.obl.Platforms, runtime.GOOS) {
		return skippedResult(current, "unsupported platform "+runtime.GOOS)
	}
	implementation, ok := opt.Registry.Get(current.spec.Provider)
	if !ok {
		return provider.Result{
			Provider: current.spec.Provider, Status: "error",
			Diagnostics: []string{"unknown provider"},
			Evidence: []provider.Evidence{{
				ID: current.key, Class: "deterministic", Summary: "unknown provider", Passed: boolPtr(false),
			}},
		}
	}
	spec := current.spec
	if spec.WorkingDirectory == "" {
		spec.WorkingDirectory = opt.Config.Verification.WorkingDirectory
	}
	request := provider.Request{
		RunID: opt.RunID, AttemptID: opt.AttemptID,
		RequirementID: current.req.ID, ObligationID: current.obl.ID,
		Root: opt.Root, EvidenceDir: opt.EvidenceDir,
		BaseCommit: opt.BaseCommit, HeadCommit: opt.HeadCommit, DiffHash: opt.DiffHash,
		RequirementHash: current.req.Hash, ObligationHash: current.obl.Hash,
		PlanHash:            firstNonEmpty(opt.PlanHash, opt.IRHash),
		EvidenceClass:       current.obl.EvidenceClass,
		ConfidenceThreshold: current.obl.ConfidenceThreshold,
		ChangedFiles:        opt.ChangedFiles, Spec: spec,
		Changes:      opt.Changes,
		Timeout:      effectiveTimeout(spec.Timeout, current.obl.Timeout, current.req.Timeout, opt.Config.Verification.DefaultTimeout),
		RetainStdout: opt.Config.Evidence.RetainStdout,
		RetainStderr: opt.Config.Evidence.RetainStderr,
	}
	request.ExecutionAttempt = 1
	cacheKey, cacheable := cacheKey(request, current, implementation.Version(), opt)
	if implementation.Version() == "external" || len(spec.Outputs) > 0 || len(spec.Artifacts) > 0 {
		cacheable = false
	}
	if cacheable && !opt.NoCache {
		if cached, ok := loadCache(opt.CacheDir, cacheKey); ok {
			cached = redactResult(cached, opt.Config.Evidence.Redact.Environment)
			source, _ := json.Marshal(cached)
			sourceHash := hash(source)
			cached.FromCache = true
			cached.SourceEvidenceHash = sourceHash
			cached.DurationMS = 0
			now := time.Now().UTC()
			for index := range cached.Evidence {
				cached.Evidence[index].ID += "-cached-" + opt.RunID
				cached.Evidence[index].RunID = opt.RunID
				cached.Evidence[index].AttemptID = opt.AttemptID
				cached.Evidence[index].SourceEvidenceHash = sourceHash
				cached.Evidence[index].StartedAt = now
				cached.Evidence[index].CompletedAt = now
				cached.Evidence[index].Artifacts = nil
			}
			return cached
		}
	}
	attempts, backoff := retrySettings(spec.Retry, current.obl.Retry)
	var result provider.Result
	var observations []provider.Evidence
	finalEvidenceCount := 0
	var stdout, stderr strings.Builder
	for attempt := 1; attempt <= attempts; attempt++ {
		request.ExecutionAttempt = attempt
		result = implementation.Execute(ctx, request)
		enrichEvidence(&result, request, implementation)
		validateOutputs(request, &result)
		if attempts > 1 {
			for index := range result.Evidence {
				result.Evidence[index].ID += fmt.Sprintf("-try-%d", attempt)
			}
		}
		if attempts > 1 {
			appendAttemptLog(&stdout, attempt, result.Stdout)
			appendAttemptLog(&stderr, attempt, result.Stderr)
		}
		if err := collectArtifacts(request, &result); err != nil {
			result.Status = "error"
			result.Diagnostics = append(result.Diagnostics, err.Error())
			result.SecurityViolation = security.IsPathViolation(err)
		}
		observations = append(observations, result.Evidence...)
		finalEvidenceCount = len(result.Evidence)
		if !resultFailed(result) || attempt == attempts || ctx.Err() != nil {
			break
		}
		if backoff > 0 {
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
		}
	}
	if !resultFailed(result) && len(observations) > finalEvidenceCount {
		for index := 0; index < len(observations)-finalEvidenceCount; index++ {
			if observations[index].Data == nil {
				observations[index].Data = map[string]any{}
			}
			observations[index].Data["retry_superseded"] = true
		}
	}
	result.Evidence = observations
	if attempts > 1 {
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()
	}
	if cacheable && !opt.NoCache && cacheSuccess(result) {
		_ = saveCache(opt.CacheDir, cacheKey, redactResult(result, opt.Config.Evidence.Redact.Environment))
	}
	return result
}

func validateOutputs(request provider.Request, result *provider.Result) {
	if result.Status != "completed" {
		return
	}
	for _, pattern := range request.Spec.Outputs {
		matches, err := doublestar.FilepathGlob(filepath.Join(request.Root, filepath.FromSlash(pattern)))
		if err == nil && len(matches) > 0 {
			continue
		}
		result.Evidence = append(result.Evidence, provider.Evidence{
			ID:    firstNonEmpty(request.Spec.ID, request.Spec.Provider) + "-output",
			Class: "deterministic", Status: "failed",
			Summary: fmt.Sprintf("required output %q was not produced", pattern), Passed: boolPtr(false),
		})
	}
}

func appendAttemptLog(output *strings.Builder, attempt int, content string) {
	if content == "" {
		return
	}
	fmt.Fprintf(output, "== provider attempt %d ==\n", attempt)
	output.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		output.WriteByte('\n')
	}
}

func redactResult(result provider.Result, patterns []string) provider.Result {
	raw, err := json.Marshal(result)
	if err != nil {
		return result
	}
	redactor := security.NewRedactor(patterns, os.Environ())
	var generic any
	_ = json.Unmarshal(raw, &generic)
	redactJSONStrings(generic, redactor)
	raw, _ = json.Marshal(generic)
	var redacted provider.Result
	_ = json.Unmarshal(raw, &redacted)
	return redacted
}

func redactJSONStrings(value any, redactor security.Redactor) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if text, ok := child.(string); ok {
				typed[key] = redactor.Redact(text)
			} else {
				redactJSONStrings(child, redactor)
			}
		}
	case []any:
		for index, child := range typed {
			if text, ok := child.(string); ok {
				typed[index] = redactor.Redact(text)
			} else {
				redactJSONStrings(child, redactor)
			}
		}
	}
}

func effectiveTimeout(values ...string) time.Duration {
	for _, value := range values {
		if value != "" {
			if duration, err := config.ParseDuration(value); err == nil {
				return duration
			}
		}
	}
	return 10 * time.Minute
}

func retrySettings(providerRetry, obligationRetry ir.Retry) (int, time.Duration) {
	retry := providerRetry
	if retry.Attempts == 0 {
		retry = obligationRetry
	}
	attempts := retry.Attempts
	if attempts < 1 {
		attempts = 1
	}
	backoff, _ := time.ParseDuration(retry.Backoff)
	return attempts, backoff
}

func enrichEvidence(result *provider.Result, request provider.Request, implementation provider.Provider) {
	for index := range result.Evidence {
		evidence := &result.Evidence[index]
		evidence.SchemaVersion = "1.0"
		evidence.RunID = request.RunID
		evidence.AttemptID = request.AttemptID
		evidence.RequirementID = request.RequirementID
		evidence.ObligationID = request.ObligationID
		evidence.VerifierID = request.Spec.ID
		evidence.Provider = implementation.Name()
		if result.ProviderVersion != "" {
			evidence.ProviderVersion = result.ProviderVersion
		} else {
			evidence.ProviderVersion = implementation.Version()
		}
		evidence.RepositoryCommit = firstNonEmpty(request.HeadCommit, "unknown")
		evidence.BaseCommit = firstNonEmpty(request.BaseCommit, "unknown")
		evidence.DiffHash = firstNonEmpty(request.DiffHash, ir.HashBytes(nil))
		evidence.RequirementHash = request.RequirementHash
		evidence.ObligationHash = request.ObligationHash
		evidence.PlanHash = request.PlanHash
		if evidence.Class == "" {
			evidence.Class = firstNonEmpty(request.Spec.EvidenceClass, request.EvidenceClass, "deterministic")
		}
		if evidence.Status == "" {
			switch {
			case result.Status == "error":
				evidence.Status = "error"
			case result.Status == "skipped":
				evidence.Status = "skipped"
			case evidence.Passed == nil:
				evidence.Status = "unknown"
			case *evidence.Passed:
				evidence.Status = "passed"
			default:
				evidence.Status = "failed"
			}
		}
		now := time.Now().UTC()
		if evidence.StartedAt.IsZero() {
			evidence.StartedAt = now
		}
		if evidence.CompletedAt.IsZero() {
			evidence.CompletedAt = now
		}
	}
}

func collectArtifacts(request provider.Request, result *provider.Result) error {
	if len(request.Spec.Artifacts) == 0 {
		return nil
	}
	if err := makeDirectories(request.EvidenceDir, 0o755); err != nil {
		return err
	}
	for _, pattern := range request.Spec.Artifacts {
		matches, err := doublestar.FilepathGlob(filepath.Join(request.Root, filepath.FromSlash(pattern)))
		if err != nil {
			return fmt.Errorf("collect artifact %q: %w", pattern, err)
		}
		for _, match := range matches {
			relative, _ := filepath.Rel(request.Root, match)
			relative = filepath.ToSlash(relative)
			source, err := security.ResolveInside(request.Root, relative)
			if err != nil {
				return fmt.Errorf("collect artifact %q: %w", relative, err)
			}
			info, err := lstatPath(source)
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("artifact must be a regular file: %s", relative)
			}
			destinationRelative := filepath.Join(
				safeSegment(request.RequirementID), safeSegment(request.ObligationID),
				safeSegment(firstNonEmpty(request.Spec.ID, request.Spec.Provider)), filepath.FromSlash(relative),
			)
			if request.ExecutionAttempt > 1 {
				destinationRelative = filepath.Join(
					fmt.Sprintf("try-%03d", request.ExecutionAttempt), destinationRelative,
				)
			}
			destination, err := security.ResolveInside(request.EvidenceDir, destinationRelative)
			if err != nil {
				return err
			}
			hashValue, err := copyArtifact(source, destination)
			if err != nil {
				return err
			}
			resultArtifact := provider.Artifact{
				Path:   filepath.ToSlash(filepath.Join("artifacts", destinationRelative)),
				SHA256: hashValue,
			}
			for index := range result.Evidence {
				result.Evidence[index].Artifacts = append(result.Evidence[index].Artifacts, resultArtifact)
			}
		}
	}
	return nil
}

func copyArtifact(source, destination string) (string, error) {
	input, err := openFile(source)
	if err != nil {
		return "", err
	}
	defer input.Close()
	if err := makeDirectories(filepath.Dir(destination), 0o755); err != nil {
		return "", err
	}
	temporary, err := createTemporary(filepath.Dir(destination), ".artifact-*")
	if err != nil {
		return "", err
	}
	name := temporary.Name()
	defer os.Remove(name)
	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(temporary, hasher), input); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := renamePath(name, destination); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func safeSegment(value string) string {
	value = filepath.Base(filepath.Clean(value))
	if value == "." || value == ".." || value == string(filepath.Separator) {
		return "artifact"
	}
	return value
}

func skippedResult(current job, reason string) provider.Result {
	return provider.Result{
		Provider: current.spec.Provider, Status: "skipped", Diagnostics: []string{reason},
		Evidence: []provider.Evidence{{
			ID: current.key, Class: "deterministic", Summary: reason,
		}},
	}
}

func resultFailed(result provider.Result) bool {
	if result.Status != "completed" {
		return true
	}
	for _, evidence := range result.Evidence {
		if evidence.Data != nil && evidence.Data["retry_superseded"] == true {
			continue
		}
		if evidence.Passed == nil || !*evidence.Passed {
			return true
		}
	}
	return len(result.Evidence) == 0
}

func cacheSuccess(result provider.Result) bool {
	if result.Status != "completed" || len(result.Evidence) == 0 {
		return false
	}
	for _, evidence := range result.Evidence {
		if evidence.Data != nil && evidence.Data["retry_superseded"] == true {
			continue
		}
		if evidence.Class != "deterministic" || evidence.Passed == nil || !*evidence.Passed {
			return false
		}
	}
	return true
}

func cacheKey(request provider.Request, current job, providerVersion string, opt Options) (string, bool) {
	inputHash, err := hashInputs(opt.Root, current.spec.Inputs, opt.ChangedFiles, opt.HeadCommit, opt.DiffHash)
	if err != nil {
		return "", false
	}
	raw, err := json.Marshal(struct {
		Head, Diff, Requirement, Obligation, Plan, ProviderVersion, Environment, Inputs, Timeout string
		Spec                                                                                     ir.ProviderSpec
	}{
		Head: opt.HeadCommit, Diff: opt.DiffHash,
		Requirement: current.req.Hash, Obligation: current.obl.Hash,
		Plan: firstNonEmpty(opt.PlanHash, opt.IRHash), ProviderVersion: providerVersion,
		Environment: provider.EnvironmentFingerprint(request), Inputs: inputHash,
		Timeout: request.Timeout.String(), Spec: request.Spec,
	})
	if err != nil {
		return "", false
	}
	return hash(raw), true
}

func hashInputs(root string, patterns, changed []string, head, diff string) (string, error) {
	if len(patterns) == 0 {
		patterns = append([]string{}, changed...)
		if len(patterns) == 0 {
			return hash([]byte(head + "\x00" + diff)), nil
		}
	}
	var records []string
	for _, pattern := range patterns {
		matches, err := doublestar.FilepathGlob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			return "", err
		}
		if len(matches) == 0 {
			records = append(records, pattern+"\x00missing")
		}
		for _, path := range matches {
			info, err := lstatPath(path)
			if err != nil {
				return "", err
			}
			var content []byte
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := readlinkPath(path)
				if err != nil {
					return "", err
				}
				content = []byte("symlink:" + target)
			} else if !info.IsDir() {
				content, err = readPath(path)
				if err != nil {
					return "", err
				}
			}
			relative, _ := filepath.Rel(root, path)
			records = append(records, filepath.ToSlash(relative)+"\x00"+info.Mode().String()+"\x00"+hash(content))
		}
	}
	sort.Strings(records)
	raw, _ := json.Marshal(records)
	return hash(raw), nil
}

func loadCache(directory, key string) (provider.Result, bool) {
	if directory == "" {
		return provider.Result{}, false
	}
	raw, err := readPath(filepath.Join(directory, key+".json"))
	if err != nil {
		return provider.Result{}, false
	}
	var result provider.Result
	if json.Unmarshal(raw, &result) != nil || !cacheSuccess(result) {
		return provider.Result{}, false
	}
	return result, true
}

func saveCache(directory, key string, result provider.Result) error {
	if directory == "" {
		return nil
	}
	if err := makeDirectories(directory, 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	temporary, err := createTemporary(directory, ".cache-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return renamePath(name, filepath.Join(directory, key+".json"))
}

func hash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// assignLeafIDs returns a copy of n with unique provider leaf IDs filled in.
func assignLeafIDs(n ir.VerifyNode, counter *int) ir.VerifyNode {
	out := n
	if n.Provider != nil {
		spec := *n.Provider
		if spec.ID == "" {
			*counter++
			spec.ID = fmt.Sprintf("%s#%d", spec.Provider, *counter)
		}
		out.Provider = &spec
	}
	if len(n.All) > 0 {
		out.All = make([]ir.VerifyNode, len(n.All))
		for index, child := range n.All {
			out.All[index] = assignLeafIDs(child, counter)
		}
	}
	if len(n.Any) > 0 {
		out.Any = make([]ir.VerifyNode, len(n.Any))
		for index, child := range n.Any {
			out.Any[index] = assignLeafIDs(child, counter)
		}
	}
	if n.Not != nil {
		child := assignLeafIDs(*n.Not, counter)
		out.Not = &child
	}
	return out
}

func collectLeaves(n ir.VerifyNode, fn func(ir.ProviderSpec)) {
	if n.Provider != nil {
		fn(*n.Provider)
	}
	for _, child := range n.All {
		collectLeaves(child, fn)
	}
	for _, child := range n.Any {
		collectLeaves(child, fn)
	}
	if n.Not != nil {
		collectLeaves(*n.Not, fn)
	}
}

func split3(value string) []string {
	var output []string
	start := 0
	for index := 0; index < len(value); index++ {
		if value[index] == '/' {
			output = append(output, value[start:index])
			start = index + 1
			if len(output) == 2 {
				output = append(output, value[start:])
				return output
			}
		}
	}
	return nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func boolPtr(value bool) *bool { return &value }

// mutateResults is an optional test hook applied after provider execution.
var mutateResults func(map[string]provider.Result)

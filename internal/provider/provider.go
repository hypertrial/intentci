package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/hypertrial/intentci/internal/ir"
)

// Diagnostic is a provider validation issue.
type Diagnostic struct {
	Message string
}

// Request is passed to a provider execution.
type Request struct {
	RunID               string
	AttemptID           string
	ExecutionAttempt    int
	RequirementID       string
	ObligationID        string
	Root                string
	EvidenceDir         string
	BaseCommit          string
	HeadCommit          string
	DiffHash            string
	RequirementHash     string
	ObligationHash      string
	PlanHash            string
	EvidenceClass       string
	ConfidenceThreshold *float64
	ChangedFiles        []string
	Changes             []Change
	Spec                ir.ProviderSpec
	Timeout             time.Duration
	RetainStdout        bool
	RetainStderr        bool
}

// Change is the provider-facing repository change record.
type Change struct {
	Path      string
	OldPath   string
	Status    string
	Additions int
	Deletions int
	Binary    bool
	OldMode   string
	NewMode   string
}

// Result is normalized provider output.
type Result struct {
	Provider           string         `json:"provider"`
	ProviderVersion    string         `json:"provider_version"`
	Status             string         `json:"status"` // completed|error|skipped
	Evidence           []Evidence     `json:"evidence"`
	Diagnostics        []string       `json:"diagnostics,omitempty"`
	Stdout             string         `json:"stdout,omitempty"`
	Stderr             string         `json:"stderr,omitempty"`
	ExitCode           *int           `json:"exit_code,omitempty"`
	DurationMS         int64          `json:"duration_ms"`
	FromCache          bool           `json:"from_cache,omitempty"`
	SecurityViolation  bool           `json:"security_violation,omitempty"`
	SourceEvidenceHash string         `json:"source_evidence_hash,omitempty"`
	Extra              map[string]any `json:"extra,omitempty"`
}

// Evidence is a single evidence record.
type Evidence struct {
	SchemaVersion      string         `json:"schema_version,omitempty"`
	ID                 string         `json:"id"`
	RunID              string         `json:"run_id,omitempty"`
	AttemptID          string         `json:"attempt_id,omitempty"`
	RequirementID      string         `json:"requirement_id,omitempty"`
	ObligationID       string         `json:"obligation_id,omitempty"`
	VerifierID         string         `json:"verifier_id,omitempty"`
	Provider           string         `json:"provider,omitempty"`
	ProviderVersion    string         `json:"provider_version,omitempty"`
	Class              string         `json:"class"` // deterministic|probabilistic|human|informational
	Confidence         *float64       `json:"confidence,omitempty"`
	Strength           string         `json:"strength,omitempty"`
	Status             string         `json:"status,omitempty"`
	Summary            string         `json:"summary"`
	Paths              []string       `json:"paths,omitempty"`
	Passed             *bool          `json:"passed,omitempty"`
	Data               map[string]any `json:"data,omitempty"`
	RepositoryCommit   string         `json:"repository_commit,omitempty"`
	BaseCommit         string         `json:"base_commit,omitempty"`
	DiffHash           string         `json:"diff_hash,omitempty"`
	RequirementHash    string         `json:"requirement_hash,omitempty"`
	ObligationHash     string         `json:"obligation_hash,omitempty"`
	PlanHash           string         `json:"verification_plan_hash,omitempty"`
	StartedAt          time.Time      `json:"started_at,omitempty"`
	CompletedAt        time.Time      `json:"completed_at,omitempty"`
	SourceEvidenceHash string         `json:"source_evidence_hash,omitempty"`
	Artifacts          []Artifact     `json:"artifacts,omitempty"`
}

// Artifact identifies a collected evidence artifact.
type Artifact struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"media_type,omitempty"`
}

// Provider converts tools into evidence.
type Provider interface {
	Name() string
	Version() string
	Validate(spec ir.ProviderSpec) []Diagnostic
	Execute(ctx context.Context, req Request) Result
}

// Registry maps provider names to implementations.
type Registry struct {
	byName   map[string]Provider
	lookPath func(string) (string, error)
}

// NewRegistry returns a registry with built-in providers.
func NewRegistry(builtins ...Provider) *Registry {
	r := &Registry{byName: map[string]Provider{}, lookPath: exec.LookPath}
	for _, p := range builtins {
		r.byName[p.Name()] = p
	}
	return r
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.byName[name]
	if !ok && validProviderName(name) {
		if path, err := r.lookPath("intentci-provider-" + name); err == nil {
			return &ExternalProvider{ProviderName: name, Path: path}, true
		}
	}
	return p, ok
}

// DefaultRegistry returns all v1 built-in providers.
func DefaultRegistry() *Registry {
	return NewRegistry(
		&CommandProvider{},
		&BoundaryProvider{},
		&GitDiffProvider{},
		&JUnitProvider{},
		&SARIFProvider{},
		&JSONProvider{},
		&ManualProvider{},
	)
}

func boolPtr(b bool) *bool { return &b }

func validProviderName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func minimalEnvironment(req Request) []string {
	values := environmentValues(req)
	for name, value := range map[string]string{
		"INTENTCI_RUN_ID":           req.RunID,
		"INTENTCI_ATTEMPT_ID":       req.AttemptID,
		"INTENTCI_PROVIDER_ATTEMPT": fmt.Sprint(req.ExecutionAttempt),
		"INTENTCI_REQUIREMENT_ID":   req.RequirementID,
		"INTENTCI_OBLIGATION_ID":    req.ObligationID,
		"INTENTCI_BASE_COMMIT":      req.BaseCommit,
		"INTENTCI_HEAD_COMMIT":      req.HeadCommit,
		"INTENTCI_EVIDENCE_DIR":     req.EvidenceDir,
	} {
		values[name] = value
	}
	return sortedEnvironment(values)
}

func environmentValues(req Request) map[string]string {
	allowed := append([]string{"PATH", "TMPDIR", "TMP", "TEMP", "SYSTEMROOT", "COMSPEC"}, req.Spec.InheritEnv...)
	values := map[string]string{}
	for _, entry := range os.Environ() {
		name, value, ok := strings.Cut(entry, "=")
		if ok && matchesAny(allowed, name) {
			values[name] = value
		}
	}
	for name, value := range req.Spec.Environment {
		values[name] = value
	}
	return values
}

func sortedEnvironment(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	env := make([]string, 0, len(names))
	for _, name := range names {
		env = append(env, name+"="+values[name])
	}
	return env
}

// EnvironmentFingerprint hashes stable inherited and explicit environment,
// excluding run-specific variables injected by IntentCI.
func EnvironmentFingerprint(req Request) string {
	raw, _ := json.Marshal(sortedEnvironment(environmentValues(req)))
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func matchesAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if ok, _ := path.Match(pattern, value); ok {
			return true
		}
	}
	return false
}

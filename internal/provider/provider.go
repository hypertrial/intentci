package provider

import (
	"context"
	"time"

	"github.com/hypertrial/intentci/internal/ir"
)

// Diagnostic is a provider validation issue.
type Diagnostic struct {
	Message string
}

// Request is passed to a provider execution.
type Request struct {
	RunID         string
	RequirementID string
	ObligationID  string
	Root          string
	BaseCommit    string
	HeadCommit    string
	ChangedFiles  []string
	Spec          ir.ProviderSpec
	Timeout       time.Duration
	RetainStdout  bool
	RetainStderr  bool
}

// Result is normalized provider output.
type Result struct {
	Provider        string         `json:"provider"`
	ProviderVersion string         `json:"provider_version"`
	Status          string         `json:"status"` // completed|error|skipped
	Evidence        []Evidence     `json:"evidence"`
	Diagnostics     []string       `json:"diagnostics,omitempty"`
	Stdout          string         `json:"stdout,omitempty"`
	Stderr          string         `json:"stderr,omitempty"`
	ExitCode        *int           `json:"exit_code,omitempty"`
	DurationMS      int64          `json:"duration_ms"`
	FromCache       bool           `json:"from_cache,omitempty"`
	Extra           map[string]any `json:"extra,omitempty"`
}

// Evidence is a single evidence record.
type Evidence struct {
	ID       string         `json:"id"`
	Class    string         `json:"class"` // deterministic|probabilistic|manual
	Strength string         `json:"strength,omitempty"`
	Summary  string         `json:"summary"`
	Paths    []string       `json:"paths,omitempty"`
	Passed   *bool          `json:"passed,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
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
	byName map[string]Provider
}

// NewRegistry returns a registry with built-in providers.
func NewRegistry(builtins ...Provider) *Registry {
	r := &Registry{byName: map[string]Provider{}}
	for _, p := range builtins {
		r.byName[p.Name()] = p
	}
	return r
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.byName[name]
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

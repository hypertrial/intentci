package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// SchemaVersion is the Intent IR schema version.
const SchemaVersion = 1

// Document is the canonical compiled Intent IR.
type Document struct {
	SchemaVersion int           `json:"schema_version"`
	Project       string        `json:"project"`
	Hash          string        `json:"hash"`
	Requirements  []Requirement `json:"requirements"`
}

// VerificationPlan is the immutable executable subset selected for a run.
type VerificationPlan struct {
	SchemaVersion int               `json:"schema_version"`
	IRHash        string            `json:"ir_hash"`
	Hash          string            `json:"hash"`
	Requirements  []PlanRequirement `json:"requirements"`
}

// PlanRequirement records the obligations selected for a requirement.
type PlanRequirement struct {
	ID          string           `json:"id"`
	Hash        string           `json:"hash"`
	Obligations []PlanObligation `json:"obligations"`
}

// PlanObligation records an obligation and its verification expression.
type PlanObligation struct {
	ID     string     `json:"id"`
	Hash   string     `json:"hash"`
	Verify VerifyNode `json:"verify"`
}

// Requirement is a compiled requirement.
type Requirement struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Status      string       `json:"status"`
	Priority    string       `json:"priority"`
	Owners      []string     `json:"owners,omitempty"`
	DependsOn   []string     `json:"depends_on,omitempty"`
	AppliesTo   AppliesTo    `json:"applies_to"`
	Tags        []string     `json:"tags,omitempty"`
	Timeout     string       `json:"timeout,omitempty"`
	Intent      string       `json:"intent"`
	Rationale   string       `json:"rationale,omitempty"`
	Constraints []Constraint `json:"constraints,omitempty"`
	Boundaries  Boundaries   `json:"boundaries"`
	Obligations []Obligation `json:"obligations"`
	SourcePath  string       `json:"source_path"`
	Hash        string       `json:"hash"`
}

type AppliesTo struct {
	Paths   []string `json:"paths,omitempty"`
	Symbols []string `json:"symbols,omitempty"`
}

type Constraint struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"` // must | must_not
	Statement string `json:"statement"`
}

type Boundaries struct {
	Allowed   []string `json:"allowed,omitempty"`
	Forbidden []string `json:"forbidden,omitempty"`
}

type Obligation struct {
	ID                  string     `json:"id"`
	Statement           string     `json:"statement"`
	Required            bool       `json:"required"`
	Description         string     `json:"description,omitempty"`
	Rationale           string     `json:"rationale,omitempty"`
	EvidenceClass       string     `json:"evidence_class,omitempty"`
	ConfidenceThreshold *float64   `json:"confidence_threshold,omitempty"`
	Timeout             string     `json:"timeout,omitempty"`
	Retry               Retry      `json:"retry,omitempty"`
	Platforms           []string   `json:"platforms,omitempty"`
	Tags                []string   `json:"tags,omitempty"`
	DependsOn           []string   `json:"depends_on,omitempty"`
	ManualReview        bool       `json:"manual_review,omitempty"`
	Severity            string     `json:"severity,omitempty"`
	Verify              VerifyNode `json:"verify"`
	Hash                string     `json:"hash"`
}

// Retry configures repeated provider execution.
type Retry struct {
	Attempts int    `json:"attempts,omitempty" yaml:"attempts,omitempty"`
	Backoff  string `json:"backoff,omitempty" yaml:"backoff,omitempty"`
}

// VerifyNode is a logical verification expression.
type VerifyNode struct {
	All      []VerifyNode  `json:"all,omitempty"`
	Any      []VerifyNode  `json:"any,omitempty"`
	Not      *VerifyNode   `json:"not,omitempty"`
	Provider *ProviderSpec `json:"provider,omitempty"`
}

// ProviderSpec configures a single provider invocation.
type ProviderSpec struct {
	Provider         string            `json:"provider"`
	ID               string            `json:"id,omitempty"`
	Run              string            `json:"run,omitempty"`
	Report           string            `json:"report,omitempty"`
	Result           map[string]any    `json:"result,omitempty"`
	Allowed          []string          `json:"allowed,omitempty"`
	Forbidden        []string          `json:"forbidden,omitempty"`
	Paths            []string          `json:"paths,omitempty"`
	Expect           map[string]any    `json:"expect,omitempty"`
	Assert           map[string]any    `json:"assert,omitempty"`
	Match            map[string]any    `json:"match,omitempty"`
	Allow            map[string]any    `json:"allow,omitempty"`
	Prompt           string            `json:"prompt,omitempty"`
	WorkingDirectory string            `json:"working_directory,omitempty"`
	InheritEnv       []string          `json:"inherit_environment,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
	Timeout          string            `json:"timeout,omitempty"`
	Retry            Retry             `json:"retry,omitempty"`
	Inputs           []string          `json:"inputs,omitempty"`
	Outputs          []string          `json:"outputs,omitempty"`
	Artifacts        []string          `json:"artifacts,omitempty"`
	DependsOn        []string          `json:"depends_on,omitempty"`
	Exclusive        bool              `json:"exclusive,omitempty"`
	EvidenceClass    string            `json:"evidence_class,omitempty"`
	Configuration    map[string]any    `json:"configuration,omitempty"`
	Extra            map[string]any    `json:"extra,omitempty"`
}

var jsonMarshal = json.Marshal

// CanonicalJSON returns deterministic JSON bytes.
func CanonicalJSON(v any) ([]byte, error) {
	return jsonMarshal(v)
}

// HashBytes returns sha256 hex of data.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ComputeHashes fills requirement and obligation hashes and document hash.
func (d *Document) ComputeHashes() error {
	sort.SliceStable(d.Requirements, func(i, j int) bool {
		return d.Requirements[i].ID < d.Requirements[j].ID
	})
	for i := range d.Requirements {
		r := &d.Requirements[i]
		for j := range r.Obligations {
			o := &r.Obligations[j]
			clone := *o
			clone.Hash = ""
			b, err := CanonicalJSON(clone)
			if err != nil {
				return err
			}
			o.Hash = HashBytes(b)
		}
		clone := *r
		clone.Hash = ""
		b, err := CanonicalJSON(clone)
		if err != nil {
			return err
		}
		r.Hash = HashBytes(b)
	}
	clone := *d
	clone.Hash = ""
	b, err := CanonicalJSON(clone)
	if err != nil {
		return err
	}
	d.Hash = HashBytes(b)
	return nil
}

// ActiveRequirements returns requirements with status active.
func (d *Document) ActiveRequirements() []Requirement {
	out := make([]Requirement, 0, len(d.Requirements))
	for _, r := range d.Requirements {
		if r.Status == "active" {
			out = append(out, r)
		}
	}
	return out
}

// RequirementByID finds a requirement by id.
func (d *Document) RequirementByID(id string) *Requirement {
	for i := range d.Requirements {
		if d.Requirements[i].ID == id {
			return &d.Requirements[i]
		}
	}
	return nil
}

// BuildVerificationPlan constructs and hashes a canonical plan.
func BuildVerificationPlan(document *Document, requirements []Requirement) (*VerificationPlan, error) {
	plan := &VerificationPlan{
		SchemaVersion: SchemaVersion, IRHash: document.Hash,
		Requirements: make([]PlanRequirement, 0, len(requirements)),
	}
	for _, requirement := range requirements {
		item := PlanRequirement{ID: requirement.ID, Hash: requirement.Hash}
		for _, obligation := range requirement.Obligations {
			counter := 0
			item.Obligations = append(item.Obligations, PlanObligation{
				ID: obligation.ID, Hash: obligation.Hash,
				Verify: normalizeVerifierIDs(obligation.Verify, &counter),
			})
		}
		plan.Requirements = append(plan.Requirements, item)
	}
	sort.SliceStable(plan.Requirements, func(i, j int) bool {
		return plan.Requirements[i].ID < plan.Requirements[j].ID
	})
	clone := *plan
	clone.Hash = ""
	raw, err := CanonicalJSON(clone)
	if err != nil {
		return nil, err
	}
	plan.Hash = HashBytes(raw)
	return plan, nil
}

func normalizeVerifierIDs(node VerifyNode, counter *int) VerifyNode {
	output := node
	if node.Provider != nil {
		spec := *node.Provider
		if spec.ID == "" {
			*counter++
			spec.ID = spec.Provider + "#" + fmt.Sprint(*counter)
		}
		output.Provider = &spec
	}
	for index, child := range node.All {
		if index == 0 {
			output.All = make([]VerifyNode, len(node.All))
		}
		output.All[index] = normalizeVerifierIDs(child, counter)
	}
	for index, child := range node.Any {
		if index == 0 {
			output.Any = make([]VerifyNode, len(node.Any))
		}
		output.Any[index] = normalizeVerifierIDs(child, counter)
	}
	if node.Not != nil {
		child := normalizeVerifierIDs(*node.Not, counter)
		output.Not = &child
	}
	return output
}

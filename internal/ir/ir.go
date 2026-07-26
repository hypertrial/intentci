package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	ID       string      `json:"id"`
	Statement string     `json:"statement"`
	Required bool        `json:"required"`
	Verify   VerifyNode  `json:"verify"`
	Hash     string      `json:"hash"`
}

// VerifyNode is a logical verification expression.
type VerifyNode struct {
	All      []VerifyNode   `json:"all,omitempty"`
	Any      []VerifyNode   `json:"any,omitempty"`
	Not      *VerifyNode    `json:"not,omitempty"`
	Provider *ProviderSpec  `json:"provider,omitempty"`
}

// ProviderSpec configures a single provider invocation.
type ProviderSpec struct {
	Provider string         `json:"provider"`
	ID       string         `json:"id,omitempty"`
	Run      string         `json:"run,omitempty"`
	Report   string         `json:"report,omitempty"`
	Result   map[string]any `json:"result,omitempty"`
	Allowed  []string       `json:"allowed,omitempty"`
	Forbidden []string      `json:"forbidden,omitempty"`
	Paths    []string       `json:"paths,omitempty"`
	Expect   map[string]any `json:"expect,omitempty"`
	Assert   map[string]any `json:"assert,omitempty"`
	Extra    map[string]any `json:"extra,omitempty"`
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

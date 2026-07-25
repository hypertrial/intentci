// Package semantic implements optional local/HTTP semantic verification providers.
package semantic

// ProtocolVersion is the provider request/response protocol version.
const ProtocolVersion = 1

// MaxInputBytes bounds diffs and snippets sent to a provider.
const MaxInputBytes = 256 * 1024

// Assessment values from the provider protocol.
const (
	AssessmentAligned               = "aligned"
	AssessmentContradiction         = "contradiction"
	AssessmentInsufficientEvidence  = "insufficient_evidence"
	AssessmentUncertain             = "uncertain"
	AssessmentNotAffected           = "not_affected"
)

// Request is the versioned JSON payload sent to a semantic provider.
type Request struct {
	ProtocolVersion int                   `json:"protocol_version"`
	Profile         string                `json:"profile"`
	BaseCommit      string                `json:"base_commit"`
	HeadCommit      string                `json:"head_commit"`
	Product         ProductContext        `json:"product"`
	Change          *ChangeContext        `json:"change,omitempty"`
	Requirements    []RequirementContext  `json:"requirements"`
	ChangedFiles    []string              `json:"changed_files"`
	Diff            string                `json:"diff"`
	Snippets        []FileSnippet         `json:"snippets,omitempty"`
	CheckResults    []CheckSummary        `json:"check_results"`
}

// ProductContext is product metadata included in the request.
type ProductContext struct {
	Name     string   `json:"name"`
	Purpose  string   `json:"purpose"`
	NonGoals []string `json:"non_goals,omitempty"`
}

// ChangeContext is Change Spec goals/non-goals for semantic input.
type ChangeContext struct {
	ID       string   `json:"id"`
	Goals    []string `json:"goals,omitempty"`
	NonGoals []string `json:"non_goals,omitempty"`
}

// RequirementContext is an approved requirement for semantic review.
type RequirementContext struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Statement    string   `json:"statement"`
	Status       string   `json:"status"`
	Severity     string   `json:"severity"`
	Semantic     string   `json:"semantic"`
	Checks       []string `json:"checks"`
	SourcePaths  []string `json:"source_paths,omitempty"`
}

// FileSnippet is a truncated file excerpt.
type FileSnippet struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// CheckSummary is a deterministic check outcome for semantic input.
type CheckSummary struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// Response is the versioned JSON payload returned by a provider.
type Response struct {
	ProtocolVersion int       `json:"protocol_version"`
	Findings        []Finding `json:"findings"`
}

// Finding is one semantic assessment for a requirement.
type Finding struct {
	RequirementID   string          `json:"requirement_id"`
	Assessment      string          `json:"assessment"`
	Confidence      float64         `json:"confidence"`
	Summary         string          `json:"summary"`
	Evidence        []EvidenceCite  `json:"evidence,omitempty"`
	MissingEvidence []string        `json:"missing_evidence,omitempty"`
}

// EvidenceCite points at repository evidence.
type EvidenceCite struct {
	Path      string `json:"path"`
	LineStart int    `json:"line_start,omitempty"`
	LineEnd   int    `json:"line_end,omitempty"`
}

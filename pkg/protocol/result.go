// Package protocol defines the versioned machine-readable result schema.
package protocol

// SchemaVersion is the JSON result schema version for IntentCI v1 protocol.
const SchemaVersion = 1

// Overall status values.
const (
	StatusPass       = "pass"
	StatusFail       = "fail"
	StatusUnverified = "unverified"
	StatusUnknown    = "unknown"
)

// Requirement-level status values (uppercase in text; lowercase in JSON).
const (
	ReqPass        = "pass"
	ReqFail        = "fail"
	ReqUnverified  = "unverified"
	ReqUnknown     = "unknown"
	ReqWaived      = "waived"
	ReqNotAffected = "not_affected"
)

// CheckStatus values.
const (
	CheckPass    = "pass"
	CheckFail    = "fail"
	CheckUnknown = "unknown"
	CheckSkipped = "skipped"
	CheckCached  = "cached"
)

// Result is the top-level verification report.
type Result struct {
	SchemaVersion    int                 `json:"schema_version"`
	RunID            string              `json:"run_id"`
	Status           string              `json:"status"`
	BaseCommit       string              `json:"base_commit"`
	HeadCommit       string              `json:"head_commit"`
	ContractHash     string              `json:"contract_hash"`
	WorkingTreeDirty bool                `json:"working_tree_dirty"`
	Profile          string              `json:"profile"`
	ChangeSpec       *ChangeSpecRef      `json:"change_spec"`
	Requirements     []RequirementResult `json:"requirements"`
	Checks           []CheckResult       `json:"checks"`
	Waivers          []Waiver            `json:"waivers"`
	ContractChanges  []ContractChange    `json:"contract_changes"`
	ChangeFindings   []ChangeFinding     `json:"change_findings"`
	Summary          Summary             `json:"summary"`
}

// Waiver records an applied Change Spec waiver.
type Waiver struct {
	ID          string `json:"id"`
	Requirement string `json:"requirement"`
	Reason      string `json:"reason"`
	Owner       string `json:"owner,omitempty"`
	Approver    string `json:"approver,omitempty"`
	Expires     string `json:"expires"`
}

// ContractChange reports Product Contract weakening relative to the base commit.
type ContractChange struct {
	Type    string `json:"type"`
	Summary string `json:"summary"`
	ID      string `json:"id,omitempty"`
}

// ChangeSpecRef identifies the selected Change Spec.
type ChangeSpecRef struct {
	ID   string `json:"id"`
	Hash string `json:"hash"`
}

// ChangeFinding reports Change Spec mutations.
type ChangeFinding struct {
	Type    string `json:"type"`
	Summary string `json:"summary"`
}

// RequirementResult is one requirement's verification outcome.
type RequirementResult struct {
	ID         string     `json:"id"`
	Title      string     `json:"title,omitempty"`
	Status     string     `json:"status"`
	Severity   string     `json:"severity"`
	AffectedBy []string   `json:"affected_by"`
	Checks     []CheckRef `json:"checks"`
	Evidence   []Evidence `json:"evidence"`
	Findings   []Finding  `json:"findings"`
	Reason     string     `json:"reason,omitempty"`
}

// CheckRef links a requirement to a check outcome.
type CheckRef struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

// CheckResult is a check execution record.
type CheckResult struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	Reason     string `json:"reason,omitempty"`
	FromCache  bool   `json:"from_cache,omitempty"`
}

// Evidence cites repository evidence for a status.
type Evidence struct {
	Type      string `json:"type"`
	Path      string `json:"path,omitempty"`
	LineStart int    `json:"line_start,omitempty"`
	LineEnd   int    `json:"line_end,omitempty"`
	Summary   string `json:"summary,omitempty"`
}

// Finding explains a non-pass outcome.
type Finding struct {
	Type    string `json:"type"`
	Summary string `json:"summary"`
}

// Summary counts requirement statuses.
type Summary struct {
	Pass           int `json:"pass"`
	Fail           int `json:"fail"`
	Unverified     int `json:"unverified"`
	Unknown        int `json:"unknown"`
	Waived         int `json:"waived"`
	NotAffected    int `json:"not_affected"`
	ChecksExecuted int `json:"checks_executed"`
	ChecksCached   int `json:"checks_cached"`
}

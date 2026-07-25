package changespec

// Spec is a Change Spec (version 1).
type Spec struct {
	Version               int               `yaml:"version" json:"version"`
	ID                    string            `yaml:"id" json:"id"`
	Status                string            `yaml:"status" json:"status"`
	Type                  string            `yaml:"type" json:"type"`
	Summary               string            `yaml:"summary" json:"summary"`
	Source                *Source           `yaml:"source" json:"source"`
	Goals                 []string          `yaml:"goals" json:"goals"`
	NonGoals              []string          `yaml:"non_goals" json:"non_goals"`
	Acceptance            []Acceptance      `yaml:"acceptance" json:"acceptance"`
	AffectedRequirements  []string          `yaml:"affected_requirements" json:"affected_requirements"`
	RequiredChecks        []string          `yaml:"required_checks" json:"required_checks"`
	Waivers               []Waiver          `yaml:"waivers" json:"waivers"`
}

// Waiver is an explicit, time-bounded skip for a requirement or acceptance criterion.
type Waiver struct {
	ID          string `yaml:"id" json:"id"`
	Requirement string `yaml:"requirement" json:"requirement"`
	Reason      string `yaml:"reason" json:"reason"`
	Owner       string `yaml:"owner" json:"owner"`
	Approver    string `yaml:"approver" json:"approver"`
	Expires     string `yaml:"expires" json:"expires"`
}

// Source cites external provenance.
type Source struct {
	Type      string `yaml:"type" json:"type"`
	Reference string `yaml:"reference" json:"reference"`
}

// Acceptance is a temporary requirement for the change.
type Acceptance struct {
	ID           string       `yaml:"id" json:"id"`
	Statement    string       `yaml:"statement" json:"statement"`
	Severity     string       `yaml:"severity" json:"severity"`
	Verification Verification `yaml:"verification" json:"verification"`
}

// Verification maps acceptance to checks.
type Verification struct {
	Checks   []string `yaml:"checks" json:"checks"`
	Semantic string   `yaml:"semantic" json:"semantic"`
}

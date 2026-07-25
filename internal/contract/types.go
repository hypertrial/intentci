package contract

import "time"

// Contract is the Product Contract (version 1).
type Contract struct {
	Version      int          `yaml:"version" json:"version"`
	Product      Product      `yaml:"product" json:"product"`
	Policy       Policy       `yaml:"policy" json:"policy"`
	Execution    Execution    `yaml:"execution" json:"execution"`
	Environment  Environment  `yaml:"environment" json:"environment"`
	Requirements []Requirement `yaml:"requirements" json:"requirements"`
	Checks       []Check      `yaml:"checks" json:"checks"`
}

// Product describes the repository product.
type Product struct {
	Name     string   `yaml:"name" json:"name"`
	Purpose  string   `yaml:"purpose" json:"purpose"`
	NonGoals []string `yaml:"non_goals" json:"non_goals"`
}

// Policy controls verification gating.
type Policy struct {
	DefaultBase      string         `yaml:"default_base" json:"default_base"`
	UnknownBlocks    *bool          `yaml:"unknown_blocks" json:"unknown_blocks"`
	UnverifiedBlocks *bool          `yaml:"unverified_blocks" json:"unverified_blocks"`
	Semantic         SemanticPolicy `yaml:"semantic" json:"semantic"`
}

// SemanticPolicy is accepted but unused in v0.1.0.
type SemanticPolicy struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	Enforcement string `yaml:"enforcement" json:"enforcement"`
}

// Execution controls check scheduling.
type Execution struct {
	MaxParallel int `yaml:"max_parallel" json:"max_parallel"`
}

// Environment lists env vars included in future cache keys.
type Environment struct {
	Include []string `yaml:"include" json:"include"`
}

// Requirement is a product requirement.
type Requirement struct {
	ID           string       `yaml:"id" json:"id"`
	Type         string       `yaml:"type" json:"type"`
	Title        string       `yaml:"title" json:"title"`
	Statement    string       `yaml:"statement" json:"statement"`
	Status       string       `yaml:"status" json:"status"`
	Severity     string       `yaml:"severity" json:"severity"`
	Sources      []Source     `yaml:"sources" json:"sources"`
	AppliesTo    AppliesTo    `yaml:"applies_to" json:"applies_to"`
	Verification Verification `yaml:"verification" json:"verification"`
}

// Source cites requirement provenance.
type Source struct {
	Path        string `yaml:"path" json:"path"`
	Description string `yaml:"description" json:"description"`
}

// AppliesTo maps requirements to paths.
type AppliesTo struct {
	Include []string `yaml:"include" json:"include"`
	Exclude []string `yaml:"exclude" json:"exclude"`
}

// Verification maps a requirement to checks.
type Verification struct {
	Mode     string   `yaml:"mode" json:"mode"`
	Checks   []string `yaml:"checks" json:"checks"`
	Semantic string   `yaml:"semantic" json:"semantic"`
}

// Check is a repository-native command.
type Check struct {
	ID          string   `yaml:"id" json:"id"`
	Description string   `yaml:"description" json:"description"`
	Command     string   `yaml:"command" json:"command"`
	Profiles    []string `yaml:"profiles" json:"profiles"`
	Inputs      []string `yaml:"inputs" json:"inputs"`
	Timeout     string   `yaml:"timeout" json:"timeout"`
	Cache       string   `yaml:"cache" json:"cache"`
	DependsOn   []string `yaml:"depends_on" json:"depends_on"`
	Exclusive   bool     `yaml:"exclusive" json:"exclusive"`
	Results     *Results `yaml:"results" json:"results"`
}

// Results is optional structured result config (unused in v0.1.0).
type Results struct {
	Format string `yaml:"format" json:"format"`
	Path   string `yaml:"path" json:"path"`
}

// DefaultTimeout is used when a check omits timeout.
const DefaultTimeout = 15 * time.Minute

// BlocksOnUnknown returns whether unknown status blocks (default true).
func (p Policy) BlocksOnUnknown() bool {
	if p.UnknownBlocks == nil {
		return true
	}
	return *p.UnknownBlocks
}

// BlocksOnUnverified returns whether unverified status blocks (default true).
func (p Policy) BlocksOnUnverified() bool {
	if p.UnverifiedBlocks == nil {
		return true
	}
	return *p.UnverifiedBlocks
}

// DefaultBaseOr returns the configured base or a fallback.
func (p Policy) DefaultBaseOr(fallback string) string {
	if p.DefaultBase != "" {
		return p.DefaultBase
	}
	return fallback
}

// MaxParallelOr returns concurrency or a fallback.
func (e Execution) MaxParallelOr(fallback int) int {
	if e.MaxParallel > 0 {
		return e.MaxParallel
	}
	return fallback
}

// ParseTimeout parses a check timeout string.
func ParseTimeout(s string) (time.Duration, error) {
	if s == "" {
		return DefaultTimeout, nil
	}
	return time.ParseDuration(s)
}

// HasProfile reports whether the check belongs to a profile.
// Empty profiles means both fast and full.
func (c Check) HasProfile(profile string) bool {
	if len(c.Profiles) == 0 {
		return true
	}
	for _, p := range c.Profiles {
		if p == profile {
			return true
		}
	}
	return false
}

// VerificationMode returns "all" by default.
func (v Verification) VerificationMode() string {
	if v.Mode == "" {
		return "all"
	}
	return v.Mode
}

// ApprovedBlocking returns approved blocking requirements.
func (c *Contract) ApprovedBlocking() []Requirement {
	var out []Requirement
	for _, r := range c.Requirements {
		if r.Status == "approved" && r.Severity == "blocking" {
			out = append(out, r)
		}
	}
	return out
}

// CheckByID returns a check by id.
func (c *Contract) CheckByID(id string) (Check, bool) {
	for _, ch := range c.Checks {
		if ch.ID == id {
			return ch, true
		}
	}
	return Check{}, false
}

// CheckMap returns checks indexed by id.
func (c *Contract) CheckMap() map[string]Check {
	m := make(map[string]Check, len(c.Checks))
	for _, ch := range c.Checks {
		m[ch.ID] = ch
	}
	return m
}

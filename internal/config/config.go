package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

const (
	DirName        = ".intentci"
	ConfigFileName = "config.yaml"
	LocalFileName  = "config.local.yaml"
)

// Config is the project configuration (.intentci/config.yaml).
type Config struct {
	Version      int             `yaml:"version" json:"version"`
	Project      Project         `yaml:"project" json:"project"`
	Requirements RequirementsCfg `yaml:"requirements" json:"requirements"`
	Verification VerificationCfg `yaml:"verification" json:"verification"`
	ChangeImpact ChangeImpactCfg `yaml:"change_impact" json:"change_impact"`
	Evidence     EvidenceCfg     `yaml:"evidence" json:"evidence"`
	Repair       RepairCfg       `yaml:"repair" json:"repair"`
	CI           CICfg           `yaml:"ci" json:"ci"`
	Telemetry    TelemetryCfg    `yaml:"telemetry" json:"telemetry"`
}

type Project struct {
	Name string `yaml:"name" json:"name"`
}

type RequirementsCfg struct {
	Paths []string `yaml:"paths" json:"paths"`
}

type VerificationCfg struct {
	DefaultTimeout       string `yaml:"default_timeout" json:"default_timeout"`
	MaxParallel          int    `yaml:"max_parallel" json:"max_parallel"`
	FailFast             bool   `yaml:"fail_fast" json:"fail_fast"`
	WorkingDirectory     string `yaml:"working_directory" json:"working_directory"`
	RequireCleanWorktree bool   `yaml:"require_clean_worktree" json:"require_clean_worktree"`
}

type ChangeImpactCfg struct {
	BaseRef                 string   `yaml:"base_ref" json:"base_ref"`
	IncludeUntracked        bool     `yaml:"include_untracked" json:"include_untracked"`
	RunUnmappedRequirements bool     `yaml:"run_unmapped_requirements" json:"run_unmapped_requirements"`
	FailOnUnmapped          bool     `yaml:"fail_on_unmapped" json:"fail_on_unmapped"`
	GlobalPaths             []string `yaml:"global_paths" json:"global_paths"`
}

type EvidenceCfg struct {
	Directory     string    `yaml:"directory" json:"directory"`
	RetainStdout  bool      `yaml:"retain_stdout" json:"retain_stdout"`
	RetainStderr  bool      `yaml:"retain_stderr" json:"retain_stderr"`
	HashAlgorithm string    `yaml:"hash_algorithm" json:"hash_algorithm"`
	Redact        RedactCfg `yaml:"redact" json:"redact"`
}

type RedactCfg struct {
	Environment []string `yaml:"environment" json:"environment"`
}

type RepairCfg struct {
	MaxAttempts             int      `yaml:"max_attempts" json:"max_attempts"`
	StopOnRepeatedDiff      bool     `yaml:"stop_on_repeated_diff" json:"stop_on_repeated_diff"`
	StopOnRepeatedFailure   bool     `yaml:"stop_on_repeated_failure" json:"stop_on_repeated_failure"`
	AllowRequirementChanges bool     `yaml:"allow_requirement_changes" json:"allow_requirement_changes"`
	AllowTestChanges        bool     `yaml:"allow_test_changes" json:"allow_test_changes"`
	ProtectedPaths          []string `yaml:"protected_paths" json:"protected_paths"`
}

type CICfg struct {
	FailOn []string `yaml:"fail_on" json:"fail_on"`
}

type TelemetryCfg struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// Default returns built-in defaults.
func Default() *Config {
	return &Config{
		Version: 1,
		Project: Project{Name: "project"},
		Requirements: RequirementsCfg{
			Paths: []string{".intentci/requirements/**/*.md"},
		},
		Verification: VerificationCfg{
			DefaultTimeout:   "10m",
			MaxParallel:      4,
			WorkingDirectory: ".",
		},
		ChangeImpact: ChangeImpactCfg{
			BaseRef:          "origin/main",
			IncludeUntracked: true,
			GlobalPaths: []string{
				".intentci/config.yaml", ".intentci/providers/**", ".intentci/schemas/**",
				"go.mod", "go.sum", "package.json", "package-lock.json", "pnpm-lock.yaml",
				"yarn.lock", "pyproject.toml", "requirements*.txt", "uv.lock",
				"Cargo.toml", "Cargo.lock", "pom.xml", "build.gradle*", "gradle.lockfile",
			},
		},
		Evidence: EvidenceCfg{
			Directory:     ".intentci/runs",
			RetainStdout:  true,
			RetainStderr:  true,
			HashAlgorithm: "sha256",
			Redact: RedactCfg{
				Environment: []string{"*TOKEN*", "*SECRET*", "*PASSWORD*", "*KEY*"},
			},
		},
		Repair: RepairCfg{
			MaxAttempts:           3,
			StopOnRepeatedDiff:    true,
			StopOnRepeatedFailure: true,
			AllowTestChanges:      true,
		},
		CI: CICfg{
			FailOn: []string{"fail", "error", "unproven", "uncertain", "review_required"},
		},
		Telemetry: TelemetryCfg{Enabled: false},
	}
}

// Dir returns the .intentci directory under root.
func Dir(root string) string {
	return filepath.Join(root, DirName)
}

// Path returns the primary config path.
func Path(root string) string {
	return filepath.Join(Dir(root), ConfigFileName)
}

// Load reads config.yaml and optional config.local.yaml from root.
func Load(root string) (*Config, error) {
	cfg := Default()
	primary := Path(root)
	data, err := os.ReadFile(primary)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := decode(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	local := filepath.Join(Dir(root), LocalFileName)
	if b, err := os.ReadFile(local); err == nil {
		if err := decode(b, cfg); err != nil {
			return nil, fmt.Errorf("parse config.local.yaml: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config.local.yaml: %w", err)
	}
	if err := applyEnvironment(cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks required fields and timeouts.
func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version %d (want 1)", c.Version)
	}
	if c.Project.Name == "" {
		return fmt.Errorf("project.name is required")
	}
	if len(c.Requirements.Paths) == 0 {
		return fmt.Errorf("requirements.paths must not be empty")
	}
	patterns := append(append([]string{}, c.Requirements.Paths...), c.ChangeImpact.GlobalPaths...)
	patterns = append(patterns, c.Repair.ProtectedPaths...)
	for _, pattern := range patterns {
		if !validRelative(pattern) || !doublestar.ValidatePattern(filepath.ToSlash(pattern)) {
			return fmt.Errorf("invalid path pattern %q", pattern)
		}
	}
	for _, pattern := range c.Evidence.Redact.Environment {
		if !doublestar.ValidatePattern(pattern) {
			return fmt.Errorf("invalid evidence.redact.environment pattern %q", pattern)
		}
	}
	if _, err := ParseDuration(c.Verification.DefaultTimeout); err != nil {
		return fmt.Errorf("verification.default_timeout: %w", err)
	}
	if c.Verification.MaxParallel < 0 {
		return fmt.Errorf("verification.max_parallel must be >= 0")
	}
	if !validRelative(c.Verification.WorkingDirectory) {
		return fmt.Errorf("verification.working_directory must be repository-relative")
	}
	if c.Evidence.Directory == "" {
		return fmt.Errorf("evidence.directory is required")
	}
	if !validRelative(c.Evidence.Directory) {
		return fmt.Errorf("evidence.directory must be repository-relative")
	}
	if c.Evidence.HashAlgorithm != "sha256" {
		return fmt.Errorf("evidence.hash_algorithm must be sha256")
	}
	if c.Repair.MaxAttempts < 1 {
		return fmt.Errorf("repair.max_attempts must be >= 1")
	}
	for _, value := range c.CI.FailOn {
		if !oneOf(value, "fail", "error", "unproven", "uncertain", "review_required") {
			return fmt.Errorf("ci.fail_on contains invalid verdict %q", value)
		}
	}
	return nil
}

func validRelative(value string) bool {
	if value == "" || filepath.IsAbs(value) {
		return false
	}
	clean := filepath.Clean(value)
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func decode(data []byte, out any) error {
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	return dec.Decode(out)
}

var lookupEnv = os.LookupEnv

func applyEnvironment(c *Config) error {
	stringValue := func(name string, dst *string) {
		if value, ok := lookupEnv(name); ok {
			*dst = value
		}
	}
	boolValue := func(name string, dst *bool) error {
		value, ok := lookupEnv(name)
		if !ok {
			return nil
		}
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		*dst = parsed
		return nil
	}
	intValue := func(name string, dst *int) error {
		value, ok := lookupEnv(name)
		if !ok {
			return nil
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		*dst = parsed
		return nil
	}
	listValue := func(name string, dst *[]string) error {
		value, ok := lookupEnv(name)
		if !ok {
			return nil
		}
		if err := json.Unmarshal([]byte(value), dst); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	}

	if err := intValue("INTENTCI_VERSION", &c.Version); err != nil {
		return err
	}
	stringValue("INTENTCI_PROJECT_NAME", &c.Project.Name)
	if err := listValue("INTENTCI_REQUIREMENTS_PATHS", &c.Requirements.Paths); err != nil {
		return err
	}
	stringValue("INTENTCI_VERIFICATION_DEFAULT_TIMEOUT", &c.Verification.DefaultTimeout)
	if err := intValue("INTENTCI_VERIFICATION_MAX_PARALLEL", &c.Verification.MaxParallel); err != nil {
		return err
	}
	if err := boolValue("INTENTCI_VERIFICATION_FAIL_FAST", &c.Verification.FailFast); err != nil {
		return err
	}
	stringValue("INTENTCI_VERIFICATION_WORKING_DIRECTORY", &c.Verification.WorkingDirectory)
	if err := boolValue("INTENTCI_VERIFICATION_REQUIRE_CLEAN_WORKTREE", &c.Verification.RequireCleanWorktree); err != nil {
		return err
	}
	stringValue("INTENTCI_CHANGE_IMPACT_BASE_REF", &c.ChangeImpact.BaseRef)
	for _, item := range []struct {
		name string
		dst  *bool
	}{
		{"INTENTCI_CHANGE_IMPACT_INCLUDE_UNTRACKED", &c.ChangeImpact.IncludeUntracked},
		{"INTENTCI_CHANGE_IMPACT_RUN_UNMAPPED_REQUIREMENTS", &c.ChangeImpact.RunUnmappedRequirements},
		{"INTENTCI_CHANGE_IMPACT_FAIL_ON_UNMAPPED", &c.ChangeImpact.FailOnUnmapped},
	} {
		if err := boolValue(item.name, item.dst); err != nil {
			return err
		}
	}
	if err := listValue("INTENTCI_CHANGE_IMPACT_GLOBAL_PATHS", &c.ChangeImpact.GlobalPaths); err != nil {
		return err
	}
	stringValue("INTENTCI_EVIDENCE_DIRECTORY", &c.Evidence.Directory)
	if err := boolValue("INTENTCI_EVIDENCE_RETAIN_STDOUT", &c.Evidence.RetainStdout); err != nil {
		return err
	}
	if err := boolValue("INTENTCI_EVIDENCE_RETAIN_STDERR", &c.Evidence.RetainStderr); err != nil {
		return err
	}
	stringValue("INTENTCI_EVIDENCE_HASH_ALGORITHM", &c.Evidence.HashAlgorithm)
	if err := listValue("INTENTCI_EVIDENCE_REDACT_ENVIRONMENT", &c.Evidence.Redact.Environment); err != nil {
		return err
	}
	if err := intValue("INTENTCI_REPAIR_MAX_ATTEMPTS", &c.Repair.MaxAttempts); err != nil {
		return err
	}
	for _, item := range []struct {
		name string
		dst  *bool
	}{
		{"INTENTCI_REPAIR_STOP_ON_REPEATED_DIFF", &c.Repair.StopOnRepeatedDiff},
		{"INTENTCI_REPAIR_STOP_ON_REPEATED_FAILURE", &c.Repair.StopOnRepeatedFailure},
		{"INTENTCI_REPAIR_ALLOW_REQUIREMENT_CHANGES", &c.Repair.AllowRequirementChanges},
		{"INTENTCI_REPAIR_ALLOW_TEST_CHANGES", &c.Repair.AllowTestChanges},
	} {
		if err := boolValue(item.name, item.dst); err != nil {
			return err
		}
	}
	if err := listValue("INTENTCI_REPAIR_PROTECTED_PATHS", &c.Repair.ProtectedPaths); err != nil {
		return err
	}
	if err := listValue("INTENTCI_CI_FAIL_ON", &c.CI.FailOn); err != nil {
		return err
	}
	return boolValue("INTENTCI_TELEMETRY_ENABLED", &c.Telemetry.Enabled)
}

// ParseDuration parses a Go duration or returns an error.
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 10 * time.Minute, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return d, nil
}

// MaxParallelOr returns MaxParallel or a fallback.
func (c *Config) MaxParallelOr(fallback int) int {
	if c.Verification.MaxParallel > 0 {
		return c.Verification.MaxParallel
	}
	if fallback > 0 {
		return fallback
	}
	return 4
}

// BaseRefOr returns the configured base ref or default.
func (c *Config) BaseRefOr(def string) string {
	if c.ChangeImpact.BaseRef != "" {
		return c.ChangeImpact.BaseRef
	}
	return def
}

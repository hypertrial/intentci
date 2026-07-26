package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DirName        = ".intentci"
	ConfigFileName = "config.yaml"
	LocalFileName  = "config.local.yaml"
)

// Config is the project configuration (.intentci/config.yaml).
type Config struct {
	Version      int              `yaml:"version" json:"version"`
	Project      Project          `yaml:"project" json:"project"`
	Requirements RequirementsCfg  `yaml:"requirements" json:"requirements"`
	Verification VerificationCfg  `yaml:"verification" json:"verification"`
	ChangeImpact ChangeImpactCfg  `yaml:"change_impact" json:"change_impact"`
	Evidence     EvidenceCfg      `yaml:"evidence" json:"evidence"`
	Repair       RepairCfg        `yaml:"repair" json:"repair"`
	CI           CICfg            `yaml:"ci" json:"ci"`
	Telemetry    TelemetryCfg     `yaml:"telemetry" json:"telemetry"`
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
	BaseRef                 string `yaml:"base_ref" json:"base_ref"`
	IncludeUntracked        bool   `yaml:"include_untracked" json:"include_untracked"`
	RunUnmappedRequirements bool   `yaml:"run_unmapped_requirements" json:"run_unmapped_requirements"`
}

type EvidenceCfg struct {
	Directory     string   `yaml:"directory" json:"directory"`
	RetainStdout  bool     `yaml:"retain_stdout" json:"retain_stdout"`
	RetainStderr  bool     `yaml:"retain_stderr" json:"retain_stderr"`
	HashAlgorithm string   `yaml:"hash_algorithm" json:"hash_algorithm"`
	Redact        RedactCfg `yaml:"redact" json:"redact"`
}

type RedactCfg struct {
	Environment []string `yaml:"environment" json:"environment"`
}

type RepairCfg struct {
	MaxAttempts              int  `yaml:"max_attempts" json:"max_attempts"`
	StopOnRepeatedDiff       bool `yaml:"stop_on_repeated_diff" json:"stop_on_repeated_diff"`
	StopOnRepeatedFailure    bool `yaml:"stop_on_repeated_failure" json:"stop_on_repeated_failure"`
	AllowRequirementChanges  bool `yaml:"allow_requirement_changes" json:"allow_requirement_changes"`
	AllowTestChanges         bool `yaml:"allow_test_changes" json:"allow_test_changes"`
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
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	local := filepath.Join(Dir(root), LocalFileName)
	if b, err := os.ReadFile(local); err == nil {
		if err := yaml.Unmarshal(b, cfg); err != nil {
			return nil, fmt.Errorf("parse config.local.yaml: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config.local.yaml: %w", err)
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
	if _, err := ParseDuration(c.Verification.DefaultTimeout); err != nil {
		return fmt.Errorf("verification.default_timeout: %w", err)
	}
	if c.Verification.MaxParallel < 0 {
		return fmt.Errorf("verification.max_parallel must be >= 0")
	}
	if c.Evidence.Directory == "" {
		return fmt.Errorf("evidence.directory is required")
	}
	if c.Repair.MaxAttempts < 1 {
		return fmt.Errorf("repair.max_attempts must be >= 1")
	}
	return nil
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

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
)

func writeConfig(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `version: 1
project: {name: primary}
requirements:
  paths: [".intentci/requirements/**/*.md"]
`
	if err := os.WriteFile(filepath.Join(root, ".intentci", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExplicitEnvironmentOverridesEveryConfigSection(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root)
	overrides := map[string]string{
		"INTENTCI_VERSION":                                 "1",
		"INTENTCI_PROJECT_NAME":                            "environment",
		"INTENTCI_REQUIREMENTS_PATHS":                      `["contracts/*.md"]`,
		"INTENTCI_VERIFICATION_DEFAULT_TIMEOUT":            "3.5s",
		"INTENTCI_VERIFICATION_MAX_PARALLEL":               "9",
		"INTENTCI_VERIFICATION_FAIL_FAST":                  "true",
		"INTENTCI_VERIFICATION_WORKING_DIRECTORY":          "src",
		"INTENTCI_VERIFICATION_REQUIRE_CLEAN_WORKTREE":     "true",
		"INTENTCI_CHANGE_IMPACT_BASE_REF":                  "main",
		"INTENTCI_CHANGE_IMPACT_INCLUDE_UNTRACKED":         "false",
		"INTENTCI_CHANGE_IMPACT_RUN_UNMAPPED_REQUIREMENTS": "true",
		"INTENTCI_CHANGE_IMPACT_FAIL_ON_UNMAPPED":          "true",
		"INTENTCI_CHANGE_IMPACT_GLOBAL_PATHS":              `["global/**"]`,
		"INTENTCI_EVIDENCE_DIRECTORY":                      "evidence",
		"INTENTCI_EVIDENCE_RETAIN_STDOUT":                  "false",
		"INTENTCI_EVIDENCE_RETAIN_STDERR":                  "false",
		"INTENTCI_EVIDENCE_HASH_ALGORITHM":                 "sha256",
		"INTENTCI_EVIDENCE_REDACT_ENVIRONMENT":             `["*PRIVATE*"]`,
		"INTENTCI_REPAIR_MAX_ATTEMPTS":                     "4",
		"INTENTCI_REPAIR_STOP_ON_REPEATED_DIFF":            "false",
		"INTENTCI_REPAIR_STOP_ON_REPEATED_FAILURE":         "false",
		"INTENTCI_REPAIR_ALLOW_REQUIREMENT_CHANGES":        "true",
		"INTENTCI_REPAIR_ALLOW_TEST_CHANGES":               "false",
		"INTENTCI_REPAIR_PROTECTED_PATHS":                  `["protected/**"]`,
		"INTENTCI_CI_FAIL_ON":                              `["fail","error"]`,
		"INTENTCI_TELEMETRY_ENABLED":                       "true",
	}
	for name, value := range overrides {
		t.Setenv(name, value)
	}
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Name != "environment" || cfg.Verification.MaxParallel != 9 ||
		!cfg.Verification.FailFast || cfg.ChangeImpact.IncludeUntracked ||
		!cfg.ChangeImpact.FailOnUnmapped || cfg.Evidence.RetainStdout ||
		cfg.Repair.MaxAttempts != 4 || !cfg.Repair.AllowRequirementChanges ||
		len(cfg.CI.FailOn) != 2 || !cfg.Telemetry.Enabled {
		t.Fatalf("%+v", cfg)
	}
}

func TestEnvironmentAndValidationFailures(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		value string
	}{
		{"INTENTCI_VERSION", "bad"},
		{"INTENTCI_VERIFICATION_MAX_PARALLEL", "bad"},
		{"INTENTCI_VERIFICATION_FAIL_FAST", "bad"},
		{"INTENTCI_VERIFICATION_REQUIRE_CLEAN_WORKTREE", "bad"},
		{"INTENTCI_CHANGE_IMPACT_INCLUDE_UNTRACKED", "bad"},
		{"INTENTCI_CHANGE_IMPACT_RUN_UNMAPPED_REQUIREMENTS", "bad"},
		{"INTENTCI_CHANGE_IMPACT_FAIL_ON_UNMAPPED", "bad"},
		{"INTENTCI_CHANGE_IMPACT_GLOBAL_PATHS", "bad"},
		{"INTENTCI_EVIDENCE_RETAIN_STDOUT", "bad"},
		{"INTENTCI_EVIDENCE_RETAIN_STDERR", "bad"},
		{"INTENTCI_EVIDENCE_REDACT_ENVIRONMENT", "bad"},
		{"INTENTCI_REPAIR_MAX_ATTEMPTS", "bad"},
		{"INTENTCI_REPAIR_STOP_ON_REPEATED_DIFF", "bad"},
		{"INTENTCI_REPAIR_PROTECTED_PATHS", "bad"},
		{"INTENTCI_CI_FAIL_ON", "bad"},
		{"INTENTCI_REQUIREMENTS_PATHS", "bad"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root)
			t.Setenv(testCase.name, testCase.value)
			if _, err := config.Load(root); err == nil {
				t.Fatal("invalid environment accepted")
			}
		})
	}
	mutations := []func(*config.Config){
		func(cfg *config.Config) { cfg.Requirements.Paths = []string{"../outside"} },
		func(cfg *config.Config) { cfg.ChangeImpact.GlobalPaths = []string{"["} },
		func(cfg *config.Config) { cfg.Evidence.Redact.Environment = []string{"["} },
		func(cfg *config.Config) { cfg.Verification.WorkingDirectory = "/absolute" },
		func(cfg *config.Config) { cfg.Evidence.Directory = "../outside" },
		func(cfg *config.Config) { cfg.Evidence.HashAlgorithm = "md5" },
		func(cfg *config.Config) { cfg.CI.FailOn = []string{"invalid"} },
	}
	for index, mutate := range mutations {
		cfg := config.Default()
		mutate(cfg)
		if err := cfg.Validate(); err == nil {
			t.Fatalf("mutation %d accepted: %+v", index, cfg)
		}
	}
}

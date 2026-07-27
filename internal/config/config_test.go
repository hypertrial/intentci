package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validConfig() Config {
	return Config{Version: 2, Checks: []Check{{
		ID: "go-tests", Intent: "Go tests pass.", Paths: []string{"**/*.go"}, Run: "go test ./...",
	}}}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{"version", func(c *Config) { c.Version = 1 }, "version must be 2"},
		{"empty checks", func(c *Config) { c.Checks = nil }, "checks must not be empty"},
		{"invalid id", func(c *Config) { c.Checks[0].ID = "Go Test" }, "must match"},
		{"duplicate id", func(c *Config) { c.Checks = append(c.Checks, c.Checks[0]) }, "duplicate"},
		{"empty intent", func(c *Config) { c.Checks[0].Intent = " " }, "intent"},
		{"empty run", func(c *Config) { c.Checks[0].Run = "" }, "run"},
		{"empty paths", func(c *Config) { c.Checks[0].Paths = nil }, "paths"},
		{"absolute path", func(c *Config) { c.Checks[0].Paths = []string{"/tmp/**"} }, "invalid"},
		{"traversal", func(c *Config) { c.Checks[0].Paths = []string{"../**"} }, "invalid"},
		{"backslash", func(c *Config) { c.Checks[0].Paths = []string{`src\**`} }, "invalid"},
		{"unclean path", func(c *Config) { c.Checks[0].Paths = []string{"src//**"} }, "invalid"},
		{"invalid glob", func(c *Config) { c.Checks[0].Paths = []string{"["} }, "invalid glob"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			test.change(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}
	if err := validConfig().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadStrictYAML(t *testing.T) {
	root := t.TempDir()
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, FileName), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`version: 2
checks:
  - id: tests
    intent: Tests pass.
    paths: ["**"]
    run: |
      echo first
      echo second
`)
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cfg.Checks[0].Run, "echo second") {
		t.Fatalf("multiline command lost: %q", cfg.Checks[0].Run)
	}

	write("version: 2\nchecks: []\nunknown: true\n")
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "field unknown") {
		t.Fatalf("unknown field error = %v", err)
	}
	write("version: 2\nchecks: []\n---\nversion: 2\n")
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "multiple YAML") {
		t.Fatalf("multiple document error = %v", err)
	}
}

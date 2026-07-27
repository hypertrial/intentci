package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

const FileName = ".intentci.yaml"

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type Config struct {
	Version int     `yaml:"version"`
	Checks  []Check `yaml:"checks"`
}

type Check struct {
	ID     string   `yaml:"id"`
	Intent string   `yaml:"intent"`
	Paths  []string `yaml:"paths"`
	Run    string   `yaml:"run"`
}

func Load(root string) (*Config, error) {
	file, err := os.Open(path.Join(root, FileName))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", FileName, err)
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", FileName, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple YAML documents are not allowed")
		}
		return nil, fmt.Errorf("parse %s: %w", FileName, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	return &cfg, nil
}

func (c Config) Validate() error {
	if c.Version != 2 {
		return fmt.Errorf("version must be 2")
	}
	if len(c.Checks) == 0 {
		return fmt.Errorf("checks must not be empty")
	}
	ids := make(map[string]bool, len(c.Checks))
	for index, check := range c.Checks {
		label := fmt.Sprintf("checks[%d]", index)
		if !idPattern.MatchString(check.ID) {
			return fmt.Errorf("%s.id %q must match %s", label, check.ID, idPattern)
		}
		if ids[check.ID] {
			return fmt.Errorf("duplicate check id %q", check.ID)
		}
		ids[check.ID] = true
		if strings.TrimSpace(check.Intent) == "" {
			return fmt.Errorf("%s.intent must not be empty", label)
		}
		if strings.TrimSpace(check.Run) == "" {
			return fmt.Errorf("%s.run must not be empty", label)
		}
		if len(check.Paths) == 0 {
			return fmt.Errorf("%s.paths must not be empty", label)
		}
		for _, pattern := range check.Paths {
			if err := validatePath(pattern); err != nil {
				return fmt.Errorf("%s.paths: %w", label, err)
			}
		}
	}
	return nil
}

func validatePath(pattern string) error {
	if pattern == "" || strings.Contains(pattern, `\`) || strings.HasPrefix(pattern, "/") {
		return fmt.Errorf("invalid repository-relative pattern %q", pattern)
	}
	clean := path.Clean(pattern)
	if clean != pattern || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("invalid repository-relative pattern %q", pattern)
	}
	for _, part := range strings.Split(pattern, "/") {
		if part == ".." {
			return fmt.Errorf("invalid repository-relative pattern %q", pattern)
		}
	}
	if !doublestar.ValidatePattern(pattern) {
		return fmt.Errorf("invalid glob %q", pattern)
	}
	return nil
}

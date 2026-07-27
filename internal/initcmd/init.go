package initcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypertrial/intentci/internal/config"
)

var writeFile = os.WriteFile
var mkdirAll = os.MkdirAll

// Options configures initialization.
type Options struct {
	Root      string
	Force     bool
	Language  string
	CIGithub  bool
	NoExample bool
}

// Run initializes .intentci in a repository.
func Run(opt Options) error {
	dir := config.Dir(opt.Root)
	cfgPath := config.Path(opt.Root)
	if _, err := os.Stat(cfgPath); err == nil && !opt.Force {
		return fmt.Errorf("%s already exists (use --force)", cfgPath)
	}
	if err := mkdirAll(filepath.Join(dir, "requirements"), 0o755); err != nil {
		return err
	}
	name := filepath.Base(opt.Root)
	if name == "" || name == "." {
		name = "project"
	}
	cfg := fmt.Sprintf(`version: 1

project:
  name: %s

requirements:
  paths:
    - .intentci/requirements/**/*.md

verification:
  default_timeout: 10m
  max_parallel: 4
  fail_fast: false
  working_directory: .
  require_clean_worktree: false

change_impact:
  base_ref: origin/main
  include_untracked: true
  run_unmapped_requirements: false
  fail_on_unmapped: false
  global_paths:
    - .intentci/config.yaml
    - .intentci/providers/**
    - .intentci/schemas/**
    - go.mod
    - go.sum
    - package.json
    - package-lock.json
    - pyproject.toml
    - uv.lock
    - Cargo.toml
    - Cargo.lock
    - pom.xml

evidence:
  directory: .intentci/runs
  retain_stdout: true
  retain_stderr: true
  hash_algorithm: sha256
  redact:
    environment:
      - "*TOKEN*"
      - "*SECRET*"
      - "*PASSWORD*"
      - "*KEY*"

repair:
  max_attempts: 3
  stop_on_repeated_diff: true
  stop_on_repeated_failure: true
  allow_requirement_changes: false
  allow_test_changes: true
  protected_paths: []

ci:
  fail_on:
    - fail
    - error
    - unproven
    - uncertain
    - review_required

telemetry:
  enabled: false
`, name)
	if err := writeFile(cfgPath, []byte(cfg), 0o644); err != nil {
		return err
	}

	gitignore := filepath.Join(dir, ".gitignore")
	_ = writeFile(gitignore, []byte("runs/\ncache/\ntmp/\nconfig.local.yaml\n"), 0o644)

	if !opt.NoExample {
		req := exampleRequirement(opt.Language)
		if err := writeFile(filepath.Join(dir, "requirements", "REQ-001.md"), []byte(req), 0o644); err != nil {
			return err
		}
	}

	if opt.CIGithub {
		wfDir := filepath.Join(opt.Root, ".github", "workflows")
		if err := mkdirAll(wfDir, 0o755); err != nil {
			return err
		}
		wf := `name: intentci
on: [push, pull_request]
jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.23.x"
      - name: Install IntentCI
        run: go install github.com/hypertrial/intentci/cmd/intentci@latest
      - name: Verify
        run: intentci verify --changed --format json
`
		if err := writeFile(filepath.Join(wfDir, "intentci.yml"), []byte(wf), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func exampleRequirement(language string) string {
	cmd := `"printf 'intentci-ok\n'"`
	switch language {
	case "go":
		cmd = `"(go test ./...) && printf 'intentci-ok\n'"`
	case "python":
		cmd = `"(pytest -q) && printf 'intentci-ok\n'"`
	case "typescript", "ts":
		cmd = `"(npm test) && printf 'intentci-ok\n'"`
	case "rust":
		cmd = `"(cargo test) && printf 'intentci-ok\n'"`
	}
	return fmt.Sprintf(`---
id: REQ-001
title: Example requirement
status: active
priority: required
owners:
  - repository-maintainers
depends_on: []
applies_to:
  paths:
    - "**"
tags:
  - example
---

# Intent

The repository smoke checks should pass.

# Rationale

Provides a starting obligation mapped to an existing test command.

# Constraints

## Must

- id: CON-001
  statement: Prefer existing repository test tooling.

## Must Not

- id: CON-002
  statement: Do not invent a parallel test framework.

# Obligations

`+"```yaml"+`
- id: OBL-001
  statement: Smoke checks pass.
  required: true
  verify:
    all:
      - provider: command
        id: smoke
        run: %s
        result:
          type: exit_code
          equals: 0
          stdout:
            contains: intentci-ok
`+"```"+`
`, cmd)
}

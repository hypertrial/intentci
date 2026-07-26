package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
)

func TestLoadParseAndLocalErrors(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, config.DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(":\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(root); err == nil {
		t.Fatal("parse error")
	}

	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(`version: 1
project: {name: demo}
requirements:
  paths: [".intentci/requirements/**/*.md"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.LocalFileName), []byte(":\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(root); err == nil {
		t.Fatal("local parse error")
	}
}

func TestValidateRemainingAndHelpers(t *testing.T) {
	cfg := config.Default()
	cfg.Requirements.Paths = nil
	if err := cfg.Validate(); err == nil {
		t.Fatal("paths")
	}
	cfg = config.Default()
	cfg.Verification.MaxParallel = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("parallel")
	}
	cfg = config.Default()
	cfg.Evidence.Directory = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("evidence")
	}
	cfg = config.Default()
	cfg.Repair.MaxAttempts = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("repair")
	}

	cfg = config.Default()
	cfg.Verification.MaxParallel = 0
	if cfg.MaxParallelOr(7) != 7 {
		t.Fatal(cfg.MaxParallelOr(7))
	}
	if cfg.MaxParallelOr(0) != 4 {
		t.Fatal(cfg.MaxParallelOr(0))
	}
	cfg.ChangeImpact.BaseRef = ""
	if cfg.BaseRefOr("fallback") != "fallback" {
		t.Fatal(cfg.BaseRefOr("fallback"))
	}
	if config.Dir("r") == "" || config.Path("r") == "" {
		t.Fatal("paths")
	}
}

func TestLoadLocalReadError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, config.DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(`version: 1
project: {name: demo}
requirements:
  paths: [".intentci/requirements/**/*.md"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, config.LocalFileName), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(root); err == nil {
		t.Fatal("expected local read error")
	}
}

func TestLoadValidateAfterParse(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, config.DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(`version: 2
project: {name: demo}
requirements:
  paths: [".intentci/requirements/**/*.md"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(root); err == nil {
		t.Fatal("validate")
	}
}

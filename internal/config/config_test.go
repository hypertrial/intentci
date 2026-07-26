package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
)

func TestDefaultAndValidate(t *testing.T) {
	cfg := config.Default()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Telemetry.Enabled {
		t.Fatal("telemetry must default false")
	}
	if cfg.MaxParallelOr(0) != 4 {
		t.Fatalf("parallel=%d", cfg.MaxParallelOr(0))
	}
	if cfg.BaseRefOr("x") != "origin/main" {
		t.Fatalf("base=%s", cfg.BaseRefOr("x"))
	}
}

func TestLoadAndLocalOverride(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, config.DirName)
	if err := os.MkdirAll(filepath.Join(dir, "requirements"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `version: 1
project: {name: demo}
requirements:
  paths: [".intentci/requirements/**/*.md"]
verification:
  default_timeout: 1m
  max_parallel: 2
`
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	local := `verification:
  max_parallel: 8
`
	if err := os.WriteFile(filepath.Join(dir, config.LocalFileName), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Name != "demo" || cfg.Verification.MaxParallel != 8 {
		t.Fatalf("%+v", cfg)
	}
}

func TestValidateErrors(t *testing.T) {
	cfg := config.Default()
	cfg.Version = 2
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected version error")
	}
	cfg = config.Default()
	cfg.Project.Name = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected name error")
	}
	cfg = config.Default()
	cfg.Verification.DefaultTimeout = "nope"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestParseDuration(t *testing.T) {
	d, err := config.ParseDuration("")
	if err != nil || d.Minutes() != 10 {
		t.Fatalf("%v %v", d, err)
	}
	if _, err := config.ParseDuration("0s"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := config.Load(t.TempDir()); err == nil {
		t.Fatal("expected error")
	}
}

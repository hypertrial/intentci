package verify

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/cache"
	"github.com/hypertrial/intentci/internal/trust"
)

func TestRun_TrustDenied(t *testing.T) {
	dir := setupRepo(t)
	body := `version: 1
product: {name: x, purpose: y}
policy: {default_base: HEAD}
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    applies_to: {include: ["**"]}
    verification: {checks: [go-test]}
checks:
  - id: go-test
    command: "true"
    profiles: [full]
    inputs: ["**"]
    timeout: 1m
`
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644)
	cfg := t.TempDir()
	oldCfg := trust.SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer trust.SetUserConfigDir(oldCfg)
	var errb bytes.Buffer
	if _, err := Run(context.Background(), Options{
		Root: dir, Base: "HEAD", Trust: false, Stderr: &errb,
	}); err == nil {
		t.Fatal("trust denied")
	}
}

func TestRun_LoadContractError(t *testing.T) {
	dir := setupRepo(t)
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(":\n"), 0o644)
	if _, err := Run(context.Background(), Options{Root: dir, Base: "HEAD", Trust: true}); err == nil {
		t.Fatal("bad contract")
	}
}

func TestRun_CacheOpenErr(t *testing.T) {
	old := openCache
	defer func() { openCache = old }()
	openCache = func(string) (*cache.Store, error) { return nil, errors.New("cache") }
	dir := setupRepo(t)
	body := `version: 1
product: {name: x, purpose: y}
policy: {default_base: HEAD}
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    applies_to: {include: ["**"]}
    verification: {checks: [go-test]}
checks:
  - id: go-test
    command: "true"
    profiles: [full]
    inputs: ["**"]
    timeout: 1m
`
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644)
	if _, err := Run(context.Background(), Options{Root: dir, Base: "HEAD", Trust: true}); err == nil {
		t.Fatal("cache")
	}
}

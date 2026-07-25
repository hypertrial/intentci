package contract_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestLoadHashHelpers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contract.yaml")
	body := `version: 1
product: {name: x, purpose: y}
requirements:
  - id: A-001
    type: behavior
    title: t
    statement: s
    status: approved
    severity: blocking
    verification: {checks: [c]}
checks:
  - id: c
    command: "true"
    timeout: 1m
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, raw, err := contract.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if contract.Hash(raw) == "" {
		t.Fatal("hash")
	}
	if contract.Path(dir) == "" {
		t.Fatal("path")
	}
	if _, _, err := contract.LoadFromRoot(dir); err == nil {
		// no .intentci
		_ = err
	}
	if err := os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contract.Path(dir), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := contract.LoadFromRoot(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := contract.ParseTimeout(""); err != nil {
		t.Fatal(err)
	}
	if _, err := contract.ParseTimeout("bad"); err == nil {
		t.Fatal("bad timeout")
	}
	p := contract.Policy{}
	if !p.BlocksOnUnknown() || !p.BlocksOnUnverified() {
		t.Fatal("defaults")
	}
	f := false
	p.UnknownBlocks = &f
	p.UnverifiedBlocks = &f
	if p.BlocksOnUnknown() || p.BlocksOnUnverified() {
		t.Fatal("false overrides")
	}
	if p.DefaultBaseOr("x") != "x" {
		t.Fatal("base")
	}
	p.DefaultBase = "main"
	if p.DefaultBaseOr("x") != "main" {
		t.Fatal("base set")
	}
	e := contract.Execution{}
	if e.MaxParallelOr(3) != 3 {
		t.Fatal("parallel")
	}
	e.MaxParallel = 2
	if e.MaxParallelOr(3) != 2 {
		t.Fatal("parallel set")
	}
	ch := contract.Check{}
	if !ch.HasProfile("fast") {
		t.Fatal("empty profiles")
	}
	ch.Profiles = []string{"full"}
	if ch.HasProfile("fast") || !ch.HasProfile("full") {
		t.Fatal("profiles")
	}
	v := contract.Verification{}
	if v.VerificationMode() != "all" {
		t.Fatal(v.VerificationMode())
	}
	v.Mode = "any"
	if v.VerificationMode() != "any" {
		t.Fatal(v.Mode)
	}
	if _, ok := c.CheckByID("missing"); ok {
		t.Fatal("missing check")
	}
	if _, ok := c.CheckByID("c"); !ok {
		t.Fatal("check")
	}
	_ = c.CheckMap()
	if _, _, err := contract.Load(filepath.Join(dir, "nope.yaml")); err == nil {
		t.Fatal("missing file")
	}
	bad := filepath.Join(dir, "bad.yaml")
	_ = os.WriteFile(bad, []byte(":\n:\n"), 0o644)
	if _, _, err := contract.Load(bad); err == nil {
		t.Fatal("bad yaml")
	}
}

func TestValidate_MoreBranches(t *testing.T) {
	c := validContract()
	c.Checks[0].Timeout = "nope"
	if err := contract.Validate(c); err == nil {
		t.Fatal("bad timeout")
	}
	c = validContract()
	c.Checks = append(c.Checks, c.Checks[0])
	if err := contract.Validate(c); err == nil {
		t.Fatal("dup check")
	}
	c = validContract()
	c.Checks[0].DependsOn = []string{"missing"}
	if err := contract.Validate(c); err == nil {
		t.Fatal("dep missing")
	}
	c = validContract()
	c.Checks[0].Inputs = []string{""}
	if err := contract.Validate(c); err == nil {
		t.Fatal("empty glob")
	}
	c = validContract()
	c.Execution.MaxParallel = 0
	c.Environment.Include = nil
	truev := true
	c.Policy.Semantic.Enabled = false
	c.Requirements[0].Sources = []contract.Source{}
	c.Requirements[0].AppliesTo = contract.AppliesTo{}
	c.Checks[0].Exclusive = false
	c.Checks[0].Results = &contract.Results{}
	if err := contract.Validate(c); err != nil {
		t.Fatal(err)
	}
	_ = truev
}

package verify

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/changespec"
)

func TestRun_ChangeValidateAndFindings(t *testing.T) {
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
	changespec.Create(dir, "DEMO-1")
	change := `version: 1
id: DEMO-1
status: approved
type: feature
summary: s
goals: [g]
acceptance:
  - id: AC-001
    statement: s
    severity: blocking
    verification: {checks: [missing]}
`
	os.WriteFile(filepath.Join(dir, ".intentci", "changes", "DEMO-1.yaml"), []byte(change), 0o644)
	var errb bytes.Buffer
	if _, err := Run(context.Background(), Options{
		Root: dir, Base: "HEAD", ChangeID: "DEMO-1", Trust: true, Stderr: &errb,
	}); err == nil {
		t.Fatal("invalid change")
	}
}

func TestRun_InvalidContract(t *testing.T) {
	dir := setupRepo(t)
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte("version: 1\n"), 0o644)
	if _, err := Run(context.Background(), Options{Root: dir, Base: "HEAD", Trust: true}); err == nil {
		t.Fatal("invalid contract")
	}
}

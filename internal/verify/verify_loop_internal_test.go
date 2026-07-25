package verify

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/changespec"
)

func TestRun_CheckLoopAndFindings(t *testing.T) {
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
	changespec.Create(dir, "C-1")
	change := `version: 1
id: C-1
status: approved
type: feature
summary: s
goals: [g]
acceptance:
  - id: AC-001
    statement: s
    severity: blocking
    verification: {checks: [go-test]}
`
	os.WriteFile(filepath.Join(dir, ".intentci", "changes", "C-1.yaml"), []byte(change), 0o644)
	o, err := Run(context.Background(), Options{
		Root: dir, Base: "HEAD", ChangeID: "C-1", Trust: true, Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Result.ChangeFindings) == 0 {
		t.Fatal("findings")
	}
	if len(o.Result.Checks) == 0 {
		t.Fatal("checks")
	}
	_ = filepath.Base
}

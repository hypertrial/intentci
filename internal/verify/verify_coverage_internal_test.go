package verify

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/internal/scheduler"
)

func TestRun_DefaultBaseFromPolicy(t *testing.T) {
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
    severity: advisory
    applies_to: {include: ["nope/**"]}
    verification: {checks: [go-test]}
checks:
  - id: go-test
    command: "true"
    profiles: [full]
    timeout: 1m
`
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644)
	if _, err := Run(context.Background(), Options{Root: dir, Trust: true}); err != nil {
		t.Fatal(err)
	}
}

func TestRun_GitResolveError(t *testing.T) {
	dir := setupRepo(t)
	body := `version: 1
product: {name: x, purpose: y}
policy: {default_base: origin/does-not-exist}
requirements: []
checks: []
`
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644)
	if _, err := Run(context.Background(), Options{Root: dir, Trust: true}); err == nil {
		t.Fatal("git resolve")
	}
}

func TestRun_MissingCheckResult(t *testing.T) {
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
	old := scheduleChecks
	defer func() { scheduleChecks = old }()
	scheduleChecks = func(ctx context.Context, checks map[string]contract.Check, ids []string, opt scheduler.Options) map[string]runner.Result {
		return map[string]runner.Result{}
	}
	o, err := Run(context.Background(), Options{Root: dir, Base: "HEAD", Trust: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Result.Checks) != 0 {
		t.Fatalf("%#v", o.Result.Checks)
	}
}

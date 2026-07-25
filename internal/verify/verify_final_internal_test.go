package verify

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_ProfileDefault(t *testing.T) {
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
	o, err := Run(context.Background(), Options{Root: dir, Base: "HEAD", Trust: true, Profile: ""})
	if err != nil {
		t.Fatal(err)
	}
	if o.Result.Profile != "full" {
		t.Fatalf("profile=%q", o.Result.Profile)
	}
}

package verify

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestRun_DefaultProfileAndExitCode(t *testing.T) {
	dir := setupRepo(t)
	body := `version: 1
product: {name: x, purpose: y}
policy: {default_base: HEAD, unknown_blocks: false, unverified_blocks: false}
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
    command: "false"
    profiles: [full]
    inputs: ["**"]
    timeout: 1m
`
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644)
	var errb bytes.Buffer
	o, err := Run(context.Background(), Options{
		Root: dir, Base: "HEAD", Trust: true, Stderr: &errb,
	})
	if err != nil {
		t.Fatal(err)
	}
	if o.ExitCode != 10 {
		t.Fatalf("exit=%d", o.ExitCode)
	}
	_ = protocol.StatusFail
}

package verify_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/verify"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestRun_Waiver(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run("git", "-c", "core.hooksPath=/dev/null", "init")
	run("git", "checkout", "-b", "main")
	if err := os.MkdirAll(filepath.Join(dir, ".intentci", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	contractBody := `version: 1
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
    command: "false"
    timeout: 1m
`
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(contractBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exp := time.Now().UTC().Add(48 * time.Hour).Format("2006-01-02")
	change := `version: 1
id: WTEST-1
status: approved
type: bugfix
summary: waive
goals: [g]
acceptance:
  - id: AC-001
    statement: s
    severity: advisory
    verification: {checks: [go-test]}
affected_requirements: [BUILD-001]
waivers:
  - id: W-001
    requirement: BUILD-001
    reason: temporary
    owner: alice
    expires: ` + exp + `
`
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "changes", "WTEST-1.yaml"), []byte(change), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "init")

	var stdout, stderr bytes.Buffer
	out, err := verify.Run(context.Background(), verify.Options{
		Root: dir, Base: "HEAD", Profile: "full", Trust: true, NoCache: true,
		ChangeID: "WTEST-1", Stdout: &stdout, Stderr: &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Result.Status != protocol.StatusPass || out.ExitCode != 0 {
		t.Fatalf("status=%s code=%d reqs=%+v", out.Result.Status, out.ExitCode, out.Result.Requirements)
	}
	if out.Result.Summary.Waived < 1 || len(out.Result.Waivers) < 1 {
		t.Fatalf("%+v", out.Result)
	}
}

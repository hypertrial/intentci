package verify_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/verify"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestRun_AttestAndContractGate(t *testing.T) {
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
    command: "true"
    timeout: 1m
    inputs: ["**"]
`
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(contractBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "init")

	// Weaken on top of committed base.
	weak := `version: 1
product: {name: x, purpose: y}
policy: {default_base: HEAD}
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: draft
    severity: advisory
    verification: {checks: [go-test]}
checks:
  - id: go-test
    command: "true"
    timeout: 1m
    inputs: ["**"]
`
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(weak), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	out, err := verify.Run(context.Background(), verify.Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: true, NoCache: true,
		Stdout: &stdout, Stderr: &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Result.Status != protocol.StatusUnverified || out.ExitCode != 11 {
		t.Fatalf("gate: status=%s code=%d changes=%+v", out.Result.Status, out.ExitCode, out.Result.ContractChanges)
	}

	// Approved type:contract Change Spec allows pass despite weakenings.
	change := `version: 1
id: CONTRACT-1
status: approved
type: contract
summary: weaken
goals: [g]
acceptance:
  - id: AC-001
    statement: s
    severity: advisory
    verification: {checks: [go-test]}
`
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "changes", "CONTRACT-1.yaml"), []byte(change), 0o644); err != nil {
		t.Fatal(err)
	}
	// Restore approved requirement for checks to pass under effective policy path with type:contract.
	// Keep weakened head; effective restores base approved definition.
	out, err = verify.Run(context.Background(), verify.Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: true, NoCache: true,
		ChangeID: "CONTRACT-1", Attest: true, Stdout: &stdout, Stderr: &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Result.Status != protocol.StatusPass || out.ExitCode != 0 {
		t.Fatalf("type contract: status=%s code=%d", out.Result.Status, out.ExitCode)
	}
	if out.AttestationPath == "" {
		t.Fatal("expected attestation path")
	}
	if _, err := os.Stat(out.AttestationPath); err != nil {
		t.Fatal(err)
	}

	// Attest skipped on non-pass
	out, err = verify.Run(context.Background(), verify.Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: true, NoCache: true,
		Attest: true, Stdout: &stdout, Stderr: &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Result.Status == protocol.StatusPass {
		t.Fatal("expected non-pass without type:contract")
	}
	if out.AttestationPath != "" {
		t.Fatal("should not attest non-pass")
	}
}

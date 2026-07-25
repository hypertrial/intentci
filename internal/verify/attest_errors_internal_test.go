package verify

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/attest"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestRun_LoadBaseContractError(t *testing.T) {
	dir := setupPassRepo(t)
	old := loadBaseContract
	defer func() { loadBaseContract = old }()
	loadBaseContract = func(string, string) (*contract.Contract, []byte, bool, error) {
		return nil, nil, false, errors.New("base boom")
	}
	var stdout, stderr bytes.Buffer
	out, err := Run(context.Background(), Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: true, NoCache: true,
		Stdout: &stdout, Stderr: &stderr,
	})
	if err == nil || out.ExitCode != 20 {
		t.Fatalf("%v %+v", err, out)
	}
}

func TestRun_AttestWriteErrors(t *testing.T) {
	dir := setupPassRepo(t)
	oldWrite := writeAttestation
	oldBuild := buildAttestation
	defer func() {
		writeAttestation = oldWrite
		buildAttestation = oldBuild
	}()
	var stdout, stderr bytes.Buffer

	writeAttestation = func(string, *attest.Attestation) (string, error) {
		return "", errors.New("write fail")
	}
	out, err := Run(context.Background(), Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: true, NoCache: true, Attest: true,
		Stdout: &stdout, Stderr: &stderr,
	})
	if err == nil || out.ExitCode != 30 {
		t.Fatalf("%v %+v", err, out)
	}

	buildAttestation = func(*protocol.Result, map[string]contract.Check, []string) (*attest.Attestation, error) {
		return nil, errors.New("build fail")
	}
	out, err = Run(context.Background(), Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: true, NoCache: true, Attest: true,
		Stdout: &stdout, Stderr: &stderr,
	})
	if err == nil || out.ExitCode != 30 {
		t.Fatalf("build %v %+v", err, out)
	}
}

func setupPassRepo(t *testing.T) string {
	t.Helper()
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
	if err := os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
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
    timeout: 1m
`
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "init")
	_ = protocol.StatusPass
	return dir
}

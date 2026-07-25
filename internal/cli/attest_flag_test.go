package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/cli"
)

func TestVerifyAttestFlag(t *testing.T) {
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
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	var out, errb bytes.Buffer
	code := cli.RunMain([]string{"verify", "--trust", "--all", "--no-cache", "--attest", "--base", "HEAD", "--format", "json"}, &out, &errb)
	if code != 0 {
		t.Fatalf("%d %s %s", code, out.String(), errb.String())
	}
	entries, _ := os.ReadDir(filepath.Join(dir, ".intentci", "tmp"))
	found := false
	for _, e := range entries {
		if len(e.Name()) > 12 && e.Name()[:12] == "attestation-" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no attestation: %v stderr=%s", entries, errb.String())
	}
}

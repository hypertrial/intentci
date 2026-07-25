package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/cli"
)

func TestVerifyShowSemanticInput(t *testing.T) {
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
policy:
  default_base: HEAD
  semantic:
    enabled: true
    enforcement: advisory
    provider:
      type: local
      command: ./missing-provider
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    applies_to: {include: ["**"]}
    verification: {checks: [go-test], semantic: optional}
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
	// Preview must work without --trust and without executing checks/providers.
	code := cli.RunMain([]string{"verify", "--all", "--show-semantic-input", "--base", "HEAD", "--format", "json"}, &out, &errb)
	if code != 0 {
		t.Fatalf("%d %s %s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), `"protocol_version"`) {
		t.Fatalf("expected request json, got %s", out.String())
	}
	if strings.Contains(out.String(), `"status": "fail"`) {
		t.Fatalf("preview should not include check execution results: %s", out.String())
	}
}

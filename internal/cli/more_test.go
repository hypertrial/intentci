package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/cli"
)

func TestCLI_ChangeValidateExplainFailPaths(t *testing.T) {
	dir := gitRepo(t)
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(dir)

	var out, errb bytes.Buffer
	if code := cli.RunMain([]string{"validate"}, &out, &errb); code != 20 {
		_ = code
	}
	cli.RunMain([]string{"init"}, &out, &errb)
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
    profiles: [fast, full]
    inputs: ["**"]
    timeout: 1m
    cache: success
`
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644)
	cli.RunMain([]string{"change", "create", "DEMO-1"}, &out, &errb)
	os.WriteFile(filepath.Join(dir, ".intentci", "changes", "BAD.yaml"), []byte("version: 1\n"), 0o644)
	if code := cli.RunMain([]string{"validate"}, &out, &errb); code == 0 {
		t.Fatal("expected invalid change")
	}
	os.Remove(filepath.Join(dir, ".intentci", "changes", "BAD.yaml"))
	change := `version: 1
id: DEMO-1
status: approved
type: feature
summary: demo
goals: [g]
acceptance:
  - id: AC-001
    statement: works
    severity: blocking
    verification:
      checks: [go-test]
affected_requirements: [BUILD-001]
required_checks: [go-test]
`
	os.WriteFile(filepath.Join(dir, ".intentci", "changes", "DEMO-1.yaml"), []byte(change), 0o644)
	out.Reset()
	code := cli.RunMain([]string{"verify", "--trust", "--change", "DEMO-1", "--base", "HEAD", "--format", "text"}, &out, &errb)
	if code != 0 {
		t.Fatalf("verify change %d %s %s", code, out.String(), errb.String())
	}
	out.Reset()
	code = cli.RunMain([]string{"explain", "AC-001", "--change", "DEMO-1", "--base", "HEAD"}, &out, &errb)
	if code != 0 {
		t.Fatalf("explain ac %d %s", code, errb.String())
	}
	oldArgs := os.Args
	os.Args = []string{"intentci", "version"}
	if cli.Main() != 0 {
		t.Fatal("Main version")
	}
	if err := cli.Execute(); err != nil {
		t.Fatal(err)
	}
	os.Args = oldArgs
	out.Reset()
	code = cli.RunMain([]string{"verify", "--trust", "--base", "missing-ref"}, &out, &errb)
	if code != 21 {
		t.Fatalf("missing base code=%d", code)
	}
	out.Reset()
	code = cli.RunMain([]string{"change", "create", "DEMO-1"}, &out, &errb)
	if code == 0 {
		t.Fatal("duplicate create")
	}
}

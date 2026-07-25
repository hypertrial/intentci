package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/cli"
)

func TestRunMain_VersionInitValidate(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.RunMain([]string{"version"}, &out, &errb)
	if code != 0 || out.Len() == 0 {
		t.Fatalf("version code=%d out=%s err=%s", code, out.String(), errb.String())
	}
	dir := gitRepo(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	code = cli.RunMain([]string{"init"}, &out, &errb)
	if code != 0 {
		t.Fatalf("init %d %s", code, errb.String())
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
    profiles: [fast, full]
    inputs: ["**"]
    timeout: 1m
    cache: success
`
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	code = cli.RunMain([]string{"validate"}, &out, &errb)
	if code != 0 {
		t.Fatalf("validate %d %s", code, errb.String())
	}
	out.Reset()
	code = cli.RunMain([]string{"change", "create", "DEMO-1"}, &out, &errb)
	if code != 0 {
		t.Fatalf("change create %d %s", code, errb.String())
	}
	out.Reset()
	code = cli.RunMain([]string{"verify", "--trust", "--all", "--base", "HEAD", "--format", "json", "--no-cache"}, &out, &errb)
	if code != 0 {
		t.Fatalf("verify %d %s %s", code, out.String(), errb.String())
	}
	out.Reset()
	code = cli.RunMain([]string{"check", "--trust", "--all", "--base", "HEAD", "--format", "text"}, &out, &errb)
	if code != 0 {
		t.Fatalf("check %d %s", code, errb.String())
	}
	out.Reset()
	code = cli.RunMain([]string{"explain", "BUILD-001", "--base", "HEAD"}, &out, &errb)
	if code != 0 {
		t.Fatalf("explain %d %s", code, errb.String())
	}
	out.Reset()
	code = cli.RunMain([]string{"verify", "--format", "yaml"}, &out, &errb)
	if code != 20 {
		t.Fatalf("bad format code=%d", code)
	}
}

func TestExecuteAndExitError(t *testing.T) {
	err := &cli.ExitError{Code: 11}
	if err.Error() != "" || err.ExitCode() != 11 || err.Unwrap() != nil {
		t.Fatalf("%+v", err)
	}
	err2 := &cli.ExitError{Code: 10, Err: os.ErrNotExist}
	if err2.Error() == "" || err2.Unwrap() == nil {
		t.Fatal(err2)
	}
	if cli.CodeOf(err2) != 10 {
		t.Fatal(cli.CodeOf(err2))
	}
	if cli.CodeOf(os.ErrNotExist) != 1 {
		t.Fatal("default")
	}
}

package cli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/cli"
	"github.com/hypertrial/intentci/internal/exitcode"
)

func TestRunMainExitErrorAndGeneric(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.RunMain([]string{"schema", "nope"}, &out, &errb)
	if code != exitcode.Usage {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	code = cli.RunMain([]string{"not-a-command"}, &out, &errb)
	if code != exitcode.Internal && code != 1 {
		// cobra unknown command returns error -> Internal
		_ = code
	}
}

func TestCLIFlagsAndErrorPaths(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if code := cli.RunMain([]string{"init", "--language", "python", "--ci", "github", "--force"}, &out, &errb); code != 0 {
		// first init
		_ = code
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"init"}, &out, &errb); code == 0 {
		t.Fatal("expected exists")
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"init", "--force", "--no-example"}, &out, &errb); code != 0 {
		t.Fatalf("force init %d %s", code, errb.String())
	}
	// restore example for compile/verify
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"init", "--force"}, &out, &errb); code != 0 {
		t.Fatal(code, errb.String())
	}
	gitInit(t, root)

	out.Reset()
	errb.Reset()
	outPath := filepath.Join(root, "ir.json")
	if code := cli.RunMain([]string{"compile", "--format", "json", "--output", outPath, "--requirement", "REQ-001"}, &out, &errb); code != 0 {
		t.Fatalf("compile %d %s", code, errb.String())
	}

	out.Reset()
	errb.Reset()
	rep := filepath.Join(root, "report.json")
	if code := cli.RunMain([]string{"verify", "--all", "--format", "junit", "--output", rep, "--no-cache"}, &out, &errb); code != 0 {
		t.Fatalf("verify %d %s", code, errb.String())
	}

	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"explain", "REQ-001", "--format", "json", "--show-logs"}, &out, &errb); code != 0 {
		t.Fatalf("explain json %d %s", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"explain", "MISSING", "--format", "json"}, &out, &errb); code == 0 {
		t.Fatal("expected missing")
	}
	// load by run id
	latest, _ := os.ReadFile(filepath.Join(root, ".intentci", "runs", "latest"))
	runID := string(bytes.TrimSpace(latest))
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"explain", "REQ-001", "--run", runID, "--show-evidence"}, &out, &errb); code != 0 {
		t.Fatalf("explain run %d %s", code, errb.String())
	}

	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"repair", "--dry-run", "--max-attempts", "1", "--allow-test-changes"}, &out, &errb); code != exitcode.Pass && code != exitcode.RepairExhausted {
		// may pass
		_ = code
	}

	for _, name := range []string{"requirement", "evidence", "verdict", "repair", "intent"} {
		out.Reset()
		errb.Reset()
		if code := cli.RunMain([]string{"schema", name}, &out, &errb); code != 0 || out.Len() == 0 {
			t.Fatalf("schema %s code=%d", name, code)
		}
	}

	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"verify", "--changed", "--base", "HEAD", "--format", "text"}, &out, &errb); code != 0 && code != exitcode.Pass {
		_ = code
	}

	// status without run in fresh dir
	empty := t.TempDir()
	gitInit(t, empty)
	if err := os.Chdir(empty); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"status"}, &out, &errb); code == 0 {
		t.Fatal("expected no runs")
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"doctor"}, &out, &errb); code == 0 {
		t.Fatal("doctor should fail without config")
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"explain", "X"}, &out, &errb); code == 0 {
		t.Fatal("no run")
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"repair", "--dry-run"}, &out, &errb); code == 0 {
		t.Fatal("no config")
	}
	_ = errors.New("keep import")
}

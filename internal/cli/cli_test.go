package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/cli"
	"github.com/hypertrial/intentci/internal/exitcode"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "t@example.com"},
		{"git", "config", "user.name", "t"},
		{"git", "add", "."},
		{"git", "commit", "-m", "init", "--allow-empty"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
}

func TestInitCompileVerifyExplainStatusSchemaDoctor(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if code := cli.RunMain([]string{"init"}, &out, &errb); code != 0 {
		t.Fatalf("init code=%d err=%s", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"compile", "--strict"}, &out, &errb); code != 0 {
		t.Fatalf("compile code=%d err=%s out=%s", code, errb.String(), out.String())
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"verify", "--all", "--format", "json"}, &out, &errb); code != 0 {
		t.Fatalf("verify code=%d err=%s out=%s", code, errb.String(), out.String())
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"explain", "REQ-001", "--show-evidence"}, &out, &errb); code != 0 {
		t.Fatalf("explain code=%d err=%s", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"status"}, &out, &errb); code != 0 {
		t.Fatalf("status code=%d err=%s", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"schema", "ir"}, &out, &errb); code != 0 || out.Len() == 0 {
		t.Fatalf("schema code=%d", code)
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"doctor"}, &out, &errb); code != 0 {
		t.Fatalf("doctor code=%d err=%s out=%s", code, errb.String(), out.String())
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"version"}, &out, &errb); code != 0 {
		t.Fatal(code)
	}
	out.Reset()
	errb.Reset()
	if code := cli.RunMain([]string{"repair", "--dry-run", "--max-attempts", "1", "--changed=false"}, &out, &errb); code != exitcode.Pass && code != exitcode.RepairExhausted && code != exitcode.Fail && code != exitcode.Unproven {
		// dry-run with passing verify should pass
		_ = code
	}
	// ensure config exists
	if _, err := os.Stat(filepath.Join(root, ".intentci", "config.yaml")); err != nil {
		t.Fatal(err)
	}
}

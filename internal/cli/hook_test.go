package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/cli"
)

func TestHookInstallUninstallCLI(t *testing.T) {
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
	if err := os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(`version: 1
product: {name: x, purpose: y}
checks: []
requirements: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	var out, errb bytes.Buffer
	if code := cli.RunMain([]string{"hook", "install"}, &out, &errb); code != 0 {
		t.Fatalf("install %d %s %s", code, out.String(), errb.String())
	}
	if code := cli.RunMain([]string{"hook", "uninstall"}, &out, &errb); code != 0 {
		t.Fatalf("uninstall %d %s %s", code, out.String(), errb.String())
	}
}

package hooks_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/hooks"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "-c", "core.hooksPath=/dev/null", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return dir
}

func TestInstallUninstall(t *testing.T) {
	dir := gitRepo(t)
	path, err := hooks.Install(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# BEGIN INTENTCI") {
		t.Fatal(string(data))
	}
	// Reinstall refreshes block.
	if _, err := hooks.Install(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := hooks.Uninstall(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected removed, err=%v", err)
	}
	// Uninstall missing is ok.
	if _, err := hooks.Uninstall(dir); err != nil {
		t.Fatal(err)
	}
}

func TestInstall_RefuseUnmanaged(t *testing.T) {
	dir := gitRepo(t)
	hook := filepath.Join(dir, ".git", "hooks", "pre-push")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := hooks.Install(dir); err == nil {
		t.Fatal("expected refuse")
	}
}

func TestInstall_ComposeReplace(t *testing.T) {
	dir := gitRepo(t)
	hook := filepath.Join(dir, ".git", "hooks", "pre-push")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/usr/bin/env bash\nset -euo pipefail\necho before\n# BEGIN INTENTCI\nold\n# END INTENTCI\necho after\n"
	if err := os.WriteFile(hook, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := hooks.Install(dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(hook)
	if !strings.Contains(string(data), "echo before") || !strings.Contains(string(data), "echo after") {
		t.Fatal(string(data))
	}
	if strings.Contains(string(data), "old\n") {
		t.Fatal("old block remains")
	}
	if _, err := hooks.Uninstall(dir); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(hook)
	if strings.Contains(string(data), "BEGIN INTENTCI") {
		t.Fatal(string(data))
	}
	if !strings.Contains(string(data), "echo before") {
		t.Fatal(string(data))
	}
}

func TestInstall_NotGit(t *testing.T) {
	if _, err := hooks.Install(t.TempDir()); err == nil {
		t.Fatal("expected error")
	}
}

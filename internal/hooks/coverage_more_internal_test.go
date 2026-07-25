package hooks

import (
	"errors"
	"os"
	"testing"
)

func TestInstall_MalformedMarkers(t *testing.T) {
	oldR := readFile
	oldGit := runGit
	defer func() {
		readFile = oldR
		runGit = oldGit
	}()
	runGit = func(string, ...string) ([]byte, error) { return []byte(".git"), nil }
	readFile = func(string) ([]byte, error) {
		return []byte("#!/bin/sh\n# END INTENTCI\n# BEGIN INTENTCI\n"), nil
	}
	if _, err := Install(t.TempDir()); err == nil {
		t.Fatal("malformed order")
	}
}

func TestUninstall_PrePushPathError(t *testing.T) {
	oldGit := runGit
	defer func() { runGit = oldGit }()
	runGit = func(string, ...string) ([]byte, error) { return nil, errors.New("nogit") }
	if _, err := Uninstall(t.TempDir()); err == nil {
		t.Fatal("expected error")
	}
}

func TestIsOnlyShebang_EmptyLines(t *testing.T) {
	if !isOnlyShebang("#!/bin/sh\n\nset -euo pipefail\n") {
		t.Fatal("empty lines in middle")
	}
	_ = os.DevNull
}

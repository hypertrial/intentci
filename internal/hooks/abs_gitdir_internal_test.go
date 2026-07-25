package hooks

import (
	"path/filepath"
	"testing"
)

func TestResolveGitDir_Absolute(t *testing.T) {
	old := runGit
	defer func() { runGit = old }()
	abs := filepath.Join(t.TempDir(), ".git")
	runGit = func(string, ...string) ([]byte, error) {
		return []byte(abs + "\n"), nil
	}
	got, err := resolveGitDir("/repo")
	if err != nil || got != filepath.Clean(abs) {
		t.Fatalf("%v %s", err, got)
	}
}

func TestUninstall_NoMarkers(t *testing.T) {
	oldR := readFile
	oldGit := runGit
	defer func() {
		readFile = oldR
		runGit = oldGit
	}()
	runGit = func(string, ...string) ([]byte, error) { return []byte(".git"), nil }
	readFile = func(string) ([]byte, error) { return []byte("#!/bin/sh\necho hi\n"), nil }
	if _, err := Uninstall(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

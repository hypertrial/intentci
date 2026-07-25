package hooks

import (
	"errors"
	"os"
	"testing"
)

func TestReplaceRemoveMalformed(t *testing.T) {
	if _, ok := replaceBlock("no markers"); ok {
		t.Fatal("replace")
	}
	if _, ok := removeBlock("BEGIN only # BEGIN INTENTCI"); ok {
		t.Fatal("remove")
	}
	if !isOnlyShebang("#!/usr/bin/env bash\nset -euo pipefail\n\n") {
		t.Fatal("shebang")
	}
	if isOnlyShebang("#!/bin/sh\necho hi\n") {
		t.Fatal("not only shebang")
	}
}

func TestInstall_IOErrors(t *testing.T) {
	oldMk := mkdirAll
	oldW := writeFile
	oldR := readFile
	oldGit := runGit
	defer func() {
		mkdirAll = oldMk
		writeFile = oldW
		readFile = oldR
		runGit = oldGit
	}()
	runGit = func(string, ...string) ([]byte, error) { return []byte(".git"), nil }
	mkdirAll = func(string, os.FileMode) error { return errors.New("mkdir") }
	if _, err := Install(t.TempDir()); err == nil {
		t.Fatal("mkdir")
	}
	mkdirAll = oldMk
	readFile = func(string) ([]byte, error) { return nil, errors.New("read") }
	if _, err := Install(t.TempDir()); err == nil {
		t.Fatal("read")
	}
	readFile = func(string) ([]byte, error) {
		return []byte("#!/bin/sh\n# BEGIN INTENTCI\n# END INTENTCI\n"), nil
	}
	writeFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	if _, err := Install(t.TempDir()); err == nil {
		t.Fatal("write")
	}
	readFile = func(string) ([]byte, error) {
		return []byte("# BEGIN INTENTCI\nbroken"), nil
	}
	writeFile = oldW
	if _, err := Install(t.TempDir()); err == nil {
		t.Fatal("malformed")
	}
}

func TestUninstall_IOErrors(t *testing.T) {
	oldR := readFile
	oldW := writeFile
	oldRm := remove
	oldGit := runGit
	defer func() {
		readFile = oldR
		writeFile = oldW
		remove = oldRm
		runGit = oldGit
	}()
	runGit = func(string, ...string) ([]byte, error) { return []byte(".git"), nil }
	readFile = func(string) ([]byte, error) { return nil, errors.New("read") }
	if _, err := Uninstall(t.TempDir()); err == nil {
		t.Fatal("read")
	}
	readFile = func(string) ([]byte, error) {
		return []byte("#!/bin/sh\necho keep\n# BEGIN INTENTCI\nx\n# END INTENTCI\n"), nil
	}
	writeFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	if _, err := Uninstall(t.TempDir()); err == nil {
		t.Fatal("write")
	}
	readFile = func(string) ([]byte, error) {
		return []byte("#!/usr/bin/env bash\nset -euo pipefail\n# BEGIN INTENTCI\nx\n# END INTENTCI\n"), nil
	}
	writeFile = oldW
	remove = func(string) error { return errors.New("rm") }
	if _, err := Uninstall(t.TempDir()); err == nil {
		t.Fatal("rm")
	}
	readFile = func(string) ([]byte, error) {
		return []byte("# BEGIN INTENTCI\nbroken"), nil
	}
	remove = oldRm
	if _, err := Uninstall(t.TempDir()); err == nil {
		t.Fatal("malformed uninstall")
	}
}

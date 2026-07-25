package cli

import (
	"bytes"
	"errors"
	"testing"
)

func TestHookCmd_GetwdAndErrors(t *testing.T) {
	old := getwd
	defer func() { getwd = old }()
	getwd = func() (string, error) { return "", errors.New("cwd") }
	var out, errb bytes.Buffer
	if code := RunMain([]string{"hook", "install"}, &out, &errb); code == 0 {
		t.Fatal("install getwd")
	}
	if code := RunMain([]string{"hook", "uninstall"}, &out, &errb); code == 0 {
		t.Fatal("uninstall getwd")
	}
}

func TestHookCmd_NoIntentciRoot(t *testing.T) {
	dir := t.TempDir()
	// git repo without .intentci — FindRoot fails, falls back to cwd
	old := getwd
	defer func() { getwd = old }()
	getwd = func() (string, error) { return dir, nil }
	var out, errb bytes.Buffer
	// install will fail not a git repo
	if code := RunMain([]string{"hook", "install"}, &out, &errb); code == 0 {
		t.Fatal("expected install failure")
	}
	if code := RunMain([]string{"hook", "uninstall"}, &out, &errb); code == 0 {
		t.Fatal("expected uninstall failure")
	}
}

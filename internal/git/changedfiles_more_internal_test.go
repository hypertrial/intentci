package git

import (
	"errors"
	"testing"
)

func TestChangedFiles_StagedAndUntrackedErrors(t *testing.T) {
	old := run
	defer func() { run = old }()
	run = func(root string, args ...string) (string, error) {
		if args[0] == "diff" && len(args) == 5 {
			return "c.go", nil
		}
		if args[0] == "diff" && len(args) == 3 && args[2] == "--cached" {
			return "", errors.New("staged fail")
		}
		return "", nil
	}
	if _, err := changedFiles(t.TempDir(), "abc", true); err == nil {
		t.Fatal("staged")
	}

	run = func(root string, args ...string) (string, error) {
		if args[0] == "diff" && len(args) > 2 {
			return "c.go", nil
		}
		if args[0] == "diff" && len(args) == 3 {
			return "s.go", nil
		}
		if args[0] == "diff" && len(args) == 2 {
			return "", errors.New("unstaged fail")
		}
		return "", nil
	}
	if _, err := changedFiles(t.TempDir(), "abc", true); err == nil {
		t.Fatal("unstaged")
	}

	run = func(root string, args ...string) (string, error) {
		if args[0] == "ls-files" {
			return "", errors.New("ls fail")
		}
		if args[0] == "diff" && len(args) > 2 {
			return "c.go", nil
		}
		if args[0] == "diff" {
			return "", nil
		}
		return "", nil
	}
	if _, err := changedFiles(t.TempDir(), "abc", true); err == nil {
		t.Fatal("ls-files")
	}
}

func TestResolve_HeadError(t *testing.T) {
	old := run
	defer func() { run = old }()
	run = func(root string, args ...string) (string, error) {
		if args[0] == "rev-parse" && len(args) > 1 && args[1] == "--is-inside-work-tree" {
			return "true", nil
		}
		if args[0] == "rev-parse" && len(args) > 1 && args[1] == "HEAD" {
			return "", errors.New("head fail")
		}
		return "", errors.New("x")
	}
	if _, err := Resolve(t.TempDir(), "main"); err == nil {
		t.Fatal("head")
	}
}

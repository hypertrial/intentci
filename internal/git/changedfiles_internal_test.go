package git

import (
	"errors"
	"testing"
)

func TestChangedFiles_DirtyBranches(t *testing.T) {
	old := run
	defer func() { run = old }()
	run = func(root string, args ...string) (string, error) {
		switch {
		case args[0] == "diff" && len(args) > 1 && args[1] == "--name-only" && len(args) > 2:
			return "committed.go", nil
		case args[0] == "diff" && len(args) > 1 && args[1] == "--name-only" && len(args) == 2:
			return "", errors.New("unstaged fail")
		default:
			return "", nil
		}
	}
	if _, err := changedFiles(t.TempDir(), "abc", true); err == nil {
		t.Fatal("unstaged error")
	}

	run = func(root string, args ...string) (string, error) {
		switch {
		case args[0] == "diff" && len(args) > 2:
			return "c.go", nil
		case args[0] == "diff":
			return "staged.go", nil
		case args[0] == "ls-files":
			return "untracked.go", nil
		default:
			return "", nil
		}
	}
	files, err := changedFiles(t.TempDir(), "abc", true)
	if err != nil || len(files) < 2 {
		t.Fatalf("%v %v", files, err)
	}

	run = func(root string, args ...string) (string, error) {
		if args[0] == "diff" && len(args) > 2 {
			return "only.go", nil
		}
		return "", nil
	}
	files, err = changedFiles(t.TempDir(), "abc", false)
	if err != nil || len(files) != 1 {
		t.Fatalf("clean: %v %v", files, err)
	}
}

func TestResolve_BaseCommitError(t *testing.T) {
	old := run
	defer func() { run = old }()
	run = func(root string, args ...string) (string, error) {
		switch {
		case args[0] == "rev-parse" && len(args) > 1 && args[1] == "--is-inside-work-tree":
			return "true", nil
		case args[0] == "rev-parse" && len(args) > 1 && args[1] == "HEAD":
			return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil
		case args[0] == "rev-parse" && len(args) > 1 && args[1] == "--verify":
			return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", nil
		case args[0] == "rev-parse" && len(args) > 1:
			return "", errors.New("bad base")
		default:
			return "", errors.New("x")
		}
	}
	if _, err := Resolve(t.TempDir(), "main"); err == nil {
		t.Fatal("base commit")
	}
}

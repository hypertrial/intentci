package git

import (
	"errors"
	"testing"
)

func TestResolve_AbsAndBaseCommitError(t *testing.T) {
	old := absPath
	defer func() { absPath = old }()
	absPath = func(string) (string, error) { return "", errors.New("abs") }
	if _, err := Resolve("/tmp", "HEAD"); err == nil {
		t.Fatal("abs")
	}
	absPath = old

	oldRun := run
	defer func() { run = oldRun }()
	run = func(root string, args ...string) (string, error) {
		switch {
		case args[0] == "rev-parse" && len(args) > 1 && args[1] == "--is-inside-work-tree":
			return "true", nil
		case args[0] == "rev-parse" && len(args) > 1 && args[1] == "HEAD":
			return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil
		case args[0] == "rev-parse" && len(args) > 1 && args[1] == "--verify":
			return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", nil
		case args[0] == "rev-parse":
			return "", errors.New("base commit fail")
		case args[0] == "merge-base":
			return "cccccccccccccccccccccccccccccccccccccccc", nil
		case args[0] == "status":
			return "", nil
		case args[0] == "diff":
			return "", nil
		default:
			return "", nil
		}
	}
	if _, err := Resolve(t.TempDir(), "main"); err == nil {
		t.Fatal("base commit")
	}

	run = func(root string, args ...string) (string, error) {
		if args[0] == "rev-parse" && len(args) > 1 && args[1] == "--is-inside-work-tree" {
			return "true", nil
		}
		if args[0] == "rev-parse" && len(args) > 1 && args[1] == "HEAD" {
			return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil
		}
		if args[0] == "rev-parse" && len(args) > 1 && args[1] == "--verify" {
			return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", nil
		}
		if args[0] == "rev-parse" {
			return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", nil
		}
		if args[0] == "merge-base" {
			return "cccccccccccccccccccccccccccccccccccccccc", nil
		}
		if args[0] == "status" {
			return "", nil
		}
		if args[0] == "diff" && len(args) > 1 && args[1] == "--name-only" && len(args) > 2 {
			return "", errors.New("diff fail")
		}
		return "", nil
	}
	if _, err := Resolve(t.TempDir(), "main"); err == nil {
		t.Fatal("diff fail")
	}
}

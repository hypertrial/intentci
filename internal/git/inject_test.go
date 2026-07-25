package git

import (
	"errors"
	"testing"
)

func TestRunErrorPaths(t *testing.T) {
	old := run
	defer func() { run = old }()
	run = func(root string, args ...string) (string, error) {
		return "", errors.New("boom")
	}
	if _, err := Resolve(t.TempDir(), "HEAD"); err == nil {
		t.Fatal()
	}
	run = func(root string, args ...string) (string, error) {
		if args[0] == "rev-parse" && args[1] == "--is-inside-work-tree" {
			return "true", nil
		}
		if args[0] == "rev-parse" && args[1] == "HEAD" {
			return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil
		}
		if args[0] == "rev-parse" && args[1] == "--verify" {
			return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", nil
		}
		if args[0] == "rev-parse" {
			return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", nil
		}
		if args[0] == "merge-base" {
			return "", errors.New("unrelated")
		}
		if args[0] == "status" {
			return "", errors.New("status fail")
		}
		return "", errors.New("other")
	}
	if _, err := Resolve(t.TempDir(), "main"); err == nil {
		t.Fatal("status fail")
	}
	run = func(root string, args ...string) (string, error) {
		switch {
		case args[0] == "rev-parse" && len(args) > 1 && args[1] == "--is-inside-work-tree":
			return "true", nil
		case args[0] == "rev-parse" && len(args) > 1 && args[1] == "HEAD":
			return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil
		case args[0] == "rev-parse" && len(args) > 1 && args[1] == "--verify":
			return "b", nil
		case args[0] == "rev-parse":
			return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", nil
		case args[0] == "merge-base":
			return "cccccccccccccccccccccccccccccccccccccccc", nil
		case args[0] == "status":
			return " M x", nil
		case args[0] == "diff" && len(args) > 1 && args[1] == "--name-only" && len(args) > 2 && args[2] == "--cached":
			return "staged.txt", nil
		case args[0] == "diff" && len(args) > 1 && args[1] == "--name-only" && len(args) == 2:
			return "unstaged.txt", nil
		case args[0] == "diff":
			return "committed.txt", nil
		case args[0] == "ls-files":
			return "untracked.txt", nil
		default:
			return "", errors.New(args[0])
		}
	}
	st, err := Resolve(t.TempDir(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if !st.WorkingTreeDirty || len(st.ChangedFiles) < 3 {
		t.Fatalf("%+v", st)
	}
	// empty stderr path in runGit
	if _, err := runGit(t.TempDir(), "not-a-git-subcommand-zzz"); err == nil {
		t.Fatal()
	}
}

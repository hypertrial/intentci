package git

import (
	"errors"
	"os/exec"
	"testing"
)

func TestContainsAndRunGitStderrEmpty(t *testing.T) {
	if contains([]string{"a", "b"}, "b") != true {
		t.Fatal("contains")
	}
	if contains([]string{"a"}, "z") {
		t.Fatal("missing")
	}
	old := run
	defer func() { run = old }()
	run = func(root string, args ...string) (string, error) {
		return "", errors.New("boom")
	}
	if IsRepo(t.TempDir()) {
		t.Fatal("expected false")
	}
	oldExec := execCommand
	defer func() { execCommand = oldExec }()
	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false") // fails with empty stderr
	}
	if _, err := runGit(t.TempDir(), "rev-parse", "--is-inside-work-tree"); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveInjectedErrors(t *testing.T) {
	oldAbs, oldRun := absPath, run
	defer func() { absPath, run = oldAbs, oldRun }()

	absPath = func(string) (string, error) { return "", errors.New("abs") }
	if _, err := Resolve(".", "HEAD"); err == nil {
		t.Fatal("abs")
	}
	absPath = func(p string) (string, error) { return "/tmp/repo", nil }

	seq := []struct {
		args string
		err  error
		out  string
	}{
		{"rev-parse --is-inside-work-tree", nil, "true"},
		{"rev-parse HEAD", errors.New("no head"), ""},
	}
	i := 0
	run = func(root string, args ...string) (string, error) {
		key := joinArgs(args)
		if i < len(seq) && key == seq[i].args {
			e := seq[i]
			i++
			return e.out, e.err
		}
		return "", errors.New("unexpected " + key)
	}
	if _, err := Resolve(".", "HEAD"); err == nil {
		t.Fatal("head")
	}

	// baseCommit fail after verify ok
	i = 0
	seq = []struct {
		args string
		err  error
		out  string
	}{
		{"rev-parse --is-inside-work-tree", nil, "true"},
		{"rev-parse HEAD", nil, "h"},
		{"rev-parse --verify origin/main", nil, "origin/main"},
		{"rev-parse origin/main", errors.New("base"), ""},
	}
	run = func(root string, args ...string) (string, error) {
		key := joinArgs(args)
		for _, s := range seq {
			if s.args == key {
				return s.out, s.err
			}
		}
		return "", errors.New("unexpected " + key)
	}
	if _, err := Resolve(".", ""); err == nil {
		t.Fatal("baseCommit")
	}

	// merge-base, diff, status errors + rename + empty path
	run = func(root string, args ...string) (string, error) {
		switch joinArgs(args) {
		case "rev-parse --is-inside-work-tree":
			return "true", nil
		case "rev-parse HEAD":
			return "hhhhhhhhhhhhhhhh", nil
		case "rev-parse --verify base":
			return "base", nil
		case "rev-parse base":
			return "bbbbbbbbbbbbbbbb", nil
		case "merge-base bbbbbbbbbbbbbbbb hhhhhhhhhhhhhhhh":
			return "", errors.New("mb")
		default:
			return "", errors.New("unexpected " + joinArgs(args))
		}
	}
	if _, err := Resolve(".", "base"); err == nil {
		t.Fatal("merge-base")
	}

	run = func(root string, args ...string) (string, error) {
		switch joinArgs(args) {
		case "rev-parse --is-inside-work-tree":
			return "true", nil
		case "rev-parse HEAD":
			return "hhhhhhhhhhhhhhhh", nil
		case "rev-parse --verify base":
			return "base", nil
		case "rev-parse base":
			return "bbbbbbbbbbbbbbbb", nil
		case "merge-base bbbbbbbbbbbbbbbb hhhhhhhhhhhhhhhh":
			return "mmmmmmmmmmmmmmmm", nil
		case "diff --name-only mmmmmmmmmmmmmmmm":
			return "", errors.New("diff")
		default:
			return "", errors.New("unexpected " + joinArgs(args))
		}
	}
	if _, err := Resolve(".", "base"); err == nil {
		t.Fatal("diff")
	}

	run = func(root string, args ...string) (string, error) {
		switch joinArgs(args) {
		case "rev-parse --is-inside-work-tree":
			return "true", nil
		case "rev-parse HEAD":
			return "hhhhhhhhhhhhhhhh", nil
		case "rev-parse --verify base":
			return "base", nil
		case "rev-parse base":
			return "bbbbbbbbbbbbbbbb", nil
		case "merge-base bbbbbbbbbbbbbbbb hhhhhhhhhhhhhhhh":
			return "mmmmmmmmmmmmmmmm", nil
		case "diff --name-only mmmmmmmmmmmmmmmm":
			return "a.go\n", nil
		case "status --porcelain":
			return "", errors.New("status")
		default:
			return "", errors.New("unexpected " + joinArgs(args))
		}
	}
	if _, err := Resolve(".", "base"); err == nil {
		t.Fatal("status")
	}

	run = func(root string, args ...string) (string, error) {
		switch joinArgs(args) {
		case "rev-parse --is-inside-work-tree":
			return "true", nil
		case "rev-parse HEAD":
			return "hhhhhhhhhhhhhhhh", nil
		case "rev-parse --verify base":
			return "base", nil
		case "rev-parse base":
			return "bbbbbbbbbbbbbbbb", nil
		case "merge-base bbbbbbbbbbbbbbbb hhhhhhhhhhhhhhhh":
			return "mmmmmmmmmmmmmmmm", nil
		case "diff --name-only mmmmmmmmmmmmmmmm":
			return "a.go\n", nil
		case "status --porcelain":
			return " M a.go\n??  \nR  old.go -> new.go\nXX\n", nil
		default:
			return "", errors.New("unexpected " + joinArgs(args))
		}
	}
	st, err := Resolve(".", "base")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(st.ChangedFiles, "new.go") || !st.WorkingTreeDirty {
		t.Fatalf("%+v", st)
	}
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}

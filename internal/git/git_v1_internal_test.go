package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWithOptionsFailures(t *testing.T) {
	if _, err := ResolveWithOptions(t.TempDir(), ResolveOptions{}); err == nil {
		t.Fatal("non-repository accepted")
	}
	oldAbs, oldRun := absPath, run
	defer func() { absPath, run = oldAbs, oldRun }()

	absPath = func(string) (string, error) { return "", errors.New("absolute") }
	if _, err := ResolveWithOptions(".", ResolveOptions{HeadRef: "topic"}); err == nil {
		t.Fatal("absolute failure ignored")
	}
	absPath = func(string) (string, error) { return "/repo", nil }

	fake := func(fail string) func(string, ...string) (string, error) {
		return func(_ string, arguments ...string) (string, error) {
			key := strings.Join(arguments, " ")
			if key == fail {
				return "", errors.New(fail)
			}
			switch key {
			case "rev-parse --is-inside-work-tree":
				return "true", nil
			case "rev-parse HEAD", "rev-parse topic":
				return "head", nil
			case "rev-parse --verify base":
				return "base", nil
			case "rev-parse base", "rev-parse origin/main":
				return "base", nil
			case "merge-base base head":
				return "merge", nil
			case "diff --name-only merge", "status --porcelain",
				"diff --name-status --find-renames merge",
				"diff --numstat --find-renames merge",
				"diff --raw --find-renames merge",
				"diff --binary --no-ext-diff merge",
				"diff --name-status --find-renames merge..head",
				"diff --numstat --find-renames merge..head",
				"diff --raw --find-renames merge..head",
				"diff --binary --no-ext-diff merge..head":
				return "", nil
			default:
				return "", errors.New("unexpected " + key)
			}
		}
	}
	run = fake("diff --name-status --find-renames merge")
	if _, err := ResolveWithOptions(".", ResolveOptions{BaseRef: "base"}); err == nil {
		t.Fatal("HEAD enrichment failure ignored")
	}
	for _, failure := range []string{
		"rev-parse topic", "rev-parse origin/main", "merge-base base head",
		"diff --name-status --find-renames merge..head",
	} {
		run = fake(failure)
		if _, err := ResolveWithOptions(".", ResolveOptions{HeadRef: "topic"}); err == nil {
			t.Fatalf("%s ignored", failure)
		}
	}
	run = fake("")
	state, err := ResolveWithOptions(".", ResolveOptions{HeadRef: "topic"})
	if err != nil || state.BaseRef != "origin/main" || state.MergeBase != "merge" {
		t.Fatalf("%+v %v", state, err)
	}
}

func TestEnrichFailuresAndParsingEdges(t *testing.T) {
	oldRun := run
	defer func() { run = oldRun }()
	baseResponses := map[string]string{
		"diff --name-status --find-renames base": "M\ta\n",
		"diff --numstat --find-renames base":     "1\t2\ta\n",
		"diff --raw --find-renames base":         ":100644 100755 abc def M\ta\n",
		"diff --binary --no-ext-diff base":       "patch",
		"status --porcelain":                     "",
	}
	for _, failure := range []string{
		"diff --name-status --find-renames base",
		"diff --numstat --find-renames base",
		"diff --raw --find-renames base",
		"diff --binary --no-ext-diff base",
		"status --porcelain",
	} {
		run = func(_ string, arguments ...string) (string, error) {
			key := strings.Join(arguments, " ")
			if key == failure {
				return "", errors.New(failure)
			}
			return baseResponses[key], nil
		}
		if err := enrich(&State{Root: t.TempDir(), MergeBaseFull: "base"}, false, ""); err == nil {
			t.Fatalf("%s ignored", failure)
		}
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plain"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	run = func(_ string, arguments ...string) (string, error) {
		key := strings.Join(arguments, " ")
		switch key {
		case "diff --name-status --find-renames base":
			return "M\ta\n", nil
		case "diff --numstat --find-renames base":
			return "1\t2\tunknown\n3\t4\talso-unknown\nbad", nil
		case "diff --raw --find-renames base":
			return "short\n:100644 100755 abc def M\ta", nil
		case "diff --binary --no-ext-diff base":
			return "patch", nil
		case "status --porcelain":
			return "??   \n?? plain", nil
		default:
			return "", errors.New(key)
		}
	}
	state := &State{Root: root, MergeBaseFull: "base"}
	if err := enrich(state, true, ""); err != nil {
		t.Fatal(err)
	}
	if len(state.Changes) != 2 || state.Changes[1].Additions != 1 {
		t.Fatalf("%+v", state.Changes)
	}

	run = func(_ string, arguments ...string) (string, error) {
		key := strings.Join(arguments, " ")
		if key == "diff --numstat --find-renames base" {
			return "-\t-\ta", nil
		}
		return baseResponses[key], nil
	}
	state = &State{Root: root, MergeBaseFull: "base"}
	if err := enrich(state, false, "base"); err != nil || !state.Changes[0].Binary {
		t.Fatalf("%+v %v", state, err)
	}

	changes := parseNameStatus("bad\nC100\told\tcopy\nT\tkind\nA\tnew")
	if len(changes) != 3 || changes[0].Status != "copied" ||
		changes[1].Status != "type_changed" || changes[2].Status != "added" {
		t.Fatalf("%+v", changes)
	}
	if short("tiny") != "tiny" {
		t.Fatal("short commit")
	}
}

func TestDiffUntrackedUnexpectedExit(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()
	execCommand = func(string, ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "exit 2")
	}
	if _, err := diffUntracked(t.TempDir(), "file"); err == nil {
		t.Fatal("unexpected exit accepted")
	}
}

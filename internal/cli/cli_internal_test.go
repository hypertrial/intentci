package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/verdict"
)

func TestExitErrorAndMustConfig(t *testing.T) {
	e := &ExitError{Code: 1}
	if e.Error() != "exit 1" {
		t.Fatal(e.Error())
	}
	e.Msg = "hi"
	if e.Error() != "hi" {
		t.Fatal(e.Error())
	}
	err := exitErr(2, "x %d", 1)
	if err.Error() != "x 1" {
		t.Fatal(err)
	}
	cfg := mustConfig(t.TempDir())
	if cfg.Project.Name == "" {
		t.Fatal("default")
	}
	if short("abcdefghi") != "abcdefg" {
		t.Fatal(short("abcdefghi"))
	}
	if short("abc") != "abc" {
		t.Fatal(short("abc"))
	}

	old := getwd
	defer func() { getwd = old }()
	getwd = func() (string, error) { return "", errors.New("cwd") }
	var out, errb bytes.Buffer
	if code := RunMain([]string{"init"}, &out, &errb); code != exitcode.Internal {
		t.Fatalf("code=%d", code)
	}
	for _, args := range [][]string{
		{"compile"}, {"verify", "--all"}, {"explain", "X"}, {"repair"}, {"status"}, {"doctor"},
	} {
		out.Reset()
		errb.Reset()
		_ = RunMain(args, &out, &errb)
	}
}

func TestStatusVerdictCounts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `version: 1
project: {name: demo}
requirements:
  paths: [".intentci/requirements/**/*.md"]
`
	if err := os.WriteFile(filepath.Join(root, ".intentci", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := evidence.NewStore(root, ".intentci/runs")
	if err != nil {
		t.Fatal(err)
	}
	b := &evidence.Bundle{
		RunID: "r", HeadCommit: "abc",
		Run: verdict.RunResult{Verdict: verdict.Fail, Requirements: []verdict.RequirementResult{
			{Verdict: "pass"}, {Verdict: "fail"}, {Verdict: "unproven"}, {Verdict: "uncertain"}, {Verdict: "error"},
		}},
	}
	if err := store.WriteBundle(b); err != nil {
		t.Fatal(err)
	}
	old := getwd
	defer func() { getwd = old }()
	getwd = func() (string, error) { return root, nil }
	var out, errb bytes.Buffer
	if code := RunMain([]string{"status"}, &out, &errb); code != 0 {
		t.Fatalf("%d %s", code, errb.String())
	}
}

func TestRunMainExitErrorType(t *testing.T) {
	var errb bytes.Buffer
	code := RunMain([]string{"schema", "nope"}, &bytes.Buffer{}, &errb)
	if code != exitcode.Usage || errb.Len() == 0 {
		t.Fatalf("%d %s", code, errb.String())
	}
}

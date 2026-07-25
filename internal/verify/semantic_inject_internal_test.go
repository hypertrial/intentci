package verify

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/semantic"
)

func TestRun_SemanticEnsureTrustAndError(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "-c", "core.hooksPath=/dev/null", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for _, args := range [][]string{
		{"checkout", "-b", "main"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		_ = c.Run()
	}
	if err := os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `version: 1
product: {name: x, purpose: y}
policy:
  default_base: HEAD
  semantic:
    enabled: true
    enforcement: advisory
    provider:
      type: local
      command: /bin/true
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    applies_to: {include: ["**"]}
    verification: {checks: [c], semantic: optional}
checks:
  - id: c
    command: "true"
    profiles: [fast]
    timeout: 1m
`
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run("git", "add", ".")
	run("git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "i")

	// Trust false, no full-profile checks → EnsureTrust closure runs for local semantic.
	_, err := Run(context.Background(), Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: false,
		Stdout: os.Stdout, Stderr: os.Stderr, NoCache: true,
	})
	// May fail trust interactively; that's ok — closure was invoked.
	_ = err

	old := runSemantic
	defer func() { runSemantic = old }()
	runSemantic = func(ctx context.Context, opt semantic.RunOptions) (semantic.RunResult, error) {
		return semantic.RunResult{}, errors.New("sem boom")
	}
	outcome, err := Run(context.Background(), Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: true,
		Stdout: os.Stdout, Stderr: os.Stderr, NoCache: true,
	})
	if err == nil || outcome.ExitCode != 30 {
		t.Fatalf("%v %#v", err, outcome)
	}

	// Preview path error
	outcome, err = Run(context.Background(), Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: true,
		ShowSemanticInput: true, Stdout: os.Stdout, Stderr: os.Stderr, NoCache: true,
	})
	if err == nil || outcome.ExitCode != 30 {
		t.Fatalf("preview err: %v %#v", err, outcome)
	}

	runSemantic = func(ctx context.Context, opt semantic.RunOptions) (semantic.RunResult, error) {
		return semantic.RunResult{ShowedInput: false}, nil
	}
	outcome, err = Run(context.Background(), Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: true,
		ShowSemanticInput: true, Stdout: os.Stdout, Stderr: os.Stderr, NoCache: true,
	})
	if err == nil || outcome.ExitCode != 30 {
		t.Fatalf("preview missing render: %v %#v", err, outcome)
	}
}

package verify_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/semantic"
	"github.com/hypertrial/intentci/internal/verify"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestRun_SemanticShowAndMerge(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := buildSemanticFixture(t)
	body := `version: 1
product: {name: x, purpose: y}
policy:
  default_base: HEAD
  semantic:
    enabled: true
    enforcement: advisory
    provider:
      type: local
      command: ` + bin + `
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    applies_to: {include: ["**"]}
    verification: {checks: [c], semantic: required}
checks:
  - id: c
    command: "true"
    timeout: 1m
`
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.go"), []byte("package f\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "i")

	var out bytes.Buffer
	outcome, err := verify.Run(context.Background(), verify.Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: true,
		ShowSemanticInput: true, Stdout: &out, Stderr: os.Stderr, NoCache: true,
	})
	if err != nil || outcome.ExitCode != 0 || outcome.Result != nil {
		t.Fatalf("show: %v %#v %s", err, outcome, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("protocol_version")) {
		t.Fatalf("%s", out.String())
	}

	t.Setenv("INTENTCI_SEMANTIC_FIXTURE", "insufficient")
	outcome, err = verify.Run(context.Background(), verify.Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: true,
		Stdout: os.Stdout, Stderr: os.Stderr, NoCache: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Result.Semantic == nil || !outcome.Result.Semantic.Enabled {
		t.Fatalf("%+v", outcome.Result.Semantic)
	}
	if outcome.Result.Status != protocol.StatusUnverified {
		t.Fatalf("status %s", outcome.Result.Status)
	}
}

func TestRun_SemanticProviderUnavailable(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `version: 1
product: {name: x, purpose: y}
policy:
  default_base: HEAD
  semantic:
    enabled: true
    enforcement: blocking
    provider:
      type: local
      command: ./does-not-exist-provider
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    applies_to: {include: ["**"]}
    verification: {checks: [c], semantic: required}
checks:
  - id: c
    command: "true"
    timeout: 1m
`
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.go"), []byte("package f\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "i")

	outcome, err := verify.Run(context.Background(), verify.Options{
		Root: dir, Base: "HEAD", Profile: "full", All: true, Trust: true,
		Stdout: os.Stdout, Stderr: os.Stderr, NoCache: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != 12 || outcome.Result.Status != protocol.StatusUnknown {
		t.Fatalf("status=%s exit=%d", outcome.Result.Status, outcome.ExitCode)
	}
	_ = semantic.ProtocolVersion
}

func buildSemanticFixture(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "provider")
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = filepath.Join(root, "fixtures", "semantic-provider")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, b)
	}
	return out
}

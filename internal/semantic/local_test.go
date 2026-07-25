package semantic_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/semantic"
)

func TestLocalProvider_Fixture(t *testing.T) {
	bin := buildFixtureProvider(t)
	p := &semantic.LocalProvider{Command: bin, Dir: t.TempDir()}
	t.Setenv("INTENTCI_SEMANTIC_FIXTURE", "contradiction")
	resp, err := p.Analyze(context.Background(), semantic.Request{
		ProtocolVersion: 1,
		Requirements:    []semantic.RequirementContext{{ID: "R-1", Statement: "s"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Findings) != 1 || resp.Findings[0].Assessment != semantic.AssessmentContradiction {
		t.Fatalf("%+v", resp)
	}
}

func TestLocalProvider_Fail(t *testing.T) {
	bin := buildFixtureProvider(t)
	p := &semantic.LocalProvider{Command: bin}
	t.Setenv("INTENTCI_SEMANTIC_FIXTURE", "fail")
	if _, err := p.Analyze(context.Background(), semantic.Request{Requirements: []semantic.RequirementContext{{ID: "R-1"}}}); err == nil {
		t.Fatal("expected fail")
	}
}

func TestNewProvider(t *testing.T) {
	if _, err := semantic.NewProvider(nil); err == nil {
		t.Fatal("nil")
	}
	if _, err := semantic.NewProvider(&contract.SemanticProvider{Type: "local", Command: "true"}); err != nil {
		t.Fatal(err)
	}
	if _, err := semantic.NewProvider(&contract.SemanticProvider{Type: "http", URL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := semantic.NewProvider(&contract.SemanticProvider{Type: "nope"}); err == nil {
		t.Fatal("bad type")
	}
	if _, err := semantic.NewProvider(&contract.SemanticProvider{Type: "local", Command: "x", Timeout: "bad"}); err == nil {
		t.Fatal("bad timeout")
	}
}

func buildFixtureProvider(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "provider")
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = filepath.Join(root, "fixtures", "semantic-provider")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, outBytes)
	}
	return out
}

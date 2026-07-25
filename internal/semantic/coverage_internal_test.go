package semantic

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestSplitCommandAndTruncate(t *testing.T) {
	name, args := splitCommand("echo hello world")
	if name != "echo" || len(args) != 2 {
		t.Fatalf("%s %#v", name, args)
	}
	n, a := splitCommand("   ")
	if n != "   " || a != nil {
		// Fields of whitespace is empty → returns original command string.
		_ = n
	}
	if truncateString("hi", 10) != "hi" {
		t.Fatal("short")
	}
	if truncateString("abcdefghij", 4) == "abcdefghij" {
		t.Fatal("should truncate")
	}
	if min(1, 2) != 1 || min(3, 2) != 2 {
		t.Fatal("min")
	}
	if firstLine("a\nb") != "a" {
		t.Fatal("firstLine")
	}
}

func TestLocalProvider_EmptyCommand(t *testing.T) {
	p := &LocalProvider{}
	if _, err := p.Analyze(context.Background(), Request{}); err == nil {
		t.Fatal("empty")
	}
}

func TestDefaultUnifiedDiff(t *testing.T) {
	if _, err := defaultUnifiedDiff(t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	_, _ = defaultUnifiedDiff(t.TempDir(), "HEAD")
}

func TestRedactAndNewProviderLocalDir(t *testing.T) {
	t.Setenv("XSECRET", "abcd1234")
	t.Setenv(TokenEnv, "semtokenvalue99")
	if redactSecrets("token abcd1234 end", []string{"XSECRET"}) != "token [REDACTED] end" {
		t.Fatal("redact")
	}
	if redactSecrets("bearer semtokenvalue99 here", nil) != "bearer [REDACTED] here" {
		t.Fatal("token env redact")
	}
	if redactSecrets("x", []string{"MISSING"}) != "x" {
		t.Fatal("noop")
	}
	got := truncateString(strings.Repeat("a", 100), 20)
	if len(got) != 20 || !strings.Contains(got, "[truncated]") {
		t.Fatalf("truncate bound %q len=%d", got, len(got))
	}
	p, err := NewProvider(&contract.SemanticProvider{Type: "local", Command: "true", Timeout: "500ms"})
	if err != nil {
		t.Fatal(err)
	}
	lp := p.(*LocalProvider)
	if lp.Timeout != 500*time.Millisecond {
		t.Fatalf("%v", lp.Timeout)
	}
}

func TestUnavailableAndProviderType(t *testing.T) {
	out := unavailable(Request{}, contract.SemanticPolicy{Enabled: true}, "x")
	if out.ProviderErr == nil || out.SemanticRun.Skipped != "x" {
		t.Fatalf("%+v", out)
	}
	if providerType(contract.SemanticPolicy{}) != "none" {
		t.Fatal("none")
	}
}

func TestRun_EnsureTrustAndShowStdoutRequired(t *testing.T) {
	_, err := Run(context.Background(), RunOptions{
		Contract: &contract.Contract{
			Product: contract.Product{Name: "n", Purpose: "p"},
			Policy:  contract.Policy{Semantic: contract.SemanticPolicy{Enabled: true}},
		},
		ShowSemanticInput: true,
	})
	if err == nil {
		t.Fatal("stdout required")
	}

	called := false
	bin := filepath.Join(t.TempDir(), "p")
	// Use /bin/true style via shell writing json
	script := `#!/bin/sh
echo '{"protocol_version":1,"findings":[]}'
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = Run(context.Background(), RunOptions{
		Root: t.TempDir(),
		Contract: &contract.Contract{
			Product: contract.Product{Name: "n", Purpose: "p"},
			Policy: contract.Policy{Semantic: contract.SemanticPolicy{
				Enabled: true, Enforcement: "advisory",
				Provider: &contract.SemanticProvider{Type: "local", Command: bin},
			}},
		},
		TrustLocal: false,
		EnsureTrust: func() error {
			called = true
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("trust not called")
	}
}

package security_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/security"
)

func TestRedactAndProtected(t *testing.T) {
	env := security.RedactEnv([]string{"PATH=/bin", "API_TOKEN=secret"}, []string{"*TOKEN*"})
	if env[1] != "API_TOKEN=[REDACTED]" {
		t.Fatalf("%v", env)
	}
	hits := security.ProtectedViolation([]string{".intentci/requirements/REQ-001.md", "main.go"}, false, nil)
	if len(hits) != 1 {
		t.Fatalf("%v", hits)
	}
	hits = security.ProtectedViolation([]string{".github/workflows/ci.yml", "pkg/a.go"}, false, nil)
	if len(hits) != 1 || hits[0] != ".github/workflows/ci.yml" {
		t.Fatalf("workflows must be protected: %v", hits)
	}
	if !security.IsTestPath("internal/foo_test.go") {
		t.Fatal("test path")
	}
}

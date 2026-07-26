package security_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/security"
)

func TestRedactEmptyAndProtectedAllow(t *testing.T) {
	env := []string{"A=1"}
	if got := security.RedactEnv(env, nil); len(got) != 1 || got[0] != "A=1" {
		t.Fatal(got)
	}
	hits := security.ProtectedViolation([]string{".intentci/requirements/x.md", ".intentci/schemas/a.json", "main.go"}, true, []string{"extra/**"})
	for _, h := range hits {
		if h == ".intentci/requirements/x.md" {
			t.Fatalf("requirements should be allowed: %v", hits)
		}
	}
	if !security.IsTestPath("tests/foo.go") || !security.IsTestPath("pkg/test/x.go") {
		t.Fatal("test paths")
	}
	if security.IsTestPath("main.go") {
		t.Fatal("not test")
	}
}

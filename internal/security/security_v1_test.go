package security_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/security"
)

func TestRedactorRemovesNamesAndValues(t *testing.T) {
	redactor := security.NewRedactor(
		[]string{"*TOKEN*", "*PASSWORD*"},
		[]string{"API_TOKEN=long-secret", "PASSWORD=hunter2", "PATH=/bin"},
	)
	got := redactor.Redact("long-secret API_TOKEN=other PASSWORD: visible PATH=/bin")
	if strings.Contains(got, "long-secret") || strings.Contains(got, "other") ||
		strings.Contains(got, "visible") || !strings.Contains(got, "PATH=/bin") {
		t.Fatalf("redaction failed: %s", got)
	}
	if unchanged := security.NewRedactor(nil, nil).Redact("plain"); unchanged != "plain" {
		t.Fatal(unchanged)
	}
}

func TestResolveInsidePathSafety(t *testing.T) {
	root := t.TempDir()
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "safe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "safe", "file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"safe/file", "safe/missing", "safe/missing/child"} {
		path, err := security.ResolveInside(root, relative)
		if err != nil || !strings.HasPrefix(path, rootReal) {
			t.Fatalf("%s: path=%s err=%v", relative, path, err)
		}
	}
	for _, unsafe := range []string{"", "../escape", filepath.Join(root, "safe", "file")} {
		_, err := security.ResolveInside(root, unsafe)
		if err == nil || !security.IsPathViolation(err) {
			t.Fatalf("%q: %v", unsafe, err)
		}
	}
	if _, err := security.ResolveInside(filepath.Join(root, "missing-root"), "x"); err == nil {
		t.Fatal("missing root accepted")
	}

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "outside-link")); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"outside-link", "outside-link/new"} {
		_, err := security.ResolveInside(root, relative)
		if err == nil || !security.IsPathViolation(err) {
			t.Fatalf("%s: %v", relative, err)
		}
	}
	if err := os.Symlink("safe", filepath.Join(root, "inside-link")); err != nil {
		t.Fatal(err)
	}
	if _, err := security.ResolveInside(root, "inside-link/file"); err != nil {
		t.Fatal(err)
	}
}

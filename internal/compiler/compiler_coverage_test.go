package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/compiler"
	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/ir"
)

func writeReq(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".intentci", "requirements")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompileLoadConfigNilAndFilter(t *testing.T) {
	root := writeRepo(t, 2)
	res, err := compiler.Compile(compiler.Options{Root: root, RequirementID: "REQ-001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Document.Requirements) != 1 {
		t.Fatalf("%d", len(res.Document.Requirements))
	}
	// nil config loads from disk
	res, err = compiler.Compile(compiler.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Document.Requirements) != 2 {
		t.Fatal(len(res.Document.Requirements))
	}
}

func TestCompileMissingConfig(t *testing.T) {
	_, err := compiler.Compile(compiler.Options{Root: t.TempDir()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompileCycleUnknownDepProvidersBoundaries(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `version: 1
project: {name: demo}
requirements:
  paths: [".intentci/requirements/**/*.md", ".intentci/requirements"]
`
	if err := os.WriteFile(filepath.Join(root, ".intentci", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	// also write non-md and a dir match noise
	_ = os.WriteFile(filepath.Join(root, ".intentci", "requirements", "skip.txt"), []byte("x"), 0o644)

	a := `---
id: REQ-A
title: a
status: active
priority: required
depends_on: [REQ-B, REQ-MISSING]
---
# Intent
a
# Boundaries
` + "```yaml" + `
allowed: [same]
forbidden: [same]
` + "```" + `
# Obligations
` + "```yaml" + `
- id: O1
  verify:
    all:
      - provider: nope
        id: x
      - provider: command
        id: c
        run: ""
      - provider: boundary
        id: b
      - provider: command
        id: ok
        run: "true"
  verify_unused: 1
- id: O1
  verify:
    any:
      - provider: command
        id: a2
        run: "true"
  statement: dup
- id: O2
  verify:
    not:
      provider: command
      id: n
      run: "true"
` + "```" + `
`
	// fix - can't have verify twice. Rewrite properly.
	a = `---
id: REQ-A
title: a
status: active
priority: required
depends_on: [REQ-B, REQ-MISSING]
---
# Intent
a
# Boundaries
` + "```yaml" + `
allowed: [same]
forbidden: [same]
` + "```" + `
# Obligations
` + "```yaml" + `
- id: O1
  statement: s
  verify:
    all:
      - provider: nope
        id: x
      - provider: command
        id: c
        run: ""
      - provider: boundary
        id: b
      - provider: command
        id: ok
        run: "true"
- id: O1
  statement: dup
  verify:
    any:
      - provider: command
        id: a2
        run: "true"
- id: O2
  statement: n
  verify:
    not:
      provider: command
      id: n
      run: "true"
` + "```" + `
`
	b := `---
id: REQ-B
title: b
status: active
priority: required
depends_on: [REQ-A]
---
# Intent
b
# Obligations
` + "```yaml" + `
- id: O
  verify:
    provider: command
    id: c
    run: "true"
` + "```" + `
`
	writeReq(t, root, "a.md", a)
	writeReq(t, root, "b.md", b)
	res, err := compiler.Compile(compiler.Options{Root: root, Strict: true})
	if err == nil {
		t.Fatalf("expected fail diags=%v", res.Diagnostics)
	}
	if len(res.Diagnostics) == 0 {
		t.Fatal("expected diags")
	}
}

func TestWriteIRErrors(t *testing.T) {
	doc := &ir.Document{SchemaVersion: 1, Project: "p"}
	_ = doc.ComputeHashes()
	// parent is a file
	base := t.TempDir()
	file := filepath.Join(base, "notdir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := compiler.WriteIR(doc, filepath.Join(file, "ir.json")); err == nil {
		t.Fatal("expected mkdir error")
	}
	if err := compiler.WriteIR(doc, filepath.Join(base, "ok", "ir.json")); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverBadPattern(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	cfg.Requirements.Paths = []string{"["} // invalid glob
	if err := os.MkdirAll(filepath.Join(root, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`version: 1
project: {name: demo}
requirements:
  paths: ["["]
`)
	if err := os.WriteFile(filepath.Join(root, ".intentci", "config.yaml"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := compiler.Compile(compiler.Options{Root: root, Config: cfg})
	if err == nil {
		t.Fatal("expected glob error")
	}
}

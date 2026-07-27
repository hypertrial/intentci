package compiler_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/compiler"
	"github.com/hypertrial/intentci/internal/config"
)

func writeRepo(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".intentci", "requirements")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Project.Name = "demo"
	raw := []byte(`version: 1
project: {name: demo}
requirements:
  paths: [".intentci/requirements/**/*.md"]
`)
	if err := os.WriteFile(filepath.Join(root, ".intentci", "config.yaml"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= n; i++ {
		body := fmt.Sprintf(`---
id: REQ-%03d
title: Requirement %d
status: active
priority: required
applies_to:
  paths: ["**"]
---

# Intent

Intent %d

# Obligations

`+"```yaml"+`
- id: OBL-001
  statement: smoke
  required: true
  verify:
    all:
      - provider: command
        id: smoke
        run: "true"
        result: {type: exit_code, equals: 0}
`+"```"+`
`, i, i, i)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("REQ-%03d.md", i)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCompile100(t *testing.T) {
	root := writeRepo(t, 100)
	res, err := compiler.Compile(compiler.Options{Root: root, Strict: true})
	if err != nil {
		t.Fatalf("%v diags=%v", err, res.Diagnostics)
	}
	if len(res.Document.Requirements) != 100 {
		t.Fatalf("got %d", len(res.Document.Requirements))
	}
	if res.Document.Hash == "" {
		t.Fatal("missing hash")
	}
	out := filepath.Join(t.TempDir(), "ir.json")
	if err := compiler.WriteIR(res.Document, out); err != nil {
		t.Fatal(err)
	}
}

func TestCompileDuplicateAndCycle(t *testing.T) {
	root := writeRepo(t, 1)
	// add duplicate
	body := `---
id: REQ-001
title: dup
status: active
priority: required
---
# Intent
x
# Obligations
` + "```yaml" + `
- id: OBL-001
  statement: s
  verify:
    all:
      - provider: command
        id: c
        run: true
` + "```"
	_ = os.WriteFile(filepath.Join(root, ".intentci", "requirements", "dup.md"), []byte(body), 0o644)
	res, err := compiler.Compile(compiler.Options{Root: root, Strict: true})
	if err == nil {
		t.Fatalf("expected error, diags=%v", res.Diagnostics)
	}
}

func TestCompileIsDeterministic(t *testing.T) {
	root := writeRepo(t, 1)
	var want []byte
	for i := 0; i < 25; i++ {
		res, err := compiler.Compile(compiler.Options{Root: root})
		if err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "ir.json")
		if err := compiler.WriteIR(res.Document, out); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			want = got
		} else if !bytes.Equal(got, want) {
			t.Fatalf("compile %d was nondeterministic", i)
		}
	}
}

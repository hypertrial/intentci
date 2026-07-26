package compiler

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/ir"
)

func TestDiscoverAbsAndNonMD(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "reqs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "subdir.md")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	files, err := discover(root, []string{"reqs/**/*"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("%v", files)
	}
	old := absPath
	defer func() { absPath = old }()
	absPath = func(string) (string, error) { return "", errors.New("abs") }
	files, err = discover(root, []string{"reqs/**/*.md"})
	if err != nil || len(files) != 0 {
		t.Fatalf("%v %v", files, err)
	}
}

func TestCompileHashErrorAndNonStrict(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".intentci", "requirements"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgBody := `version: 1
project: {name: demo}
requirements:
  paths: [".intentci/requirements/**/*.md"]
`
	if err := os.WriteFile(filepath.Join(root, ".intentci", "config.yaml"), []byte(cfgBody), 0o644); err != nil {
		t.Fatal(err)
	}
	body := `---
id: BAD
title: t
status: active
priority: required
---
# Intent
i
# Obligations
` + "```yaml" + `
- id: O
  verify:
    provider: command
    id: c
    run: "true"
` + "```"
	if err := os.WriteFile(filepath.Join(root, ".intentci", "requirements", "a.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Project.Name = "demo"
	res, err := Compile(Options{Root: root, Config: cfg, Strict: false})
	if err == nil {
		t.Fatalf("expected hasErrors path, diags=%v", res.Diagnostics)
	}

	oldH := computeHashes
	defer func() { computeHashes = oldH }()
	computeHashes = func(*ir.Document) error { return errors.New("hash") }
	body2 := `---
id: REQ-1
title: t
status: active
priority: required
---
# Intent
i
# Obligations
` + "```yaml" + `
- id: O
  verify:
    provider: command
    id: c
    run: "true"
` + "```"
	root2 := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root2, ".intentci", "requirements"), 0o755)
	_ = os.WriteFile(filepath.Join(root2, ".intentci", "config.yaml"), []byte(cfgBody), 0o644)
	_ = os.WriteFile(filepath.Join(root2, ".intentci", "requirements", "a.md"), []byte(body2), 0o644)
	if _, err := Compile(Options{Root: root2, Config: cfg}); err == nil {
		t.Fatal("hash")
	}
	computeHashes = oldH

	doc := &ir.Document{SchemaVersion: 1, Project: "p", Requirements: []ir.Requirement{{
		ID: "R", Obligations: []ir.Obligation{{ID: "O", Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
			Provider: "command", Extra: map[string]any{"ch": make(chan int)},
		}}}},
	}}}
	oldW := writeFile
	defer func() { writeFile = oldW }()
	writeFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	doc2 := &ir.Document{SchemaVersion: 1, Project: "p"}
	_ = doc2.ComputeHashes()
	if err := WriteIR(doc2, filepath.Join(t.TempDir(), "ir.json")); err == nil {
		t.Fatal("write")
	}
	writeFile = oldW
	if err := WriteIR(doc, filepath.Join(t.TempDir(), "ir.json")); err == nil {
		t.Fatal("canonical")
	}
}

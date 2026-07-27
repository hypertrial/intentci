package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/parser"
)

const sample = `---
id: REQ-AUTH-001
title: Existing customers can authenticate
status: active
priority: required
owners:
  - platform-auth
depends_on: []
applies_to:
  paths:
    - src/auth/**
tags:
  - authentication
---

# Intent

Existing customers must authenticate.

# Rationale

Accounts require auth.

# Constraints

## Must

- id: CON-001
  statement: Reuse the existing session store.

## Must Not

- id: CON-002
  statement: Do not modify migrations.

# Boundaries

` + "```yaml" + `
allowed:
  - src/auth/**
forbidden:
  - migrations/**
` + "```" + `

# Obligations

` + "```yaml" + `
- id: OBL-001
  statement: Valid credentials create a session.
  required: true
  verify:
    all:
      - provider: command
        id: auth-valid-test
        run: true
        result:
          type: exit_code
          equals: 0
- id: OBL-002
  statement: No migration change.
  required: true
  verify:
    all:
      - provider: boundary
        id: no-migration
        forbidden:
          - migrations/**
` + "```" + `
`

func TestParseSample(t *testing.T) {
	req, diags := parser.Parse("req.md", []byte(sample))
	if len(diags) != 0 {
		t.Fatalf("diags=%v", diags)
	}
	if req.ID != "REQ-AUTH-001" || len(req.Obligations) != 2 {
		t.Fatalf("%+v", req)
	}
	if req.Obligations[0].Verify.All[0].Provider.Provider != "command" {
		t.Fatalf("%+v", req.Obligations[0])
	}
}

func TestParseFileAndErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(path, []byte("no front matter"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diags := parser.ParseFile(path)
	if len(diags) == 0 {
		t.Fatal("expected diags")
	}
	_, diags = parser.Parse("x.md", []byte("---\nid: bad\ntitle: t\nstatus: active\npriority: required\n---\n# Intent\n\nx\n"))
	if len(diags) == 0 {
		t.Fatal("expected invalid id / missing obligations")
	}
}

func TestRejectsInvalidStatusAndPriority(t *testing.T) {
	bad := `---
id: REQ-001
title: typo
status: activ
priority: requred
---
# Intent
x
# Obligations
` + "```yaml" + `
- id: OBL-001
  statement: x
  required: true
  verify:
    provider: command
    run: "true"
` + "```"
	_, diags := parser.Parse("bad.md", []byte(bad))
	if len(diags) != 2 {
		t.Fatalf("got diagnostics %v", diags)
	}
}

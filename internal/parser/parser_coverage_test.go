package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/parser"
)

func TestDiagnosticError(t *testing.T) {
	d := parser.Diagnostic{Message: "msg"}
	if d.Error() != "msg" {
		t.Fatal(d.Error())
	}
	d.Path = "a.md"
	if d.Error() != "a.md: msg" {
		t.Fatal(d.Error())
	}
}

func TestParseFileMissing(t *testing.T) {
	_, diags := parser.ParseFile(filepath.Join(t.TempDir(), "nope.md"))
	if len(diags) == 0 {
		t.Fatal("expected read error")
	}
}

func TestParseFrontMatterVariants(t *testing.T) {
	// CRLF front matter
	raw := "---\r\nid: REQ-1\ntitle: t\nstatus: active\npriority: required\n---\r\n# Intent\n\nx\n\n# Obligations\n\n```yaml\n- id: O\n  verify:\n    provider: command\n    id: c\n    run: \"true\"\n```\n"
	_, diags := parser.Parse("cr.md", []byte(raw))
	_ = diags

	// unterminated
	_, diags = parser.Parse("u.md", []byte("---\nid: x\n"))
	if len(diags) == 0 {
		t.Fatal("unterminated")
	}

	// bad yaml fm
	_, diags = parser.Parse("b.md", []byte("---\n:\n---\n# Intent\nx\n"))
	if len(diags) == 0 {
		t.Fatal("bad yaml")
	}

	// trailing --- without newline after
	_, _ = parser.Parse("t.md", []byte("---\nid: REQ-99\ntitle: t\nstatus: active\npriority: required\n---"))
}

func TestParseMissingFieldsAndSections(t *testing.T) {
	_, diags := parser.Parse("m.md", []byte("---\nid: BAD\n---\n# Rationale\nr\n"))
	if len(diags) < 4 {
		t.Fatalf("diags=%v", diags)
	}
}

func TestParseConstraintsBoundariesObligationsErrors(t *testing.T) {
	body := `---
id: REQ-X-1
title: t
status: active
priority: required
---

# Intent

intent

# Constraints

## Must

not: valid: yaml: [

## Must Not

- id: C2
  statement: s

# Boundaries

` + "```yaml" + `
allowed: [
` + "```" + `

# Obligations

` + "```yaml" + `
- id: ""
  verify: {}
- id: O1
  required: false
  verify:
    all: notalist
- id: O2
  verify:
    any:
      - notamap
- id: O3
  verify:
    not: []
- id: O4
  verify:
    not:
      provider: command
      run: "true"
- id: O5
  verify:
    all:
      - provider: 1
- id: O6
  verify:
    all:
      - all:
          - provider: command
            id: nested
            run: "true"
            report: r
            result: {equals: 0}
            expect: {changed: false}
            assert: {ok: true}
            allowed: [a]
            forbidden: [b]
            paths: [c]
            extra_key: 1
- id: O7
  verify:
    provider: command
    id: leaf
    run: "true"
` + "```" + `
`
	req, diags := parser.Parse("c.md", []byte(body))
	if req.ID != "REQ-X-1" {
		t.Fatal(req.ID)
	}
	if len(diags) == 0 {
		t.Fatal("expected diags")
	}
}

func TestParseVerifyBranches(t *testing.T) {
	mk := func(verify string) string {
		return `---
id: REQ-Y-1
title: t
status: active
priority: required
---

# Intent

i

# Obligations

` + "```yml" + `
- id: O
  verify:
` + indent(verify, "    ") + `
` + "```" + `
`
	}
	cases := []string{
		"all:\n  - provider: command\n    run: \"true\"\n",
		"any:\n  - provider: command\n    id: a\n    run: \"true\"\n",
		"not:\n  provider: command\n  id: n\n  run: \"true\"\n",
		"provider: command\n  id: p\n  run: \"true\"\n",
		"foo: 1\n",
	}
	for _, c := range cases {
		_, diags := parser.Parse("v.md", []byte(mk(c)))
		_ = diags
	}
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func TestParseWriteAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.md")
	body := `---
id: REQ-Z-1
title: t
status: active
priority: required
---

# Intent

ok

# Boundaries

allowed:
  - a/**
forbidden:
  - b/**

# Obligations

` + "```" + `
- id: O
  statement: s
  verify:
    all:
      - provider: command
        id: c
        run: "true"
` + "```" + `
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	req, diags := parser.ParseFile(path)
	if len(diags) != 0 {
		t.Fatalf("%v", diags)
	}
	if req.ID != "REQ-Z-1" {
		t.Fatal(req)
	}
}

func TestParseBOMAndEmptyConstraint(t *testing.T) {
	raw := "\uFEFF---\nid: REQ-BOM-1\ntitle: t\nstatus: active\npriority: required\n---\n# Intent\ni\n\n# Constraints\n\n## Must\n\n\n# Obligations\n\n```yaml\n- id: O\n  verify:\n    provider: command\n    run: \"true\"\n```\n"
	_, diags := parser.Parse("bom.md", []byte(raw))
	_ = diags
}

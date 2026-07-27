package parser_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/parser"
)

func TestUnquotedTrueRunCoerced(t *testing.T) {
	const body = `---
id: REQ-1
title: t
status: active
priority: required
---

# Intent

x

# Obligations

` + "```yaml" + `
- id: OBL-001
  statement: s
  required: true
  verify:
    all:
      - provider: command
        id: smoke
        run: true
        result:
          type: exit_code
          equals: 0
      - provider: command
        id: num
        run: 42
        report: 1.5
` + "```" + `
`
	req, diags := parser.Parse("r.md", []byte(body))
	if len(diags) != 0 {
		t.Fatalf("%v", diags)
	}
	if req.Obligations[0].Verify.All[0].Provider.Run != "true" {
		t.Fatalf("run=%q", req.Obligations[0].Verify.All[0].Provider.Run)
	}
	if req.Obligations[0].Verify.All[1].Provider.Run != "42" {
		t.Fatalf("num run=%q", req.Obligations[0].Verify.All[1].Provider.Run)
	}
}

func TestScalarStringBranches(t *testing.T) {
	// Cover int64/float/nil/default via exported? use Parse extras.
	// Direct coverage through false bool and nested list strings.
	const body = `---
id: REQ-2
title: t
status: active
priority: required
---

# Intent

x

# Obligations

` + "```yaml" + `
- id: OBL-001
  statement: s
  required: true
  verify:
    all:
      - provider: command
        id: f
        run: false
        result: {type: exit_code, equals: 0}
` + "```" + `
`
	req, diags := parser.Parse("r.md", []byte(body))
	if len(diags) != 0 {
		t.Fatalf("%v", diags)
	}
	if req.Obligations[0].Verify.All[0].Provider.Run != "false" {
		t.Fatalf("%q", req.Obligations[0].Verify.All[0].Provider.Run)
	}
}

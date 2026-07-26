package parser

import (
	"testing"

	"github.com/hypertrial/intentci/internal/ir"
)

func TestSplitFrontMatterCRLFAndMapToVerify(t *testing.T) {
	fm, body, err := splitFrontMatter("---\r\nid: x\n---\r\nbody")
	if err != nil || fm == "" {
		t.Fatalf("%q %q %v", fm, body, err)
	}
	_, _, err = splitFrontMatter("---\nid: x\n---\r\nbody")
	if err != nil {
		t.Fatal(err)
	}
	// EOF closing marker
	fm, body, err = splitFrontMatter("---\nid: x\n---")
	if err != nil || body != "" {
		t.Fatalf("%q %q %v", fm, body, err)
	}
	_, err = mapToVerify(nil)
	if err == nil {
		t.Fatal("missing verify")
	}
	_, err = mapToVerify(map[string]any{"not": map[string]any{"nope": 1}})
	if err == nil {
		t.Fatal("not child")
	}
	_, err = mapToVerify(map[string]any{"provider": 1})
	if err == nil {
		t.Fatal("provider name")
	}
	_, err = toNodeList([]any{map[string]any{"all": "bad"}})
	if err == nil {
		t.Fatal("nested")
	}
	n, err := mapToVerify(map[string]any{"provider": "command", "run": "true"})
	if err != nil || n.Provider == nil {
		t.Fatal(err)
	}
	_ = ir.VerifyNode{}
}

func TestParseMissingID(t *testing.T) {
	_, diags := Parse("x.md", []byte("---\ntitle: t\nstatus: active\npriority: required\n---\n# Intent\ni\n\n# Obligations\n\n```yaml\n- id: O\n  verify:\n    provider: command\n    run: \"true\"\n```\n"))
	found := false
	for _, d := range diags {
		if d.Message == "missing id" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%v", diags)
	}
}

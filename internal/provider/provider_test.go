package provider_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
)

func TestCommandAndBoundary(t *testing.T) {
	reg := provider.DefaultRegistry()
	p, _ := reg.Get("command")
	res := p.Execute(context.Background(), provider.Request{
		Root: t.TempDir(), Spec: ir.ProviderSpec{Provider: "command", ID: "c", Run: "true", Result: map[string]any{"equals": 0}},
		RetainStdout: true, RetainStderr: true,
	})
	if res.Status != "completed" || res.Evidence[0].Passed == nil || !*res.Evidence[0].Passed {
		t.Fatalf("%+v", res)
	}
	b, _ := reg.Get("boundary")
	res = b.Execute(context.Background(), provider.Request{
		ChangedFiles: []string{"migrations/1.sql"},
		Spec:         ir.ProviderSpec{Provider: "boundary", Forbidden: []string{"migrations/**"}},
	})
	if res.Evidence[0].Passed == nil || *res.Evidence[0].Passed {
		t.Fatalf("expected fail %+v", res)
	}
}

func TestJUnitSARIFJSONManual(t *testing.T) {
	root := t.TempDir()
	junitPath := filepath.Join(root, "out.xml")
	if err := os.WriteFile(junitPath, []byte(`<testsuite tests="1" failures="0" errors="0"><testcase name="a"/></testsuite>`), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := provider.DefaultRegistry()
	jp, _ := reg.Get("junit")
	res := jp.Execute(context.Background(), provider.Request{Root: root, Spec: ir.ProviderSpec{Provider: "junit", Report: "out.xml"}})
	if res.Evidence[0].Passed == nil || !*res.Evidence[0].Passed {
		t.Fatalf("%+v", res)
	}
	sarifPath := filepath.Join(root, "out.sarif")
	if err := os.WriteFile(sarifPath, []byte(`{"version":"2.1.0","runs":[{"results":[]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sp, _ := reg.Get("sarif")
	res = sp.Execute(context.Background(), provider.Request{Root: root, Spec: ir.ProviderSpec{Provider: "sarif", Report: "out.sarif"}})
	if !*res.Evidence[0].Passed {
		t.Fatal(res)
	}
	jsonPath := filepath.Join(root, "out.json")
	if err := os.WriteFile(jsonPath, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	jsp, _ := reg.Get("json")
	res = jsp.Execute(context.Background(), provider.Request{Root: root, Spec: ir.ProviderSpec{Provider: "json", Report: "out.json", Assert: map[string]any{"ok": true}}})
	if !*res.Evidence[0].Passed {
		t.Fatal(res)
	}
	mp, _ := reg.Get("manual")
	res = mp.Execute(context.Background(), provider.Request{Spec: ir.ProviderSpec{Provider: "manual", ID: "m"}})
	if res.Evidence[0].Class != "human" {
		t.Fatal(res)
	}
	gp, _ := reg.Get("git-diff")
	res = gp.Execute(context.Background(), provider.Request{
		ChangedFiles: []string{"a.go"}, Spec: ir.ProviderSpec{Provider: "git-diff", Paths: []string{"a.go"}, Expect: map[string]any{"changed": false}},
	})
	if *res.Evidence[0].Passed {
		t.Fatal("expected fail")
	}
}

func TestGeneratedReportsCannotReuseStalePass(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name     string
		provider string
		report   string
		content  string
	}{
		{name: "junit", provider: "junit", report: "out.xml", content: `<testsuite tests="1" failures="0"/>`},
		{name: "sarif", provider: "sarif", report: "out.sarif", content: `{"version":"2.1.0","runs":[{"results":[]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, tc.report)
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			old := time.Now().Add(-time.Hour)
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
			p, _ := provider.DefaultRegistry().Get(tc.provider)
			res := p.Execute(context.Background(), provider.Request{
				Root: root,
				Spec: ir.ProviderSpec{Provider: tc.provider, Run: "false", Report: tc.report},
			})
			if res.Status != "error" {
				t.Fatalf("stale passing report produced %q: %+v", res.Status, res)
			}
		})
	}
}

package provider_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
)

func TestValidateAllProviders(t *testing.T) {
	reg := provider.DefaultRegistry()
	checks := []struct {
		name string
		spec ir.ProviderSpec
		want int
	}{
		{"command", ir.ProviderSpec{}, 1},
		{"command", ir.ProviderSpec{Run: "true"}, 0},
		{"boundary", ir.ProviderSpec{}, 1},
		{"boundary", ir.ProviderSpec{Allowed: []string{"a"}}, 0},
		{"git-diff", ir.ProviderSpec{}, 1},
		{"git-diff", ir.ProviderSpec{Paths: []string{"a"}}, 0},
		{"json", ir.ProviderSpec{}, 1},
		{"json", ir.ProviderSpec{Report: "x"}, 0},
		{"junit", ir.ProviderSpec{}, 1},
		{"junit", ir.ProviderSpec{Report: "x"}, 0},
		{"sarif", ir.ProviderSpec{}, 1},
		{"sarif", ir.ProviderSpec{Run: "true"}, 0},
		{"manual", ir.ProviderSpec{}, 0},
	}
	for _, c := range checks {
		p, ok := reg.Get(c.name)
		if !ok {
			t.Fatalf("missing %s", c.name)
		}
		diags := p.Validate(c.spec)
		if len(diags) != c.want {
			t.Fatalf("%s validate=%v want %d", c.name, diags, c.want)
		}
	}
}

func TestBoundaryAllowedAndDedupe(t *testing.T) {
	p, _ := provider.DefaultRegistry().Get("boundary")
	res := p.Execute(context.Background(), provider.Request{
		ChangedFiles: []string{"src/a.go", "src/a.go", "other.go"},
		Spec:         ir.ProviderSpec{Allowed: []string{"src/**"}, Forbidden: []string{"migrations/**"}},
	})
	if res.Evidence[0].Passed == nil || *res.Evidence[0].Passed {
		t.Fatalf("expected fail %+v", res)
	}
	res = p.Execute(context.Background(), provider.Request{
		ChangedFiles: []string{"src/a.go"},
		Spec:         ir.ProviderSpec{Allowed: []string{"src/**"}},
	})
	if !*res.Evidence[0].Passed {
		t.Fatal(res)
	}
}

func TestCommandTimeoutExitAndExpect(t *testing.T) {
	p := &provider.CommandProvider{}
	res := p.Execute(context.Background(), provider.Request{
		Root: t.TempDir(), Timeout: 5 * time.Millisecond,
		Spec: ir.ProviderSpec{ID: "t", Run: "sleep 2", Result: map[string]any{"equals": 0}},
	})
	if res.Status != "error" {
		t.Fatalf("%+v", res)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: t.TempDir(),
		Spec: ir.ProviderSpec{Run: "false", Result: map[string]any{"equals": float64(1)}},
	})
	if res.Evidence[0].Passed == nil || !*res.Evidence[0].Passed {
		t.Fatalf("%+v", res)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: t.TempDir(),
		Spec: ir.ProviderSpec{Run: "false", Result: map[string]any{"equals": 0}},
	})
	if *res.Evidence[0].Passed {
		t.Fatal("expected fail")
	}
	// non-exit error via custom Exec
	p.Exec = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "definitely-not-a-real-binary-xyz")
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: t.TempDir(), Spec: ir.ProviderSpec{ID: "e", Run: "x"},
	})
	if res.Status != "error" {
		t.Fatalf("%+v", res)
	}
}

func TestGitDiffExpectChanged(t *testing.T) {
	p, _ := provider.DefaultRegistry().Get("git-diff")
	res := p.Execute(context.Background(), provider.Request{
		ChangedFiles: []string{"a.go"},
		Spec:         ir.ProviderSpec{Paths: []string{"a.go"}, Expect: map[string]any{"changed": true}},
	})
	if !*res.Evidence[0].Passed {
		t.Fatal(res)
	}
	res = p.Execute(context.Background(), provider.Request{
		ChangedFiles: nil,
		Spec:         ir.ProviderSpec{Forbidden: []string{"a.go"}, Expect: map[string]any{"changed": true}},
	})
	if *res.Evidence[0].Passed {
		t.Fatal("expected fail")
	}
	res = p.Execute(context.Background(), provider.Request{
		ChangedFiles: nil,
		Spec:         ir.ProviderSpec{Paths: []string{"a.go"}},
	})
	if !*res.Evidence[0].Passed {
		t.Fatal(res)
	}
}

func TestJSONErrorsAndLookup(t *testing.T) {
	p, _ := provider.DefaultRegistry().Get("json")
	root := t.TempDir()
	res := p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: "missing.json", ID: "j"},
	})
	if res.Status != "error" {
		t.Fatal(res)
	}
	path := filepath.Join(root, "bad.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: "bad.json"},
	})
	if res.Status != "error" {
		t.Fatal(res)
	}
	if err := os.WriteFile(path, []byte(`[1,2]`), 0o644); err != nil {
		t.Fatal(err)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: path, Assert: map[string]any{"ok": true}},
	})
	if res.Evidence[0].Passed == nil || *res.Evidence[0].Passed {
		t.Fatalf("lookup nil should fail assert %+v", res)
	}
	if err := os.WriteFile(path, []byte(`{"ok":false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: "bad.json", Assert: map[string]any{"ok": true}},
	})
	if *res.Evidence[0].Passed {
		t.Fatal("assert fail")
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: "bad.json"},
	})
	if !*res.Evidence[0].Passed {
		t.Fatal(res)
	}
}

func TestJUnitBranches(t *testing.T) {
	p := &provider.JUnitProvider{}
	root := t.TempDir()
	res := p.Execute(context.Background(), provider.Request{Root: root})
	if res.Status != "error" {
		t.Fatalf("empty report should error: %+v", res)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Run: "true"},
	})
	if res.Status != "error" {
		t.Fatalf("report required %+v", res)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: "missing.xml"},
	})
	if res.Status != "error" {
		t.Fatal(res)
	}
	path := filepath.Join(root, "bad.xml")
	if err := os.WriteFile(path, []byte("<not"), 0o644); err != nil {
		t.Fatal(err)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: "bad.xml"},
	})
	if res.Status != "error" {
		t.Fatal(res)
	}
	// testsuites with cases when tests=0
	suites := `<testsuites><testsuite tests="0" failures="0" errors="0">
<testcase name="a"/><testcase name="b"><failure message="f"/></testcase>
</testsuite></testsuites>`
	if err := os.WriteFile(path, []byte(suites), 0o644); err != nil {
		t.Fatal(err)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{ID: "j", Report: "bad.xml"}, RetainStdout: true,
	})
	if res.Evidence[0].Passed == nil || *res.Evidence[0].Passed {
		t.Fatalf("%+v", res)
	}
	// single suite with cases when tests=0
	suite := `<testsuite tests="0" failures="0" errors="0">
<testcase name="a"><error message="e"/></testcase>
</testsuite>`
	if err := os.WriteFile(path, []byte(suite), 0o644); err != nil {
		t.Fatal(err)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: path},
	})
	if *res.Evidence[0].Passed {
		t.Fatal(res)
	}
	// timeout path
	p.Exec = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "sleep 2")
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Timeout: 5 * time.Millisecond,
		Spec: ir.ProviderSpec{ID: "t", Run: "sleep", Report: "bad.xml"},
	})
	if res.Status != "error" {
		t.Fatalf("%+v", res)
	}

	// A fresh passing report cannot override a nonzero generator exit.
	res = (&provider.JUnitProvider{}).Execute(context.Background(), provider.Request{
		Root: root,
		Spec: ir.ProviderSpec{
			ID: "fresh", Report: "fresh.xml",
			Run: `printf '<testsuite tests="1" failures="0"/>' > fresh.xml; exit 1`,
		},
		RetainStdout: true,
	})
	if res.Status != "error" {
		t.Fatalf("%+v", res)
	}

	// A fresh failing report remains a failure even when the generator exits nonzero.
	res = (&provider.JUnitProvider{}).Execute(context.Background(), provider.Request{
		Root: root,
		Spec: ir.ProviderSpec{
			ID: "fresh-fail", Report: "fresh-fail.xml",
			Run: `printf '<testsuite tests="1" failures="1"/>' > fresh-fail.xml; exit 1`,
		},
	})
	if res.Status != "completed" || *res.Evidence[0].Passed {
		t.Fatalf("%+v", res)
	}

	if err := os.Mkdir(filepath.Join(root, "report-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "report-dir", "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	res = (&provider.JUnitProvider{}).Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Run: "true", Report: "report-dir"},
	})
	if res.Status != "error" {
		t.Fatalf("%+v", res)
	}
}

func TestSARIFBranches(t *testing.T) {
	p := &provider.SARIFProvider{}
	root := t.TempDir()
	res := p.Execute(context.Background(), provider.Request{Root: root})
	if res.Status != "error" {
		t.Fatalf("empty report should error: %+v", res)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Run: "true"}, RetainStdout: true,
	})
	if res.Status != "error" {
		t.Fatal(res)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: "missing.sarif"},
	})
	if res.Status != "error" {
		t.Fatal(res)
	}
	path := filepath.Join(root, "bad.sarif")
	if err := os.WriteFile(path, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Report: "bad.sarif"},
	})
	if res.Status != "error" {
		t.Fatal(res)
	}
	if err := os.WriteFile(path, []byte(`{"runs":[{"results":[{},{}]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{ID: "s", Report: path}, RetainStdout: true,
	})
	if *res.Evidence[0].Passed {
		t.Fatal("expected findings fail")
	}

	res = (&provider.SARIFProvider{}).Execute(context.Background(), provider.Request{
		Root: root,
		Spec: ir.ProviderSpec{
			ID: "fresh", Report: "fresh.sarif",
			Run: `printf '{"runs":[{"results":[]}]}' > fresh.sarif; exit 1`,
		},
		RetainStdout: true,
	})
	if res.Status != "error" {
		t.Fatalf("%+v", res)
	}

	res = (&provider.SARIFProvider{}).Execute(context.Background(), provider.Request{
		Root: root,
		Spec: ir.ProviderSpec{
			ID: "fresh-fail", Report: "fresh-fail.sarif",
			Run: `printf '{"runs":[{"results":[{}]}]}' > fresh-fail.sarif; exit 1`,
		},
	})
	if res.Status != "completed" || *res.Evidence[0].Passed {
		t.Fatalf("%+v", res)
	}

	if err := os.Mkdir(filepath.Join(root, "sarif-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sarif-dir", "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	res = (&provider.SARIFProvider{}).Execute(context.Background(), provider.Request{
		Root: root, Spec: ir.ProviderSpec{Run: "true", Report: "sarif-dir"},
	})
	if res.Status != "error" {
		t.Fatalf("%+v", res)
	}

	p.Exec = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "sleep 2")
	}
	res = p.Execute(context.Background(), provider.Request{
		Root: root, Timeout: 5 * time.Millisecond,
		Spec: ir.ProviderSpec{ID: "timeout", Run: "sleep", Report: "timeout.sarif"},
	})
	if res.Status != "error" {
		t.Fatalf("%+v", res)
	}
}

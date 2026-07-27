package provider_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
)

func TestExternalProviderProtocol(t *testing.T) {
	root := t.TempDir()
	writeExecutable := func(name, body string) string {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	valid := writeExecutable("valid", `cat >/dev/null
echo diagnostic >&2
printf '%s' '{"protocol_version":"1.9","provider":"custom","provider_version":"2.3.4","status":"completed","evidence":[{"id":"e","class":"deterministic","summary":"ok","passed":true}],"future":"ignored"}'`)
	external := &provider.ExternalProvider{ProviderName: "custom", Path: valid}
	if external.Name() != "custom" || external.Version() != "external" || len(external.Validate(ir.ProviderSpec{})) != 0 {
		t.Fatal("external metadata")
	}
	result := external.Execute(context.Background(), provider.Request{
		Root: root, Timeout: 10 * time.Second, RetainStdout: true, RetainStderr: true,
		Spec: ir.ProviderSpec{WorkingDirectory: ".", Configuration: map[string]any{"x": true}},
	})
	if result.Status != "completed" || result.ProviderVersion != "2.3.4" ||
		len(result.Evidence) != 1 || !strings.Contains(result.Stderr, "diagnostic") ||
		len(result.Diagnostics) != 1 {
		t.Fatalf("%+v", result)
	}

	cases := []struct {
		name string
		body string
	}{
		{"malformed", `cat >/dev/null; echo nope`},
		{"major", `cat >/dev/null; echo '{"protocol_version":"2.0","provider_version":"1","status":"completed"}'`},
		{"version", `cat >/dev/null; echo '{"protocol_version":"1.0","status":"completed"}'`},
		{"status", `cat >/dev/null; echo '{"protocol_version":"1.0","provider_version":"1","status":"bogus"}'`},
		{"exit", `cat >/dev/null; echo failed >&2; exit 3`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			path := writeExecutable(testCase.name, testCase.body)
			got := (&provider.ExternalProvider{ProviderName: testCase.name, Path: path}).Execute(
				context.Background(),
				provider.Request{Root: root, Timeout: 10 * time.Second, Spec: ir.ProviderSpec{WorkingDirectory: "."}},
			)
			if got.Status != "error" || len(got.Diagnostics) == 0 {
				t.Fatalf("%+v", got)
			}
		})
	}

	slow := writeExecutable("slow", `sleep 2`)
	timed := (&provider.ExternalProvider{ProviderName: "slow", Path: slow}).Execute(
		context.Background(),
		provider.Request{Root: root, Timeout: time.Millisecond, Spec: ir.ProviderSpec{WorkingDirectory: "."}},
	)
	if timed.Status != "error" {
		t.Fatal(timed)
	}
	unsafe := external.Execute(context.Background(), provider.Request{
		Root: root, Timeout: 10 * time.Second, Spec: ir.ProviderSpec{WorkingDirectory: "../outside"},
	})
	if !unsafe.SecurityViolation {
		t.Fatal(unsafe)
	}
}

func TestDynamicRegistryAndStableEnvironmentFingerprint(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "intentci-provider-demo")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	registry := provider.DefaultRegistry()
	if _, ok := registry.Get("demo"); !ok {
		t.Fatal("dynamic provider not resolved")
	}
	if _, ok := registry.Get("../demo"); ok {
		t.Fatal("unsafe provider name resolved")
	}
	first := provider.Request{
		RunID: "one", AttemptID: "a", Spec: ir.ProviderSpec{Environment: map[string]string{"FIXED": "yes"}},
	}
	second := first
	second.RunID = "two"
	second.AttemptID = "b"
	if provider.EnvironmentFingerprint(first) != provider.EnvironmentFingerprint(second) {
		t.Fatal("run-specific variables contaminated the cache fingerprint")
	}
	second.Spec.Environment = map[string]string{"FIXED": "no"}
	if provider.EnvironmentFingerprint(first) == provider.EnvironmentFingerprint(second) {
		t.Fatal("explicit environment change did not invalidate fingerprint")
	}
}

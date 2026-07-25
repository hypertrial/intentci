package explain_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/explain"
	"github.com/hypertrial/intentci/internal/verify"
)

func TestExplain_SemanticSkippedNone(t *testing.T) {
	dir := gitRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `version: 1
product: {name: x, purpose: y}
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    verification: {checks: [go-test]}
checks:
  - id: go-test
    command: "true"
`
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(verify.LastResultPath(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
  "schema_version":1,"run_id":"1","status":"pass","base_commit":"a","head_commit":"b","contract_hash":"c",
  "semantic":{"enabled":true,"skipped":"disabled","finding_count":0},
  "requirements":[{"id":"BUILD-001","status":"pass","severity":"blocking","findings":[],"evidence":[],"affected_by":[],"checks":[]}],
  "checks":[],"summary":{"pass":1,"fail":0,"unverified":0,"unknown":0,"waived":0}
}`)
	if err := os.WriteFile(verify.LastResultPath(dir), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := explain.Run(explain.Options{Root: dir, ID: "BUILD-001", Base: "HEAD", Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "skipped: disabled") {
		t.Fatalf("%s", buf.String())
	}

	// single-line evidence
	raw2 := []byte(`{
  "schema_version":1,"run_id":"1","status":"pass","base_commit":"a","head_commit":"b","contract_hash":"c",
  "requirements":[{"id":"BUILD-001","status":"pass","severity":"blocking",
    "findings":[],"evidence":[{"type":"semantic","path":"a.go","line_start":3,"summary":"x"}],
    "affected_by":[],"checks":[]}],
  "checks":[],"summary":{"pass":1,"fail":0,"unverified":0,"unknown":0,"waived":0}
}`)
	if err := os.WriteFile(verify.LastResultPath(dir), raw2, 0o644); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := explain.Run(explain.Options{Root: dir, ID: "BUILD-001", Base: "HEAD", Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "a.go:3") {
		t.Fatalf("%s", buf.String())
	}
}

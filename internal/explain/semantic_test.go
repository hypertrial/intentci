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

func TestExplain_SemanticFindings(t *testing.T) {
	dir := gitRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	contractBody := `version: 1
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
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(contractBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(verify.LastResultPath(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
  "schema_version":1,"run_id":"1","status":"unverified","base_commit":"a","head_commit":"b","contract_hash":"c",
  "semantic":{"enabled":true,"provider":"local","enforcement":"advisory","finding_count":1},
  "requirements":[{
    "id":"BUILD-001","status":"unverified","severity":"blocking",
    "findings":[{"type":"semantic_contradiction","summary":"state advanced early"}],
    "evidence":[{"type":"semantic","path":"a.go","line_start":1,"line_end":2,"summary":"x"}],
    "affected_by":[],"checks":[]
  }],
  "checks":[],
  "summary":{"pass":0,"fail":0,"unverified":1,"unknown":0,"waived":0}
}`)
	if err := os.WriteFile(verify.LastResultPath(dir), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := explain.Run(explain.Options{Root: dir, ID: "BUILD-001", Base: "HEAD", Out: &buf}); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, "semantic_contradiction") || !strings.Contains(s, "a.go:1-2") {
		t.Fatalf("%s", s)
	}
}

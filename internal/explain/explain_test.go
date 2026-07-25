package explain_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/changespec"
	"github.com/hypertrial/intentci/internal/explain"
	"github.com/hypertrial/intentci/internal/verify"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "git", "-c", "core.hooksPath=/dev/null", "init")
	runGit(t, dir, "git", "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "git", "add", ".")
	runGit(t, dir, "git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestExplain_RequirementAndAC(t *testing.T) {
	dir := gitRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".intentci", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	contractBody := `version: 1
product:
  name: x
  purpose: y
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    sources:
      - path: README
        description: d
    applies_to:
      include: ["**"]
    verification:
      checks: [go-test]
checks:
  - id: go-test
    command: "true"
    timeout: 1m
`
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(contractBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := changespec.Create(dir, "DEMO-1"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(verify.LastResultPath(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"schema_version":1,"run_id":"1","status":"pass","base_commit":"a","head_commit":"b","contract_hash":"c","requirements":[{"id":"BUILD-001","status":"pass","severity":"blocking","reason":"r","findings":[{"type":"x","summary":"ok"}],"affected_by":[],"checks":[],"evidence":[]}],"checks":[],"summary":{"pass":1,"fail":0,"unverified":0,"unknown":0,"waived":0}}`)
	if err := os.WriteFile(verify.LastResultPath(dir), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := explain.Run(explain.Options{Root: dir, ID: "BUILD-001", Base: "HEAD", Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("BUILD-001")) {
		t.Fatalf("%s", buf.String())
	}
	buf.Reset()
	if err := explain.Run(explain.Options{Root: dir, ID: "AC-001", ChangeID: "DEMO-1", Base: "HEAD", Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if err := explain.Run(explain.Options{Root: dir, ID: "AC-001", Out: &buf}); err == nil {
		t.Fatal("AC without change")
	}
	if err := explain.Run(explain.Options{Root: dir, ID: "NOPE", Base: "HEAD", Out: &buf}); err == nil {
		t.Fatal("unknown req")
	}
}

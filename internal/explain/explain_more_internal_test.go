package explain

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/changespec"
)

func TestRun_NilOutAndErrors(t *testing.T) {
	dir := t.TempDir()
	if err := Run(Options{Root: dir, ID: "X"}); err == nil {
		t.Fatal("missing contract")
	}
	os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755)
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte("version: 1\n"), 0o644)
	if err := Run(Options{Root: dir, ID: "X"}); err == nil {
		t.Fatal("invalid contract")
	}

	dir2 := t.TempDir()
	os.MkdirAll(filepath.Join(dir2, ".intentci", "changes"), 0o755)
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
    timeout: 1m
`
	os.WriteFile(filepath.Join(dir2, ".intentci", "contract.yaml"), []byte(body), 0o644)
	changespec.Create(dir2, "DEMO-1")
	change := `version: 1
id: DEMO-1
status: approved
type: feature
summary: s
goals: [g]
acceptance:
  - id: AC-001
    statement: s
    severity: blocking
    verification: {checks: [go-test, missing-check]}
`
	os.WriteFile(filepath.Join(dir2, ".intentci", "changes", "DEMO-1.yaml"), []byte(change), 0o644)
	var buf bytes.Buffer
	if err := Run(Options{Root: dir2, ID: "AC-001", ChangeID: "DEMO-1", Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("(missing)")) {
		t.Fatal(buf.String())
	}
	_ = Run(Options{Root: dir2, ID: "BUILD-001"})
}

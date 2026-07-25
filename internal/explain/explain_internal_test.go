package explain

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/changespec"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/verify"
)

func TestExplain_AllBranches(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".intentci", "changes"), 0o755)
	contractBody := `version: 1
product: {name: x, purpose: y}
policy: {default_base: HEAD}
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    applies_to: {include: ["**"], exclude: ["skip/**"]}
    verification: {checks: [go-test]}
checks:
  - id: go-test
    command: "true"
    timeout: 1m
`
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(contractBody), 0o644)
	changespec.Create(dir, "DEMO-1")

	c, _, _ := contract.LoadFromRoot(dir)
	var buf bytes.Buffer
	if err := explainRequirement(Options{Root: dir, ID: "BUILD-001", Out: &buf}, c, &git.State{}, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("(none)")) {
		t.Fatalf("empty sources: %s", buf.String())
	}
	c.Requirements[0].Verification.Checks = []string{"go-test", "missing-check"}
	buf.Reset()
	if err := explainRequirement(Options{Root: dir, ID: "BUILD-001", Out: &buf}, c, &git.State{}, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("(missing)")) {
		t.Fatalf("missing check line: %s", buf.String())
	}

	buf.Reset()
	if err := Run(Options{Root: dir, ID: "BUILD-001", Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("(none)")) {
		t.Fatalf("empty sources/changed: %s", buf.String())
	}

	os.Remove(verify.LastResultPath(dir))
	buf.Reset()
	if err := Run(Options{Root: dir, ID: "BUILD-001", Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Recent local result:")) {
		t.Fatalf("no last result section: %s", buf.String())
	}

	raw := []byte(`{"schema_version":1,"run_id":"1","status":"pass","requirements":[{"id":"OTHER","status":"pass"}]}`)
	os.MkdirAll(filepath.Dir(verify.LastResultPath(dir)), 0o755)
	os.WriteFile(verify.LastResultPath(dir), raw, 0o644)
	buf.Reset()
	if err := Run(Options{Root: dir, ID: "BUILD-001", Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("not present in last result")) {
		t.Fatalf("missing id in last: %s", buf.String())
	}

	lastRaw := []byte(`{"schema_version":1,"run_id":"1","status":"pass","requirements":[{"id":"BUILD-001","status":"pass","reason":"ok","findings":[{"summary":"f"}]}]}`)
	os.WriteFile(verify.LastResultPath(dir), lastRaw, 0o644)
	buf.Reset()
	if err := Run(Options{Root: dir, ID: "BUILD-001", Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("reason: ok")) {
		t.Fatalf("recent result: %s", buf.String())
	}

	buf.Reset()
	if err := Run(Options{Root: dir, ID: "AC-001", ChangeID: "DEMO-1", Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if err := Run(Options{Root: dir, ID: "AC-999", ChangeID: "DEMO-1", Out: &buf}); err == nil {
		t.Fatal("unknown AC")
	}
	if err := Run(Options{Root: dir, ID: "AC-001", Out: &buf}); err == nil {
		t.Fatal("AC without change")
	}
	// Contract requirement ids may also use AC-*; prefer the contract when present.
	c.Requirements = append(c.Requirements, contract.Requirement{
		ID: "AC-002", Type: "behavior", Title: "t", Statement: "s",
		Status: "approved", Severity: "blocking",
		Verification: contract.Verification{Checks: []string{"go-test"}},
	})
	contractWithAC := `version: 1
product: {name: x, purpose: y}
policy: {default_base: HEAD}
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    verification: {checks: [go-test]}
  - id: AC-002
    type: behavior
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
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(contractWithAC), 0o644)
	buf.Reset()
	if err := Run(Options{Root: dir, ID: "AC-002", Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Requirement AC-002")) {
		t.Fatalf("expected contract requirement explain: %s", buf.String())
	}
	if err := Run(Options{Root: dir, ID: "NOPE", Out: &buf}); err == nil {
		t.Fatal("unknown req")
	}
}

func TestWriteRecent_NilLast(t *testing.T) {
	var buf bytes.Buffer
	writeRecent(&buf, nil, "X")
	if !bytes.Contains(buf.Bytes(), []byte("(none)")) {
		t.Fatal(buf.String())
	}
}

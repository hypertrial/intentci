package changespec_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/changespec"
	"github.com/hypertrial/intentci/internal/contract"
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

func validContract() *contract.Contract {
	return &contract.Contract{
		Version: 1,
		Product: contract.Product{Name: "x", Purpose: "y"},
		Requirements: []contract.Requirement{{
			ID: "BUILD-001", Type: "reliability", Title: "t", Statement: "s",
			Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"go-test"}},
		}},
		Checks: []contract.Check{{ID: "go-test", Command: "true", Timeout: "1m"}},
	}
}

func TestCreateLoadValidateDiff(t *testing.T) {
	dir := gitRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a contract so Create scaffolds the first known check id.
	body := []byte(`version: 1
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
    verification:
      checks: [go-test]
checks:
  - id: go-test
    command: "true"
    timeout: 1m
`)
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := changespec.Create(dir, "DEMO-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := changespec.Create(dir, "DEMO-1"); err == nil {
		t.Fatal("duplicate create")
	}
	if _, err := changespec.Create(dir, ""); err == nil {
		t.Fatal("empty id")
	}
	spec, raw, err := changespec.Load(dir, "DEMO-1")
	if err != nil {
		t.Fatal(err)
	}
	_ = changespec.Hash(raw)
	c := validContract()
	if err := changespec.Validate(spec, c); err != nil {
		t.Fatal(err)
	}
	spec.Status = "approved"
	spec.Acceptance[0].Verification.Checks = []string{"missing"}
	if err := changespec.Validate(spec, c); err == nil {
		t.Fatal("expected unknown check")
	}
	spec.Acceptance[0].Verification.Checks = []string{"go-test"}
	spec.RequiredChecks = []string{"missing"}
	if err := changespec.Validate(spec, c); err == nil {
		t.Fatal("expected required check error")
	}
	spec.RequiredChecks = nil
	spec.AffectedRequirements = []string{"NOPE-1"}
	if err := changespec.Validate(spec, c); err == nil {
		t.Fatal("expected affected req error")
	}
	c.Requirements = append(c.Requirements, contract.Requirement{
		ID: "DRAFT-001", Type: "behavior", Title: "t", Statement: "s",
		Status: "draft", Severity: "blocking",
		Verification: contract.Verification{Checks: []string{"go-test"}},
	})
	spec.AffectedRequirements = []string{"DRAFT-001"}
	if err := changespec.Validate(spec, c); err == nil {
		t.Fatal("expected non-approved affected req error")
	}
	spec.AffectedRequirements = []string{"BUILD-001"}
	spec.Acceptance = append(spec.Acceptance, spec.Acceptance[0])
	if err := changespec.Validate(spec, c); err == nil {
		t.Fatal("duplicate AC")
	}

	findings := changespec.DiffApproved("DEMO-1", nil, false, &changespec.Spec{Status: "draft"}, nil)
	if findings != nil {
		t.Fatal("draft without approved base should not diff")
	}
	findings = changespec.DiffApproved("DEMO-1", nil, false, &changespec.Spec{Status: "approved"}, []byte("a"))
	if len(findings) != 1 || findings[0].Type != "change_spec_added" {
		t.Fatalf("%+v", findings)
	}
	findings = changespec.DiffApproved("DEMO-1", []byte("a"), true, &changespec.Spec{Status: "approved"}, []byte("a"))
	if findings != nil {
		t.Fatal("equal")
	}
	findings = changespec.DiffApproved("DEMO-1", []byte("status: approved\n"), true, &changespec.Spec{Status: "approved"}, []byte("status: approved\nx\n"))
	if len(findings) != 1 || findings[0].Type != "change_spec_modified" {
		t.Fatalf("%+v", findings)
	}
	findings = changespec.DiffApproved("DEMO-1", []byte("status: draft\n"), true, &changespec.Spec{Status: "approved"}, []byte("status: approved\n"))
	if len(findings) != 1 || findings[0].Type != "change_spec_approved" {
		t.Fatalf("%+v", findings)
	}
	findings = changespec.DiffApproved("DEMO-1", []byte("status: approved\n"), true, &changespec.Spec{Status: "draft"}, []byte("status: draft\n"))
	if len(findings) != 1 || findings[0].Type != "change_spec_modified" {
		t.Fatalf("demotion: %+v", findings)
	}
	findings = changespec.DiffApproved("DEMO-1", []byte("status: draft\n"), true, &changespec.Spec{Status: "draft"}, []byte("status: draft\nx\n"))
	if findings != nil {
		t.Fatal("draft-to-draft should not emit findings")
	}
	_ = path
	if !strings.HasSuffix(changespec.Path(dir, "DEMO-1"), "DEMO-1.yaml") {
		t.Fatal(changespec.Path(dir, "DEMO-1"))
	}
}

func TestLoadBase(t *testing.T) {
	dir := gitRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".intentci", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "version: 1\nid: X-1\nstatus: draft\ntype: feature\nsummary: s\ngoals:\n  - g\nacceptance:\n  - id: AC-001\n    statement: s\n    severity: blocking\n    verification:\n      checks: [go-test]\n"
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "changes", "X-1.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "git", "add", ".")
	runGit(t, dir, "git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "change")
	data, ok, err := changespec.LoadBase(dir, "HEAD", "X-1")
	if err != nil || !ok || len(data) == 0 {
		t.Fatalf("%v %v %s", ok, err, data)
	}
	_, ok, err = changespec.LoadBase(dir, "HEAD", "NOPE")
	if err != nil || ok {
		t.Fatalf("missing: %v %v", ok, err)
	}
}

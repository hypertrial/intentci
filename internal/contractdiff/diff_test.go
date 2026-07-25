package contractdiff_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/contractdiff"
)

func baseContract() *contract.Contract {
	return &contract.Contract{
		Version: 1,
		Product: contract.Product{Name: "x", Purpose: "y"},
		Requirements: []contract.Requirement{{
			ID: "BUILD-001", Type: "reliability", Title: "t", Statement: "s",
			Status: "approved", Severity: "blocking",
			AppliesTo:     contract.AppliesTo{Include: []string{"**"}},
			Verification: contract.Verification{Mode: "all", Checks: []string{"go-test"}, Semantic: "required"},
		}},
		Checks: []contract.Check{{ID: "go-test", Command: "true", Timeout: "1m"}},
	}
}

func TestDiff_Weakenings(t *testing.T) {
	base := baseContract()
	head := baseContract()
	head.Requirements[0].Status = "draft"
	head.Requirements[0].Severity = "advisory"
	head.Requirements[0].Verification.Mode = "any"
	head.Requirements[0].Verification.Checks = nil
	head.Requirements[0].Verification.Semantic = "off"
	head.Requirements[0].AppliesTo.Include = []string{"cmd/**"}
	head.Checks[0].Command = "false"
	changes := contractdiff.Diff(base, head)
	if len(changes) == 0 {
		t.Fatal("expected weakenings")
	}
	types := map[string]bool{}
	for _, c := range changes {
		types[c.Type] = true
	}
	for _, want := range []string{
		"requirement_demoted_draft", "severity_lowered", "mode_narrowed",
		"check_removed", "applies_to_narrowed", "check_modified", "semantic_disabled",
	} {
		if !types[want] {
			t.Fatalf("missing %s in %+v", want, changes)
		}
	}
}

func TestDiff_RemovedRequirement(t *testing.T) {
	base := baseContract()
	head := baseContract()
	head.Requirements = nil
	head.Checks = nil
	changes := contractdiff.Diff(base, head)
	found := false
	for _, c := range changes {
		if c.Type == "requirement_removed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", changes)
	}
}

func TestDiff_Deprecated(t *testing.T) {
	base := baseContract()
	head := baseContract()
	head.Requirements[0].Status = "deprecated"
	changes := contractdiff.Diff(base, head)
	found := false
	for _, c := range changes {
		if c.Type == "requirement_deprecated" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", changes)
	}
}

func TestDiff_Nil(t *testing.T) {
	if contractdiff.Diff(nil, baseContract()) != nil {
		t.Fatal("nil base")
	}
	if contractdiff.Diff(baseContract(), nil) != nil {
		t.Fatal("nil head")
	}
}

func TestEffective_RetainsBase(t *testing.T) {
	base := baseContract()
	head := baseContract()
	head.Requirements[0].Status = "draft"
	head.Requirements[0].Verification.Checks = []string{}
	eff := contractdiff.Effective(base, head)
	if len(eff.ApprovedBlocking()) != 1 {
		t.Fatalf("%+v", eff.Requirements)
	}
	if eff.ApprovedBlocking()[0].Status != "approved" {
		t.Fatal("expected base definition retained")
	}
	if contractdiff.Effective(nil, head) != head {
		t.Fatal("nil base")
	}
	if contractdiff.Effective(base, nil) != base {
		t.Fatal("nil head")
	}
}

func TestLoadBase(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run("git", "-c", "core.hooksPath=/dev/null", "init")
	run("git", "checkout", "-b", "main")
	if err := os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`version: 1
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
`)
	if err := os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "init")
	head, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	c, raw, ok, err := contractdiff.LoadBase(dir, string(bytesTrim(head)))
	if err != nil || !ok || c == nil || len(raw) == 0 {
		t.Fatalf("%v ok=%v c=%v", err, ok, c)
	}
	_, _, ok, err = contractdiff.LoadBase(dir, "")
	if err != nil || ok {
		t.Fatal("empty merge base")
	}
	_, _, ok, _ = contractdiff.LoadBase(dir, "deadbeef")
	if ok {
		t.Fatal("missing path")
	}
}

func bytesTrim(b []byte) string {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return string(b)
}

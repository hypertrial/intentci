package semantic_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/changespec"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/internal/semantic"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestBuildRequest(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nsecret=supersecretvalue\n"), 0o644)
	t.Setenv("SECRET_KEY", "supersecretvalue")

	c := &contract.Contract{
		Product: contract.Product{Name: "p", Purpose: "why", NonGoals: []string{"ng"}},
		Environment: contract.Environment{Include: []string{"SECRET_KEY"}},
		Requirements: []contract.Requirement{{
			ID: "R-1", Status: "approved", Severity: "blocking", Title: "t", Statement: "s",
			Sources:      []contract.Source{{Path: "docs/x.md"}},
			Verification: contract.Verification{Checks: []string{"c"}, Semantic: "required"},
		}, {
			ID: "R-2", Status: "draft", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"c"}, Semantic: "optional"},
		}, {
			ID: "R-3", Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"c"}, Semantic: "off"},
		}},
	}
	req, err := semantic.BuildRequest(semantic.BuildOptions{
		Root:         dir,
		Profile:      "full",
		BaseCommit:   "abc",
		HeadCommit:   "def",
		ChangedFiles: []string{"a.go"},
		Contract:     c,
		Change:       &changespec.Spec{ID: "CHG-1", Goals: []string{"g"}, NonGoals: []string{"ng2"}},
		Selection: impact.Selection{Requirements: []impact.SelectedRequirement{
			{Requirement: c.Requirements[0]},
			{Requirement: c.Requirements[1]},
			{Requirement: c.Requirements[2]},
		}},
		CheckResults: map[string]runner.Result{"c": {Status: protocol.CheckPass}},
		EnvInclude:   []string{"SECRET_KEY"},
		DiffFn:       func(root, mergeBase string) (string, error) { return "diff supersecretvalue", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Change == nil || req.Change.ID != "CHG-1" || len(req.Change.Goals) != 1 {
		t.Fatalf("change %+v", req.Change)
	}
	if len(req.Requirements) != 1 || req.Requirements[0].ID != "R-1" {
		t.Fatalf("reqs %+v", req.Requirements)
	}
	if req.Diff != "diff [REDACTED]" {
		t.Fatalf("diff %q", req.Diff)
	}
	if len(req.Snippets) != 1 || req.Snippets[0].Content == "" {
		t.Fatalf("snippets %+v", req.Snippets)
	}
	if len(req.CheckResults) != 1 {
		t.Fatal("checks")
	}
}

func TestBuildRequest_NilContract(t *testing.T) {
	if _, err := semantic.BuildRequest(semantic.BuildOptions{}); err == nil {
		t.Fatal("expected error")
	}
}

package attest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/attest"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestBuildWrite(t *testing.T) {
	zero := 0
	res := &protocol.Result{
		Status:       protocol.StatusPass,
		BaseCommit:   "abc",
		HeadCommit:   "def1234",
		ContractHash: "sha256:x",
		ChangeSpec:   &protocol.ChangeSpecRef{ID: "C-1", Hash: "sha256:y"},
		Checks: []protocol.CheckResult{{
			ID: "go-test", Status: protocol.CheckPass, ExitCode: &zero, DurationMS: 1,
		}},
	}
	checks := map[string]contract.Check{
		"go-test": {ID: "go-test", Command: "true", Timeout: "1m"},
	}
	att, err := attest.Build(res, checks, []string{"PATH"})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path, err := attest.Write(dir, att)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".intentci", "tmp", "attestation-def1234.json")
	if path != want {
		t.Fatalf("%s != %s", path, want)
	}
}

func TestBuild_RejectsNonPass(t *testing.T) {
	if _, err := attest.Build(&protocol.Result{Status: protocol.StatusFail}, nil, nil); err == nil {
		t.Fatal("expected error")
	}
	if _, err := attest.Build(nil, nil, nil); err == nil {
		t.Fatal("nil")
	}
	if _, err := attest.Write(t.TempDir(), nil); err == nil {
		t.Fatal("nil write")
	}
	if _, err := attest.Write(t.TempDir(), &attest.Attestation{Status: protocol.StatusFail, HeadCommit: "x"}); err == nil {
		t.Fatal("non-pass write")
	}
}

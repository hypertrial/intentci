package scheduler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestApplyJUnit(t *testing.T) {
	dir := t.TempDir()
	ch := contract.Check{ID: "j", Results: &contract.Results{Format: "junit", Path: "out.xml"}}
	res := runner.Result{Status: protocol.CheckPass}
	got := applyJUnit(dir, ch, res)
	if got.Status != protocol.CheckUnknown {
		t.Fatalf("missing file: %+v", got)
	}
	failXML := `<testsuite failures="1" errors="0"><testcase name="t"><failure message="x"/></testcase></testsuite>`
	if err := os.WriteFile(filepath.Join(dir, "out.xml"), []byte(failXML), 0o644); err != nil {
		t.Fatal(err)
	}
	got = applyJUnit(dir, ch, runner.Result{Status: protocol.CheckPass})
	if got.Status != protocol.CheckFail {
		t.Fatalf("%+v", got)
	}
	got = applyJUnit(dir, ch, runner.Result{Status: protocol.CheckFail, Reason: "exit"})
	if got.Status != protocol.CheckFail {
		t.Fatalf("%+v", got)
	}
	// corrupt after fail exit keeps fail
	if err := os.WriteFile(filepath.Join(dir, "out.xml"), []byte("<bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = applyJUnit(dir, ch, runner.Result{Status: protocol.CheckFail, Reason: "exit"})
	if got.Status != protocol.CheckFail {
		t.Fatalf("%+v", got)
	}
	got = applyJUnit(dir, ch, runner.Result{Status: protocol.CheckPass})
	if got.Status != protocol.CheckUnknown {
		t.Fatalf("%+v", got)
	}
	emptyPath := contract.Check{ID: "j", Results: &contract.Results{Format: "junit", Path: ""}}
	got = applyJUnit(dir, emptyPath, runner.Result{Status: protocol.CheckPass})
	if got.Status != protocol.CheckUnknown {
		t.Fatalf("%+v", got)
	}
	got = applyJUnit(dir, contract.Check{ID: "x"}, runner.Result{Status: protocol.CheckPass})
	if got.Status != protocol.CheckPass {
		t.Fatalf("%+v", got)
	}
	okXML := `<testsuite failures="0" errors="0"><testcase name="t"/></testsuite>`
	if err := os.WriteFile(filepath.Join(dir, "out.xml"), []byte(okXML), 0o644); err != nil {
		t.Fatal(err)
	}
	got = applyJUnit(dir, ch, runner.Result{Status: protocol.CheckPass})
	if got.Status != protocol.CheckPass {
		t.Fatalf("%+v", got)
	}
}

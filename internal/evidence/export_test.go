package evidence

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestHelpers(t *testing.T) {
	st, reason := resolveStatus("any", 0, 0, 0, 0, 0, 1)
	if st != protocol.ReqUnverified || reason == "" {
		t.Fatal(st, reason)
	}
	st, _ = resolveStatus("any", 1, 0, 0, 0, 0, 2)
	if st != protocol.ReqPass {
		t.Fatal(st)
	}
	st, _ = resolveStatus("all", 0, 0, 0, 0, 0, 0)
	if st != protocol.ReqUnverified {
		t.Fatal(st)
	}
	findings := []protocol.Finding{{Type: "completion_condition", Summary: "x"}}
	out := appendCompletion(findings, contract.Requirement{ID: "A-1", Statement: "s"})
	if len(out) != 1 {
		t.Fatal(len(out))
	}
	if firstLine() != "see check output" {
		t.Fatal(firstLine())
	}
	if firstLine("", "  hi\nthere") != "hi" {
		t.Fatal(firstLine("", "  hi\nthere"))
	}
	if firstLine("only") != "only" {
		t.Fatal()
	}
}

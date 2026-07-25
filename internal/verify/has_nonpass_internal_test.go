package verify

import (
	"testing"

	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestHasNonPassChecks(t *testing.T) {
	if hasNonPassChecks([]protocol.CheckResult{{Status: protocol.CheckPass}}) {
		t.Fatal("pass")
	}
	if !hasNonPassChecks([]protocol.CheckResult{{Status: protocol.CheckFail}}) {
		t.Fatal("fail")
	}
	if hasNonPassChecks([]protocol.CheckResult{{Status: protocol.CheckSkipped}, {Status: protocol.CheckCached}}) {
		t.Fatal("skipped/cached ok")
	}
}

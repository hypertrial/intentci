package attest_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/attest"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestBuild_NoChangeSpec(t *testing.T) {
	res := &protocol.Result{
		Status:     protocol.StatusPass,
		HeadCommit: "h",
		BaseCommit: "b",
		Checks: []protocol.CheckResult{
			{ID: "b", Status: protocol.CheckPass},
			{ID: "a", Status: protocol.CheckPass},
		},
	}
	att, err := attest.Build(res, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if att.ChangeSpecHash != "" {
		t.Fatal(att.ChangeSpecHash)
	}
	if att.Checks[0].ID != "a" || att.Checks[1].ID != "b" {
		t.Fatalf("unsorted %+v", att.Checks)
	}
}

package protocol_test

import (
	"testing"

	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestConstants(t *testing.T) {
	if protocol.SchemaVersion != 1 {
		t.Fatal(protocol.SchemaVersion)
	}
	_ = protocol.StatusPass
	_ = protocol.ReqFail
	_ = protocol.CheckCached
	_ = protocol.Result{}
	_ = protocol.ChangeFinding{}
	_ = protocol.Summary{}
}

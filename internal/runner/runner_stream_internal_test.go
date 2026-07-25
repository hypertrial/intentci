package runner

import (
	"bytes"
	"context"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestRun_StreamsToStdout(t *testing.T) {
	var buf bytes.Buffer
	res := Run(context.Background(), contract.Check{ID: "x", Command: "echo hi", Timeout: "2s"}, Options{
		Dir: t.TempDir(), Stdout: &buf, Stderr: &buf,
	})
	if res.Status != protocol.CheckPass || buf.Len() == 0 {
		t.Fatalf("%+v buf=%q", res, buf.String())
	}
}

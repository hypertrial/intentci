package scheduler

import (
	"bytes"
	"context"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestRun_PrefixFlushPartialLine(t *testing.T) {
	var out bytes.Buffer
	checks := map[string]contract.Check{
		"a": {ID: "a", Command: "printf partial", Timeout: "5s"},
	}
	_ = Run(context.Background(), checks, []string{"a"}, Options{
		Dir: t.TempDir(), Stdout: &out,
	})
	if out.Len() == 0 {
		t.Fatal("expected prefixed output")
	}
}

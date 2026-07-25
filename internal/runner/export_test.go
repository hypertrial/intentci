package runner

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestExitCodeAndDirError(t *testing.T) {
	if exitCode(errors.New("x")) != 1 {
		t.Fatal()
	}
	if exitCode(&exec.ExitError{}) == 99999 {
		t.Fatal()
	}
	res := Run(context.Background(), contract.Check{ID: "x", Command: "true", Timeout: "1s"}, Options{Dir: "/dev/null/not-a-dir"})
	if res.Status != protocol.CheckFail && res.Status != protocol.CheckUnknown {
		t.Fatalf("%+v", res)
	}
}

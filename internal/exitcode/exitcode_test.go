package exitcode_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/exitcode"
)

func TestConstants(t *testing.T) {
	if exitcode.Pass != 0 || exitcode.Fail != 1 || exitcode.SecurityBoundary != 10 {
		t.Fatal("unexpected exit codes")
	}
}

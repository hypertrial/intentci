package attest

import (
	"errors"
	"os"
	"testing"

	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestWrite_IOErrors(t *testing.T) {
	oldMk := mkdirAll
	oldW := writeFile
	oldM := marshalIndent
	defer func() {
		mkdirAll = oldMk
		writeFile = oldW
		marshalIndent = oldM
	}()
	att := &Attestation{Status: protocol.StatusPass, HeadCommit: "abc"}
	mkdirAll = func(string, os.FileMode) error { return errors.New("mkdir") }
	if _, err := Write(t.TempDir(), att); err == nil {
		t.Fatal("mkdir")
	}
	mkdirAll = oldMk
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("marshal") }
	if _, err := Write(t.TempDir(), att); err == nil {
		t.Fatal("marshal")
	}
	marshalIndent = oldM
	writeFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	if _, err := Write(t.TempDir(), att); err == nil {
		t.Fatal("write")
	}
}

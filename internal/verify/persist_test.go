package verify

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestPersistErrors(t *testing.T) {
	oldMk := mkdirAll
	oldW := writeFile
	oldM := marshalIndent
	defer func() {
		mkdirAll = oldMk
		writeFile = oldW
		marshalIndent = oldM
	}()

	mkdirAll = func(string, os.FileMode) error { return errors.New("mkdir") }
	if err := persistLastResult(t.TempDir(), &protocol.Result{}); err == nil {
		t.Fatal("mkdir")
	}
	mkdirAll = os.MkdirAll

	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("marshal") }
	if err := persistLastResult(t.TempDir(), &protocol.Result{}); err == nil {
		t.Fatal("marshal")
	}
	marshalIndent = json.MarshalIndent

	writeFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	if err := persistLastResult(t.TempDir(), &protocol.Result{}); err == nil {
		t.Fatal("write")
	}
}

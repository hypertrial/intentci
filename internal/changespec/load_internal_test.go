package changespec

import (
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestToJSONMap_NilSpec(t *testing.T) {
	m := ToJSONMap(nil)
	if m == nil || len(m) != 0 {
		t.Fatalf("%#v", m)
	}
}

func validContractForTest() *contract.Contract {
	return &contract.Contract{
		Version: 1,
		Product: contract.Product{Name: "x", Purpose: "y"},
		Checks:  []contract.Check{{ID: "go-test", Command: "true", Timeout: "1m"}},
	}
}

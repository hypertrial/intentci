package report

import (
	"testing"

	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestValidateResultSchemaBadSchema(t *testing.T) {
	old := resultSchemaJSON
	resultSchemaJSON = func() []byte { return []byte("{") }
	defer func() { resultSchemaJSON = old }()
	if err := ValidateResultSchema(&protocol.Result{SchemaVersion: 1, RunID: "x", Status: "pass"}); err == nil {
		t.Fatal("expected schema parse error")
	}
}

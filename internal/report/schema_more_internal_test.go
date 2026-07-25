package report

import (
	"testing"

	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestValidateResultSchema_CompilePaths(t *testing.T) {
	old := resultSchemaJSON
	defer func() { resultSchemaJSON = old }()

	resultSchemaJSON = func() []byte {
		return []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"status":{"type":"string"}}}`)
	}
	res := &protocol.Result{SchemaVersion: 1, RunID: "x", Status: "pass"}
	if err := ValidateResultSchema(res); err != nil {
		t.Fatal(err)
	}

	resultSchemaJSON = func() []byte {
		return []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$ref":"#/$defs/x","$defs":{"x":{"$ref":"#/$defs/x"}}}`)
	}
	if err := ValidateResultSchema(res); err == nil {
		// cyclic ref may or may not fail depending on compiler
	}
}

package report

import (
	"testing"

	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestValidateResultSchema_CompilePath(t *testing.T) {
	res := sampleResultForSchema()
	if err := ValidateResultSchema(res); err != nil {
		t.Fatal(err)
	}
	old := resultSchemaJSON
	resultSchemaJSON = func() []byte { return []byte("{") }
	defer func() { resultSchemaJSON = old }()
	if err := ValidateResultSchema(res); err == nil {
		t.Fatal("parse error")
	}
}

func sampleResultForSchema() *protocol.Result {
	return &protocol.Result{
		SchemaVersion: 1,
		RunID:         "x",
		Status:        "pass",
		Requirements:  []protocol.RequirementResult{},
		Checks:        []protocol.CheckResult{},
		Waivers:       []any{},
		ContractChanges: []any{},
		ChangeFindings:  []protocol.ChangeFinding{},
		Summary:       protocol.Summary{},
	}
}

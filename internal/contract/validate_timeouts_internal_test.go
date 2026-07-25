package contract

import "testing"

func TestValidateTimeouts_EmptyContinueDirect(t *testing.T) {
	ve := &ValidationError{}
	validateTimeouts(&Contract{Checks: []Check{{ID: "c", Timeout: ""}}}, ve)
	if !ve.empty() {
		t.Fatal(ve.Error())
	}
}

func TestCompileSchema_AddResourceAndCompile(t *testing.T) {
	if _, err := compileSchema(schemaJSON(), "https://intentci.dev/schemas/contract-v1.json"); err != nil {
		t.Fatal(err)
	}
}

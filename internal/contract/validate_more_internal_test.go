package contract

import (
	"strings"
	"testing"
)

func TestValidateGlobs_CheckInputsAndExclude(t *testing.T) {
	c := validForGlobTest()
	c.Requirements[0].AppliesTo.Exclude = []string{"[invalid"}
	if err := Validate(c); err == nil || !strings.Contains(err.Error(), "exclude") {
		t.Fatalf("exclude glob: %v", err)
	}
	c = validMinimal()
	ve := &ValidationError{}
	validateGlobs(c, ve)
	if ve.empty() {
		t.Fatal("expected empty glob errors")
	}
	c = validForGlobTest()
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
}

func validForGlobTest() *Contract {
	c := validMinimal()
	c.Checks[0].Inputs = []string{"**"}
	c.Checks[0].Timeout = "1m"
	c.Requirements[0].AppliesTo.Exclude = nil
	return c
}

func TestValidateTimeouts_Invalid(t *testing.T) {
	c := validMinimal()
	c.Checks[0].Timeout = "bad"
	if err := Validate(c); err == nil {
		t.Fatal("bad timeout")
	}
}

func TestCompileSchema_CompileError(t *testing.T) {
	raw := []byte(`not-json`)
	if _, err := compileSchema(raw, "mem://x"); err == nil {
		t.Fatal("parse error")
	}
	old := schemaJSON
	schemaJSON = func() []byte { return []byte("{") }
	defer func() { schemaJSON = old }()
	if err := validateSchema(validMinimal()); err == nil {
		t.Fatal("bad embedded schema")
	}
}

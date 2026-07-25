package changespec

import "testing"

func TestCompileSchemaErrors(t *testing.T) {
	if _, err := compileSchema([]byte("{"), "mem://x"); err == nil {
		t.Fatal()
	}
	old := schemaJSON
	schemaJSON = func() []byte { return []byte("{") }
	defer func() { schemaJSON = old }()
	if err := validateSchema(&Spec{}); err == nil {
		t.Fatal()
	}
}

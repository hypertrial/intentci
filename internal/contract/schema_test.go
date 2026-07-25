package contract

import "testing"

func TestCompileSchemaErrors(t *testing.T) {
	if _, err := compileSchema([]byte("not-json"), "mem://x"); err == nil {
		t.Fatal("expected parse error")
	}
	old := schemaJSON
	schemaJSON = func() []byte { return []byte(`{"type":"object"}`) }
	defer func() { schemaJSON = old }()
	c := &Contract{Version: 1, Product: Product{Name: "n", Purpose: "p"}}
	// missing required fields -> schema validate error path through Validate
	_ = validateSchema(c)
	schemaJSON = func() []byte { return []byte("{") }
	if err := validateSchema(c); err == nil {
		t.Fatal("bad schema")
	}
}

func TestToJSONMapNilSafe(t *testing.T) {
	m := ToJSONMap(nil)
	if m == nil {
		t.Fatal()
	}
}

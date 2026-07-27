package schema

import (
	"errors"
	"testing"
)

func TestSchemaLookupAndValidation(t *testing.T) {
	for _, name := range []string{"requirement", "evidence", "verdict", "repair", "ir", "report", "plan"} {
		if raw, ok := JSON(name); !ok || len(raw) == 0 {
			t.Fatalf("%s: ok=%t len=%d", name, ok, len(raw))
		}
	}
	if _, ok := JSON("missing"); ok {
		t.Fatal("unknown schema found")
	}
	if err := Validate("verdict", "pass"); err != nil {
		t.Fatal(err)
	}
	if err := Validate("verdict", "invalid"); err == nil {
		t.Fatal("invalid verdict passed")
	}
	if err := Validate("missing", "pass"); err == nil {
		t.Fatal("unknown schema passed")
	}

	old := marshalJSON
	defer func() { marshalJSON = old }()
	marshalJSON = func(any) ([]byte, error) { return nil, errors.New("marshal") }
	if err := Validate("verdict", "pass"); err == nil {
		t.Fatal("marshal error ignored")
	}
	if schemaURL("missing") != "" {
		t.Fatal("unknown schema URL")
	}
}

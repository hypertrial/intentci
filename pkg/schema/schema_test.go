package schema_test

import (
	"bytes"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/hypertrial/intentci/pkg/schema"
)

func TestEmbeddedSchemasCompile(t *testing.T) {
	for name, raw := range map[string][]byte{
		"contract":    schema.ContractJSON,
		"result":      schema.ResultJSON,
		"changespec":  schema.ChangeSpecJSON,
	} {
		c := jsonschema.NewCompiler()
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		url := "mem://" + name
		if err := c.AddResource(url, doc); err != nil {
			t.Fatalf("%s add: %v", name, err)
		}
		if _, err := c.Compile(url); err != nil {
			t.Fatalf("%s compile: %v", name, err)
		}
	}
}

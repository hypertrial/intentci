package changespec

import (
	"errors"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestCompileSchema_InjectedErrors(t *testing.T) {
	oldAdd := addSchemaResource
	oldCompile := compileSchemaURL
	defer func() {
		addSchemaResource = oldAdd
		compileSchemaURL = oldCompile
	}()
	addSchemaResource = func(*jsonschema.Compiler, string, any) error { return errors.New("add") }
	if _, err := compileSchema(schemaJSON(), "mem://x"); err == nil {
		t.Fatal("add")
	}
	addSchemaResource = oldAdd
	compileSchemaURL = func(*jsonschema.Compiler, string) (*jsonschema.Schema, error) {
		return nil, errors.New("compile")
	}
	if _, err := compileSchema(schemaJSON(), "mem://x"); err == nil {
		t.Fatal("compile")
	}
}

package report

import (
	"errors"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestValidateResultSchema_ResourceAndCompileErrors(t *testing.T) {
	oldAdd := addSchemaResource
	oldCompile := compileSchemaURL
	defer func() {
		addSchemaResource = oldAdd
		compileSchemaURL = oldCompile
	}()
	addSchemaResource = func(*jsonschema.Compiler, string, any) error { return errors.New("add") }
	if err := ValidateResultSchema(sampleResultForSchema()); err == nil {
		t.Fatal("add")
	}
	addSchemaResource = oldAdd
	compileSchemaURL = func(*jsonschema.Compiler, string) (*jsonschema.Schema, error) {
		return nil, errors.New("compile")
	}
	if err := ValidateResultSchema(sampleResultForSchema()); err == nil {
		t.Fatal("compile")
	}
}

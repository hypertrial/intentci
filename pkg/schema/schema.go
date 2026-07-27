package schema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed requirement.schema.json
var RequirementJSON []byte

//go:embed evidence.schema.json
var EvidenceJSON []byte

//go:embed verdict.schema.json
var VerdictJSON []byte

//go:embed repair.schema.json
var RepairJSON []byte

//go:embed ir.schema.json
var IRJSON []byte

//go:embed report.schema.json
var ReportJSON []byte

//go:embed plan.schema.json
var PlanJSON []byte

var documents = map[string][]byte{
	"requirement": RequirementJSON,
	"evidence":    EvidenceJSON,
	"verdict":     VerdictJSON,
	"repair":      RepairJSON,
	"ir":          IRJSON,
	"report":      ReportJSON,
	"plan":        PlanJSON,
}

var compiled = compileSchemas()

// JSON returns the embedded schema with the given public name.
func JSON(name string) ([]byte, bool) {
	raw, ok := documents[name]
	return raw, ok
}

// Validate checks a Go value against one of the embedded v1 schemas.
func Validate(name string, value any) error {
	schema, ok := compiled[name]
	if !ok {
		return fmt.Errorf("unknown schema %q", name)
	}
	encoded, err := marshalJSON(value)
	if err != nil {
		return err
	}
	var generic any
	_ = json.Unmarshal(encoded, &generic)
	if err := schema.Validate(generic); err != nil {
		return fmt.Errorf("%s schema validation: %w", name, err)
	}
	return nil
}

func compileSchemas() map[string]*jsonschema.Schema {
	compiler := jsonschema.NewCompiler()
	for name, raw := range documents {
		document, _ := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		_ = compiler.AddResource(schemaURL(name), document)
	}
	output := make(map[string]*jsonschema.Schema, len(documents))
	for name := range documents {
		output[name] = compiler.MustCompile(schemaURL(name))
	}
	return output
}

var marshalJSON = json.Marshal

func schemaURL(name string) string {
	switch name {
	case "requirement":
		return "https://intentci.dev/schemas/requirement-v1.json"
	case "evidence":
		return "https://intentci.dev/schemas/evidence-v1.json"
	case "verdict":
		return "https://intentci.dev/schemas/verdict-v1.json"
	case "repair":
		return "https://intentci.dev/schemas/repair-packet-v1.json"
	case "ir":
		return "https://intentci.dev/schemas/ir-v1.json"
	case "report":
		return "https://intentci.dev/schemas/report-v1.json"
	case "plan":
		return "https://intentci.dev/schemas/verification-plan-v1.json"
	default:
		return ""
	}
}

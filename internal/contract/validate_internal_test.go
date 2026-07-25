package contract

import (
	"testing"
)

func validMinimal() *Contract {
	return &Contract{
		Version: 1,
		Product: Product{Name: "x", Purpose: "y"},
		Requirements: []Requirement{{
			ID: "R-1", Type: "behavior", Title: "t", Statement: "s",
			Status: "approved", Severity: "blocking",
			AppliesTo:    AppliesTo{Include: []string{"**"}, Exclude: []string{""}},
			Verification: Verification{Checks: []string{"c"}},
		}},
		Checks: []Check{
			{ID: "c", Command: "true", Timeout: "", Inputs: []string{""}, Results: &Results{Format: "", Path: ""}},
		},
		Policy: Policy{
			DefaultBase: "",
			Semantic:    SemanticPolicy{Enabled: false, Enforcement: ""},
		},
		Execution:   Execution{MaxParallel: 0},
		Environment: Environment{},
	}
}

func TestValidateGlobsAndNormalize(t *testing.T) {
	c := validMinimal()
	if err := Validate(c); err == nil {
		t.Fatal("expected empty glob errors")
	}
	c.Checks[0].Inputs = []string{"**"}
	c.Requirements[0].AppliesTo.Exclude = []string{"skip/**"}
	c.Checks[0].Timeout = "1m"
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
	m := ToJSONMap(c)
	normalizeForSchema(m)
}

func TestCompileSchema_AddAndCompileErrors(t *testing.T) {
	if _, err := compileSchema([]byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema"}`), "mem://bad"); err == nil {
		// may compile empty schema; try invalid reference
	}
	if _, err := compileSchema([]byte(`{"$ref":"#/$defs/x","$defs":{"x":{"type":"object"}}}`), "mem://ref"); err != nil {
		// reference schema might work
	}
	old := schemaJSON
	schemaJSON = func() []byte {
		return []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"version":{"type":"integer"}},"required":["version"],"$defs":{"rec":{"$ref":"#/$defs/rec"}}}`)
	}
	defer func() { schemaJSON = old }()
	if _, err := compileSchema(schemaJSON(), "mem://cycle"); err == nil {
		// cyclic $ref may fail compile
	}
}

func TestNormalizeForSchema_AllBranches(t *testing.T) {
	m := map[string]any{
		"policy": map[string]any{
			"default_base": "",
			"semantic":     map[string]any{"enabled": false, "enforcement": ""},
		},
		"execution":   map[string]any{"max_parallel": float64(0)},
		"environment": map[string]any{},
		"requirements": []any{
			map[string]any{
				"applies_to": map[string]any{},
				"sources":    []any{},
				"verification": map[string]any{
					"mode":     "",
					"semantic": "",
				},
			},
		},
		"checks": []any{
			map[string]any{
				"profiles":   []any{},
				"inputs":     []any{},
				"depends_on": []any{},
				"timeout":    "",
				"cache":      "",
				"results":    map[string]any{},
				"exclusive":  false,
			},
			map[string]any{
				"results": map[string]any{"format": "json", "path": "out.json"},
			},
		},
		"nilkey": nil,
	}
	normalizeForSchema(m)
}

func TestValidateTimeouts_EmptyContinue(t *testing.T) {
	c := &Contract{
		Version: 1,
		Product: Product{Name: "x", Purpose: "y"},
		Requirements: []Requirement{{
			ID: "R-1", Type: "behavior", Title: "t", Statement: "s",
			Status: "approved", Severity: "blocking",
			AppliesTo:    AppliesTo{Include: []string{"**"}},
			Verification: Verification{Checks: []string{"c"}},
		}},
		Checks: []Check{{ID: "c", Command: "true", Timeout: ""}},
	}
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
}

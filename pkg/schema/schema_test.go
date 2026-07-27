package schema_test

import (
	"testing"

	"github.com/hypertrial/intentci/pkg/schema"
)

func TestEmbeddedSchemasNonEmpty(t *testing.T) {
	for name, b := range map[string][]byte{
		"requirement": schema.RequirementJSON,
		"evidence":    schema.EvidenceJSON,
		"verdict":     schema.VerdictJSON,
		"repair":      schema.RepairJSON,
		"ir":          schema.IRJSON,
		"plan":        schema.PlanJSON,
		"report":      schema.ReportJSON,
	} {
		if len(b) < 10 {
			t.Fatalf("%s schema empty", name)
		}
	}
}

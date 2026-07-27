package compiler

import (
	"reflect"
	"testing"

	"github.com/hypertrial/intentci/internal/ir"
)

func FuzzV1DependencyGraphs(f *testing.F) {
	f.Add([]byte{0, 1, 2}, false)
	f.Add([]byte{1, 0}, true)
	f.Fuzz(func(t *testing.T, edges []byte, selfCycle bool) {
		if len(edges) > 64 {
			t.Skip()
		}
		count := len(edges)%8 + 1
		document := &ir.Document{SchemaVersion: ir.SchemaVersion}
		for index := 0; index < count; index++ {
			requirement := ir.Requirement{ID: "R" + string(rune('A'+index))}
			if len(edges) > 0 {
				target := int(edges[index%len(edges)]) % count
				if target != index {
					requirement.DependsOn = []string{"R" + string(rune('A'+target))}
				}
			}
			document.Requirements = append(document.Requirements, requirement)
		}
		if selfCycle {
			document.Requirements[0].DependsOn = []string{document.Requirements[0].ID}
		}
		first := validateGraph(document)
		second := validateGraph(document)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("dependency diagnostics are nondeterministic:\n%+v\n%+v", first, second)
		}
		if selfCycle && len(first) == 0 {
			t.Fatal("self dependency was not rejected")
		}
	})
}

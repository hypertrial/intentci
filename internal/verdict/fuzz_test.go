package verdict

import (
	"slices"
	"testing"
)

func FuzzV1VerdictAggregation(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6})
	f.Add([]byte{6, 0})
	f.Fuzz(func(t *testing.T, encoded []byte) {
		if len(encoded) > 128 {
			t.Skip()
		}
		values := []string{Pass, Skipped, Unproven, Uncertain, ReviewRequired, Error, Fail}
		requirements := make([]RequirementResult, 0, len(encoded))
		for _, value := range encoded {
			requirements = append(requirements, RequirementResult{
				ID: "R", Priority: "required", Verdict: values[int(value)%len(values)],
			})
		}
		forward := AggregateRun(requirements).Verdict
		slices.Reverse(requirements)
		reverse := AggregateRun(requirements).Verdict
		if forward != reverse {
			t.Fatalf("aggregation changed with ordering: %s != %s", forward, reverse)
		}
	})
}

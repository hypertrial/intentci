package contract

import "testing"

func TestValidateTimeouts_InvalidDirect(t *testing.T) {
	ve := &ValidationError{}
	validateTimeouts(&Contract{Checks: []Check{{ID: "c", Timeout: "bad"}}}, ve)
	if ve.empty() {
		t.Fatal("expected timeout error")
	}
}

func TestNormalizeForSchema_KeepResults(t *testing.T) {
	m := map[string]any{
		"checks": []any{
			map[string]any{"results": map[string]any{"format": "json", "path": "out"}},
		},
	}
	normalizeForSchema(m)
	checks := m["checks"].([]any)
	cm := checks[0].(map[string]any)
	if _, ok := cm["results"]; !ok {
		t.Fatal("results should remain")
	}
}

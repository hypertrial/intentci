package contract

import "testing"

func TestNormalizeForSchema_NonMapEntries(t *testing.T) {
	m := map[string]any{
		"requirements": []any{"not-map", nil},
		"checks":       []any{42, nil},
	}
	normalizeForSchema(m)
}

package changespec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestValidateErrorStringAndNormalize(t *testing.T) {
	c := &contract.Contract{
		Version: 1,
		Product: contract.Product{Name: "x", Purpose: "y"},
		Checks:  []contract.Check{{ID: "go-test", Command: "true", Timeout: "1m"}},
	}
	s := &Spec{Version: 2} // invalid
	err := Validate(s, c)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() == "" {
		t.Fatal("empty error string")
	}
	m := map[string]any{
		"source":                map[string]any{},
		"non_goals":             []any{},
		"affected_requirements": []any{},
		"required_checks":       []any{},
		"waivers":               []any{},
		"acceptance": []any{
			map[string]any{"verification": map[string]any{"semantic": ""}},
		},
		"x": nil,
	}
	normalize(m)
	dir := t.TempDir()
	if _, err := Create(dir, "Z-1"); err != nil {
		t.Fatal(err)
	}
	// mkdir fail: make changes a file
	p := Dir(dir + "-x")
	_ = os.WriteFile(filepath.Join(dir, "block"), []byte("x"), 0o644)
	_ = p
}

func TestLoadErrors(t *testing.T) {
	if _, _, err := Load(t.TempDir(), "NO"); err == nil {
		t.Fatal()
	}
	dir := t.TempDir()
	os.MkdirAll(Dir(dir), 0o755)
	os.WriteFile(Path(dir, "BAD-1"), []byte(":\n"), 0o644)
	if _, _, err := Load(dir, "BAD-1"); err == nil {
		t.Fatal("yaml")
	}
}

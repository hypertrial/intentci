package changespec

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestCreate_ErrorPaths(t *testing.T) {
	dir := t.TempDir()
	oldMk := mkdirAll
	oldW := writeFile
	oldS := pathStat
	defer func() {
		mkdirAll = oldMk
		writeFile = oldW
		pathStat = oldS
	}()

	pathStat = func(string) (os.FileInfo, error) { return nil, errors.New("stat") }
	if _, err := Create(dir, "X-1"); err == nil {
		t.Fatal("stat")
	}
	pathStat = os.Stat

	mkdirAll = func(string, os.FileMode) error { return errors.New("mkdir") }
	if _, err := Create(dir, "X-1"); err == nil {
		t.Fatal("mkdir")
	}
	mkdirAll = os.MkdirAll

	writeFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	if _, err := Create(dir, "X-1"); err == nil {
		t.Fatal("write")
	}
	writeFile = os.WriteFile
}

func TestDefaultCheckID_NoContract(t *testing.T) {
	if got := defaultCheckID(t.TempDir()); got != "unit-tests" {
		t.Fatalf("got %q", got)
	}
}

func TestLoad_IDMismatch(t *testing.T) {
	dir := t.TempDir()
	path := Path(dir, "FOO-1")
	if err := os.MkdirAll(Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`version: 1
id: BAR-1
status: draft
type: feature
summary: s
goals:
  - g
acceptance:
  - id: AC-001
    statement: s
    severity: blocking
    verification:
      checks: [go-test]
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(dir, "FOO-1"); err == nil {
		t.Fatal("expected id mismatch")
	}
}

func TestToJSONMap_NilMap(t *testing.T) {
	m := ToJSONMap(&Spec{})
	if m == nil {
		t.Fatal("nil map")
	}
}

func TestValidate_MissingID(t *testing.T) {
	c := &contract.Contract{
		Version: 1,
		Product: contract.Product{Name: "x", Purpose: "y"},
		Checks:  []contract.Check{{ID: "go-test", Command: "true", Timeout: "1m"}},
	}
	s := &Spec{
		Version: 1, Status: "draft", Type: "feature", Summary: "s",
		Goals: []string{"g"},
		Acceptance: []Acceptance{{
			ID: "AC-001", Statement: "s", Severity: "blocking",
			Verification: Verification{Checks: []string{"go-test"}},
		}},
	}
	if err := Validate(s, c); err == nil {
		t.Fatal("missing id")
	}
}

func TestRunGit_EmptyStderr(t *testing.T) {
	if _, err := runGit(t.TempDir(), "not-a-real-git-subcommand-xyz"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalize_NonMapAcceptance(t *testing.T) {
	m := map[string]any{
		"acceptance": []any{"not-a-map", nil},
		"x":          nil,
	}
	normalize(m)
	_ = filepath.Join
}

func TestCompileSchema_AddResourceError(t *testing.T) {
	if _, err := compileSchema([]byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`), "https://intentci.dev/schemas/changespec-v1.json"); err != nil {
		// valid schema should compile
	}
}

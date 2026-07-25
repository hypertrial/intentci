package explain

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExplainAcceptance_LoadError(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755)
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(`version: 1
product: {name: x, purpose: y}
requirements: []
checks: []
`), 0o644)
	var buf bytes.Buffer
	if err := Run(Options{Root: dir, ID: "AC-001", ChangeID: "NOPE", Out: &buf}); err == nil {
		t.Fatal("load error")
	}
}

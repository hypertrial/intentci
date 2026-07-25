package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectDraftChecks_Individual(t *testing.T) {
	for _, tc := range []struct {
		file string
		want string
	}{
		{"pytest.ini", "pytest"},
		{"setup.py", "pytest"},
		{"Cargo.toml", "cargo-test"},
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, tc.file), []byte("x"), 0o644)
		out := detectDraftChecks(dir)
		found := false
		for _, d := range out {
			if d.ID == tc.want {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: %#v", tc.file, out)
		}
	}
}

func TestFileExists(t *testing.T) {
	if fileExists(t.TempDir(), "missing") {
		t.Fatal("missing should be false")
	}
}

func TestRenderContract_MultipleChecks(t *testing.T) {
	body := renderContract("demo", []draftCheck{
		{ID: "a", Description: "A", Command: "true", Inputs: []string{"**"}},
		{ID: "b", Description: "B", Command: "true", Inputs: []string{"**"}},
	})
	if !strings.Contains(body, "id: b") {
		t.Fatal(body)
	}
}

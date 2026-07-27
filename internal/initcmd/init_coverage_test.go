package initcmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/initcmd"
)

func TestExampleLanguagesAndErrors(t *testing.T) {
	for _, lang := range []string{"", "go", "python", "typescript", "ts", "rust", "other"} {
		root := t.TempDir()
		if err := initcmd.Run(initcmd.Options{Root: root, Language: lang}); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(root, ".intentci", "requirements", "REQ-001.md"))
		if err != nil {
			t.Fatal(err)
		}
		s := string(body)
		switch lang {
		case "go":
			if !strings.Contains(s, "go test") {
				t.Fatal(s)
			}
		case "python":
			if !strings.Contains(s, "pytest") {
				t.Fatal(s)
			}
		case "typescript", "ts":
			if !strings.Contains(s, "npm test") {
				t.Fatal(s)
			}
		case "rust":
			if !strings.Contains(s, "cargo test") {
				t.Fatal(s)
			}
		default:
			if !strings.Contains(s, "intentci-ok") {
				t.Fatal(s)
			}
		}
	}

	// mkdir fail for requirements: parent path is a file
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".intentci"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := initcmd.Run(initcmd.Options{Root: root}); err == nil {
		t.Fatal("expected error")
	}

	// CI github mkdir fail
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".github"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := initcmd.Run(initcmd.Options{Root: root, CIGithub: true, NoExample: true}); err == nil {
		t.Fatal("expected ci mkdir error")
	}
}

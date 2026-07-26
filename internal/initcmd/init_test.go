package initcmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/initcmd"
)

func TestInit(t *testing.T) {
	root := t.TempDir()
	if err := initcmd.Run(initcmd.Options{Root: root, Language: "go", CIGithub: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".intentci", "config.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".intentci", "requirements", "REQ-001.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "workflows", "intentci.yml")); err != nil {
		t.Fatal(err)
	}
	if err := initcmd.Run(initcmd.Options{Root: root}); err == nil {
		t.Fatal("expected exists error")
	}
	if err := initcmd.Run(initcmd.Options{Root: root, Force: true, NoExample: true}); err != nil {
		t.Fatal(err)
	}
}

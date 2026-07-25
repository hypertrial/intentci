package initcmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun_ExistingGitignore(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".intentci", "changes"), 0o755)
	os.WriteFile(filepath.Join(dir, ".intentci", ".gitignore"), []byte("keep\n"), 0o644)
	res, err := Run(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range res.Created {
		if filepath.Base(p) == ".gitignore" {
			t.Fatalf("should not recreate gitignore: %+v", res.Created)
		}
	}
}

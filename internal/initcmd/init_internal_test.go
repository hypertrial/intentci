package initcmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_WriteFailuresAndGitignoreExists(t *testing.T) {
	dir := t.TempDir()
	oldMk := mkdirAll
	oldW := writeFile
	oldS := fileStat
	oldA := absPath
	defer func() {
		mkdirAll = oldMk
		writeFile = oldW
		fileStat = oldS
		absPath = oldA
	}()

	mkdirAll = func(string, os.FileMode) error { return errors.New("mkdir") }
	if _, err := Run(dir); err == nil {
		t.Fatal("mkdir")
	}
	mkdirAll = os.MkdirAll

	absPath = func(string) (string, error) { return "", errors.New("abs") }
	if _, err := Run(dir); err == nil {
		t.Fatal("abs")
	}
	absPath = filepath.Abs

	dir2 := t.TempDir()
	res, err := Run(dir2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) == 0 {
		t.Fatal("expected created")
	}
	if _, err := Run(dir2); err == nil {
		t.Fatal("contract exists")
	}

	dir3 := t.TempDir()
	res, err = Run(dir3)
	if err != nil {
		t.Fatal(err)
	}
	writeFile = func(string, []byte, os.FileMode) error { return errors.New("write contract") }
	if _, err := Run(t.TempDir()); err == nil {
		t.Fatal("write contract")
	}
	writeFile = os.WriteFile

	dir4 := t.TempDir()
	os.MkdirAll(filepath.Join(dir4, ".intentci", "changes"), 0o755)
	os.WriteFile(filepath.Join(dir4, ".intentci", ".gitignore"), []byte("existing\n"), 0o644)
	res, err = Run(dir4)
	if err != nil {
		t.Fatal(err)
	}
	foundGitignore := false
	for _, p := range res.Created {
		if filepath.Base(p) == ".gitignore" {
			foundGitignore = true
		}
	}
	if foundGitignore {
		t.Fatalf("gitignore should not be recreated: %+v", res.Created)
	}
}

package initcmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRootNameAndWriteErrors(t *testing.T) {
	oldW, oldM := writeFile, mkdirAll
	defer func() { writeFile, mkdirAll = oldW, oldM }()

	root := t.TempDir() + string(filepath.Separator) + "."
	if filepath.Base(root) != "." {
		t.Fatalf("base=%q root=%q", filepath.Base(root), root)
	}
	if err := Run(Options{Root: root}); err != nil {
		t.Fatal(err)
	}

	writeFile = func(string, []byte, os.FileMode) error { return errors.New("cfg") }
	if err := Run(Options{Root: t.TempDir(), Force: true}); err == nil {
		t.Fatal("cfg write")
	}

	n := 0
	writeFile = func(name string, data []byte, perm os.FileMode) error {
		n++
		if n >= 3 {
			return errors.New("example")
		}
		return oldW(name, data, perm)
	}
	if err := Run(Options{Root: t.TempDir()}); err == nil {
		t.Fatal("example write")
	}

	n = 0
	writeFile = func(name string, data []byte, perm os.FileMode) error {
		n++
		if n >= 3 {
			return errors.New("wf")
		}
		return oldW(name, data, perm)
	}
	if err := Run(Options{Root: t.TempDir(), CIGithub: true, NoExample: true}); err == nil {
		t.Fatal("workflow write")
	}
}

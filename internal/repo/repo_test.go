package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func run(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command(args[0], args[1:]...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, output)
	}
}

func write(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "init", "-q")
	run(t, root, "git", "config", "user.email", "intentci@example.com")
	run(t, root, "git", "config", "user.name", "IntentCI")
	return root
}

func TestRootAndChanged(t *testing.T) {
	root := newGitRepo(t)
	write(t, root, ".gitignore", "ignored\n")
	write(t, root, "old.go", "package old\n")
	write(t, root, "delete.go", "package gone\n")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "commit", "-qm", "initial")

	subdir := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	gotRoot, err := Root(subdir)
	if err != nil || gotRoot != root {
		t.Fatalf("Root() = %q, %v; want %q", gotRoot, err, root)
	}
	if files, err := Changed(root); err != nil || len(files) != 0 {
		t.Fatalf("clean Changed() = %v, %v", files, err)
	}

	write(t, root, "old.go", "package changed\n")
	run(t, root, "git", "add", "old.go")
	write(t, root, "new.go", "package new\n")
	write(t, root, "ignored", "ignore me\n")
	if err := os.Remove(filepath.Join(root, "delete.go")); err != nil {
		t.Fatal(err)
	}
	files, err := Changed(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"delete.go", "new.go", "old.go"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("Changed() = %v, want %v", files, want)
	}
}

func TestChangedRenameAndUnbornRepository(t *testing.T) {
	root := newGitRepo(t)
	write(t, root, "before.go", "package before\n")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "commit", "-qm", "initial")
	run(t, root, "git", "mv", "before.go", "after.go")
	files, err := Changed(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{"after.go", "before.go"}) {
		t.Fatalf("rename Changed() = %v", files)
	}

	unborn := newGitRepo(t)
	write(t, unborn, "staged.go", "package staged\n")
	write(t, unborn, "untracked.go", "package untracked\n")
	run(t, unborn, "git", "add", "staged.go")
	files, err = Changed(unborn)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{"staged.go", "untracked.go"}) {
		t.Fatalf("unborn Changed() = %v", files)
	}
}

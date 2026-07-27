package repo

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
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
	ctx := context.Background()
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
	gotRoot, err := Root(ctx, subdir)
	if err != nil || gotRoot != root {
		t.Fatalf("Root() = %q, %v; want %q", gotRoot, err, root)
	}
	if files, err := Changed(ctx, root); err != nil || len(files) != 0 {
		t.Fatalf("clean Changed() = %v, %v", files, err)
	}

	write(t, root, "old.go", "package changed\n")
	run(t, root, "git", "add", "old.go")
	write(t, root, "new.go", "package new\n")
	write(t, root, "ignored", "ignore me\n")
	if err := os.Remove(filepath.Join(root, "delete.go")); err != nil {
		t.Fatal(err)
	}
	files, err := Changed(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"delete.go", "new.go", "old.go"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("Changed() = %v, want %v", files, want)
	}
}

func TestChangedRenameAndUnbornRepository(t *testing.T) {
	ctx := context.Background()
	root := newGitRepo(t)
	write(t, root, "before.go", "package before\n")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "commit", "-qm", "initial")
	run(t, root, "git", "mv", "before.go", "after.go")
	files, err := Changed(ctx, root)
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
	files, err = Changed(ctx, unborn)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{"staged.go", "untracked.go"}) {
		t.Fatalf("unborn Changed() = %v", files)
	}
}

func TestChangedKeepsStagedAndUnstagedPathsSeparate(t *testing.T) {
	ctx := context.Background()
	root := newGitRepo(t)
	write(t, root, "file.go", "package original\n")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "commit", "-qm", "initial")

	write(t, root, "file.go", "package staged\n")
	run(t, root, "git", "add", "file.go")
	run(t, root, "git", "restore", "--worktree", "--source=HEAD", "file.go")

	files, err := Changed(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{"file.go"}) {
		t.Fatalf("Changed() = %v, want staged and unstaged path", files)
	}
}

func TestChangedRejectsCorruptHead(t *testing.T) {
	root := newGitRepo(t)
	write(t, root, ".git/HEAD", "not-a-ref\n")
	if _, err := Changed(context.Background(), root); err == nil {
		t.Fatal("Changed() accepted a corrupt HEAD")
	}
}

func TestRootCancellation(t *testing.T) {
	fakeBin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "started")
	write(t, fakeBin, "git", `#!/bin/zsh
print $$ > "$INTENTCI_GIT_MARKER"
sleep 30
`)
	if err := os.Chmod(filepath.Join(fakeBin, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))
	t.Setenv("INTENTCI_GIT_MARKER", marker)

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := Root(ctx, t.TempDir())
		result <- err
	}()
	waitForFile(t, marker)
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "not a Git repository") {
			t.Fatalf("Root() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Git process ignored cancellation")
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("Git process %d survived: %v", pid, err)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

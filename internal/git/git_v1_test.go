package git_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	repogit "github.com/hypertrial/intentci/internal/git"
)

func TestResolveCompleteRepositoryState(t *testing.T) {
	root := t.TempDir()
	run := func(arguments ...string) string {
		t.Helper()
		command := exec.Command("git", arguments...)
		command.Dir = root
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
		return string(bytes.TrimSpace(output))
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	for name, content := range map[string][]byte{
		"rename.txt": []byte("rename\n"),
		"delete.txt": []byte("delete\n"),
		"modify.txt": []byte("before\n"),
	} {
		if err := os.WriteFile(filepath.Join(root, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", ".")
	run("commit", "-m", "base")
	base := run("rev-parse", "HEAD")

	run("mv", "rename.txt", "renamed.txt")
	if err := os.Remove(filepath.Join(root, "delete.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "modify.txt"), []byte("after\nmore\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "binary.bin"), []byte{0, 1, 2}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("one\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	state, err := repogit.ResolveWithOptions(root, repogit.ResolveOptions{
		BaseRef: "HEAD", IncludeUntracked: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.BaseCommit != base || state.HeadCommit != base || !state.WorkingTreeDirty ||
		state.DiffHash == "" || state.WorkingTreeFingerprint == "" ||
		!bytes.Contains([]byte(state.DiffPatch), []byte("untracked.txt")) {
		t.Fatalf("%+v", state)
	}
	byPath := map[string]repogit.Change{}
	for _, change := range state.Changes {
		byPath[change.Path] = change
	}
	if byPath["renamed.txt"].Status != "renamed" || byPath["renamed.txt"].OldPath != "rename.txt" ||
		byPath["delete.txt"].Status != "deleted" || byPath["modify.txt"].Additions == 0 ||
		!byPath["binary.bin"].Binary || byPath["untracked.txt"].Status != "untracked" ||
		byPath["untracked.txt"].NewMode != "100755" {
		t.Fatalf("%+v", state.Changes)
	}
	firstHash := state.DiffHash
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("two\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	updated, err := repogit.ResolveWithOptions(root, repogit.ResolveOptions{BaseRef: "HEAD", IncludeUntracked: true})
	if err != nil || updated.DiffHash == firstHash {
		t.Fatalf("err=%v first=%s updated=%s", err, firstHash, updated.DiffHash)
	}
	withoutUntracked, err := repogit.ResolveWithOptions(root, repogit.ResolveOptions{
		BaseRef: "HEAD", IncludeUntracked: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range withoutUntracked.Changes {
		if change.Status == "untracked" {
			t.Fatalf("%+v", withoutUntracked.Changes)
		}
	}

	run("add", ".")
	run("commit", "-m", "head")
	head := run("rev-parse", "HEAD")
	explicit, err := repogit.ResolveWithOptions(root, repogit.ResolveOptions{
		BaseRef: base, HeadRef: head,
	})
	if err != nil || explicit.HeadCommit != head || explicit.MergeBaseFull != base {
		t.Fatalf("%+v %v", explicit, err)
	}
}

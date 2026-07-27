package evidence_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/verdict"
)

func TestNewStoreAbsAndErrors(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "absruns")
	store, err := evidence.NewStore(t.TempDir(), abs)
	if err != nil || store.Root != abs {
		t.Fatalf("%v %+v", err, store)
	}
	// mkdir fail: evidenceDir is an existing file
	root := t.TempDir()
	file := filepath.Join(root, "runs")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := evidence.NewStore(root, "runs"); err == nil {
		t.Fatal("expected mkdir error")
	}
}

func TestWriteLoadErrors(t *testing.T) {
	store, err := evidence.NewStore(t.TempDir(), "runs")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadLatest(); err == nil {
		t.Fatal("expected missing latest")
	}
	if _, err := store.Load("nope"); err == nil {
		t.Fatal("expected missing")
	}

	// WriteBundle when run dir path is a file
	if err := os.WriteFile(filepath.Join(store.Root, "bad"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteBundle(&evidence.Bundle{RunID: "bad", Run: verdict.RunResult{Verdict: "pass"}}); err == nil {
		t.Fatal("expected mkdir error")
	}

	id := evidence.NewRunID()
	b := &evidence.Bundle{
		RunID: id, CreatedAt: time.Now().UTC(),
		Document: &ir.Document{SchemaVersion: 1, Project: "p"},
		Run:      verdict.RunResult{Verdict: verdict.Pass},
	}
	if err := store.WriteBundle(b); err != nil {
		t.Fatal(err)
	}
	// corrupt result
	if err := os.WriteFile(filepath.Join(store.Dir(id), "result.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(id); err == nil {
		t.Fatal("expected parse error")
	}

	// WriteRepairPacket mkdir fail
	if err := os.WriteFile(filepath.Join(store.Root, "rp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteRepairPacket("rp", map[string]any{"a": 1}); err == nil {
		t.Fatal("expected error")
	}
	// marshal fail
	if err := store.WriteRepairPacket("open", make(chan int)); err == nil {
		t.Fatal("expected marshal error")
	}
	if err := store.WriteRepairPacket("open", map[string]any{"ok": true}); err != nil {
		t.Fatal(err)
	}
	// LoadLatest trims whitespace
	if err := os.WriteFile(filepath.Join(store.Root, "latest"), []byte(id+"\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Finalized runs are immutable; write a separate replacement run.
	id2 := evidence.NewRunID()
	b2 := &evidence.Bundle{RunID: id2, Run: verdict.RunResult{Verdict: verdict.Pass}}
	if err := store.WriteBundle(b2); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadLatest()
	if err != nil || got.RunID != id2 {
		t.Fatalf("%v %+v", err, got)
	}
}

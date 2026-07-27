package evidence_test

import (
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/verdict"
)

func TestStoreRoundTrip(t *testing.T) {
	store, err := evidence.NewStore(t.TempDir(), "runs")
	if err != nil {
		t.Fatal(err)
	}
	id := evidence.NewRunID()
	b := &evidence.Bundle{
		RunID: id, CreatedAt: time.Now().UTC(),
		Document: &ir.Document{SchemaVersion: 1, Project: "p", Requirements: nil},
		Run:      verdict.RunResult{Verdict: verdict.Pass},
	}
	if err := store.WriteBundle(b); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteRepairPacket(id, map[string]any{"run_id": id}); err == nil {
		t.Fatal("finalized run accepted a new repair packet")
	}
	got, err := store.LoadLatest()
	if err != nil || got.RunID != id {
		t.Fatalf("%v %+v", err, got)
	}
}

package evidence

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

func TestWriteBundleMarshalAndWriteErrors(t *testing.T) {
	store, err := NewStore(t.TempDir(), "runs")
	if err != nil {
		t.Fatal(err)
	}
	badDoc := &ir.Document{SchemaVersion: 1, Project: "p", Requirements: []ir.Requirement{{
		ID: "R", Obligations: []ir.Obligation{{Verify: ir.VerifyNode{Provider: &ir.ProviderSpec{
			Extra: map[string]any{"c": make(chan int)},
		}}}},
	}}}
	if err := store.WriteBundle(&Bundle{RunID: "d1", Document: badDoc, Run: verdict.RunResult{}}); err == nil {
		t.Fatal("doc marshal")
	}

	old := writeFile
	defer func() { writeFile = old }()
	n := 0
	writeFile = func(name string, data []byte, perm os.FileMode) error {
		n++
		if n == 1 {
			return errors.New("compiled write")
		}
		return old(name, data, perm)
	}
	okDoc := &ir.Document{SchemaVersion: 1, Project: "p"}
	if err := store.WriteBundle(&Bundle{RunID: "d2", CreatedAt: time.Now().UTC(), Document: okDoc, Run: verdict.RunResult{}}); err == nil {
		t.Fatal("compiled write")
	}

	writeFile = func(name string, data []byte, perm os.FileMode) error {
		return errors.New("result write")
	}
	if err := store.WriteBundle(&Bundle{RunID: "d3", Run: verdict.RunResult{}}); err == nil {
		t.Fatal("result write")
	}

	writeFile = old
	b := &Bundle{
		RunID: "d4",
		ProviderLogs: map[string]provider.Result{
			"x": {Extra: map[string]any{"c": make(chan int)}},
		},
		Run: verdict.RunResult{},
	}
	if err := store.WriteBundle(b); err == nil {
		t.Fatal("bundle marshal")
	}
}

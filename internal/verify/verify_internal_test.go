package verify

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/initcmd"
)

func TestStoreAndPersistErrors(t *testing.T) {
	root := t.TempDir()
	if err := initcmd.Run(initcmd.Options{Root: root}); err != nil {
		t.Fatal(err)
	}
	oldStore, oldPersist := newStore, persistBundle
	defer func() { newStore, persistBundle = oldStore, oldPersist }()

	newStore = func(string, string) (*evidence.Store, error) {
		return nil, errors.New("store")
	}
	out, err := Run(context.Background(), Options{Root: root, All: true})
	if err == nil || out.ExitCode != exitcode.Internal {
		t.Fatalf("%v %+v", err, out)
	}

	newStore = oldStore
	persistBundle = func(*evidence.Store, *evidence.Bundle) error { return os.ErrPermission }
	out, err = Run(context.Background(), Options{Root: root, All: true, NoCache: true})
	if err == nil || out.ExitCode != exitcode.Internal {
		t.Fatalf("%v %+v", err, out)
	}
}

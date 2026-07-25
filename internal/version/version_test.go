package version_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/version"
)

func TestString(t *testing.T) {
	old := version.Version
	defer func() { version.Version = old }()
	version.Version = "0.2.0"
	if version.String() != "0.2.0" {
		t.Fatalf("got %s", version.String())
	}
	version.Version = "0.1.0-dev"
	if version.String() == "" {
		t.Fatal("empty")
	}
}

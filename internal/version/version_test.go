package version_test

import (
	"testing"

	"github.com/hypertrial/intentci/internal/version"
)

func TestString(t *testing.T) {
	old := version.Version
	defer func() { version.Version = old }()

	version.Version = "1.1.0"
	if version.String() != "1.1.0" {
		t.Fatalf("got %q", version.String())
	}
	version.Version = ""
	if version.String() != "1.1.0" {
		t.Fatalf("got %q", version.String())
	}
}

package version

import "testing"

func TestString(t *testing.T) {
	old := Version
	defer func() { Version = old }()
	Version = "test"
	if String() != "test" {
		t.Fatal(String())
	}
	Version = ""
	if String() != "2.0.2" {
		t.Fatal(String())
	}
}

package version

import "testing"

func TestStringBranches(t *testing.T) {
	old := Version
	defer func() { Version = old }()
	Version = "9.9.9"
	if String() != "9.9.9" {
		t.Fatal(String())
	}
	Version = ""
	if String() != "0.4.0-dev" {
		t.Fatal(String())
	}
}

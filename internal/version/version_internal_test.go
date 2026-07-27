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
	if String() != "1.1.1" {
		t.Fatal(String())
	}
}

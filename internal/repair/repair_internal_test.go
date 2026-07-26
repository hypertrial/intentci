package repair

import (
	"testing"
)

func TestHelpers(t *testing.T) {
	if hashStrings([]string{"a", "b"}) == "" {
		t.Fatal("hash")
	}
	before := map[string]string{"a": "1", "b": "1"}
	after := map[string]string{"b": "1", "c": "1"}
	got := diffPaths(before, after)
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
	// snapshotDiff error path (not a git repo)
	m, err := snapshotDiff(t.TempDir())
	if err == nil || len(m) != 0 {
		t.Fatalf("err=%v m=%v", err, m)
	}
	if contains([]string{"x"}, "y") || !contains([]string{"x"}, "x") {
		t.Fatal("contains")
	}
}

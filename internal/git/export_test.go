package git

import "testing"

func TestShortAndSplit(t *testing.T) {
	if short("abc") != "abc" {
		t.Fatal(short("abc"))
	}
	if short("abcdefghi") != "abcdefg" {
		t.Fatal(short("abcdefghi"))
	}
	if splitLines("") != nil {
		t.Fatal("empty")
	}
	if len(splitLines("a\nb")) != 2 {
		t.Fatal(splitLines("a\nb"))
	}
}

func TestResolveEmptyRepo(t *testing.T) {
	// cover ensureRef/isGitRepo negative via Resolve on temp
	if _, err := Resolve("/no/such/path/intentci-git", "HEAD"); err == nil {
		t.Fatal("expected error")
	}
}

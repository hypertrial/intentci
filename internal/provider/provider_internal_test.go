package provider

import "testing"

func TestFirstNonEmptyEmpty(t *testing.T) {
	if firstNonEmpty("", "") != "" {
		t.Fatal("want empty")
	}
}

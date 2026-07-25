package cache

import "testing"

func TestHashInputs_MissingRoot(t *testing.T) {
	if _, err := hashInputs("/no/such/root/intentci", []string{"**"}); err == nil {
		t.Fatal("expected walk error")
	}
}

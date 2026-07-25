package cli

import (
	"bytes"
	"testing"
)

func TestVerify_GetwdError(t *testing.T) {
	old := getwd
	defer func() { getwd = old }()
	getwd = func() (string, error) { return "", errGetwd }
	var out, errb bytes.Buffer
	if code := RunMain([]string{"verify"}, &out, &errb); code != 1 {
		t.Fatalf("code=%d", code)
	}
}

var errGetwd = errGetwdType("getwd")

type errGetwdType string

func (e errGetwdType) Error() string { return string(e) }

package junit

import (
	"errors"
	"testing"
)

func TestFromSuites_EmptyMessages(t *testing.T) {
	res := fromSuites([]Suite{{
		Name: "s", Failures: 1, Cases: []Case{
			{Name: "a", Failure: &Failure{}},
			{Name: "b", ClassName: "c", Error: &Failure{}},
		},
	}})
	if res.OK || len(res.Failures) < 2 {
		t.Fatalf("%+v", res)
	}
}

func TestParseFile_ReadError(t *testing.T) {
	old := readFile
	defer func() { readFile = old }()
	readFile = func(string) ([]byte, error) { return nil, errors.New("boom") }
	if _, err := ParseFile("x"); err == nil {
		t.Fatal("expected error")
	}
}

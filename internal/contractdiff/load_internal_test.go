package contractdiff

import (
	"errors"
	"testing"
)

func TestLoadBase_ParseError(t *testing.T) {
	old := gitShow
	defer func() { gitShow = old }()
	gitShow = func(root, object string) func() ([]byte, error) {
		return func() ([]byte, error) {
			return []byte(":::not-yaml::: {"), nil
		}
	}
	_, _, _, err := LoadBase("/tmp", "abc")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadBase_GitError(t *testing.T) {
	old := gitShow
	defer func() { gitShow = old }()
	gitShow = func(root, object string) func() ([]byte, error) {
		return func() ([]byte, error) {
			return nil, errors.New("missing")
		}
	}
	_, _, ok, err := LoadBase("/tmp", "abc")
	if err != nil || ok {
		t.Fatal("expected missing")
	}
}

func TestRunGit_Error(t *testing.T) {
	if _, err := runGit(t.TempDir(), "not-a-real-subcommand"); err == nil {
		t.Fatal("expected error")
	}
}

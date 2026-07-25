package git

import "testing"

func TestChangedFiles_NotDirty(t *testing.T) {
	old := run
	defer func() { run = old }()
	run = func(root string, args ...string) (string, error) {
		if args[0] == "diff" && len(args) > 2 {
			return "only.go", nil
		}
		return "", nil
	}
	files, err := changedFiles(t.TempDir(), "abc", false)
	if err != nil || len(files) != 1 {
		t.Fatalf("%v %v", files, err)
	}
}

package git

import "testing"

func TestChangedFiles_DuplicateNames(t *testing.T) {
	old := run
	defer func() { run = old }()
	run = func(root string, args ...string) (string, error) {
		if args[0] == "diff" && len(args) > 2 {
			return "dup.go\ndup.go", nil
		}
		return "", nil
	}
	files, err := changedFiles(t.TempDir(), "abc", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("%#v", files)
	}
}

package main

import (
	"os"
	"testing"
)

func TestMainRunsVersion(t *testing.T) {
	oldExit := exitFunc
	oldArgs := os.Args
	defer func() {
		exitFunc = oldExit
		os.Args = oldArgs
	}()
	var code int
	exitFunc = func(c int) { code = c }
	os.Args = []string{"intentci", "version"}
	main()
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

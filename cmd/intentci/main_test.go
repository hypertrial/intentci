package main

import (
	"os"
	"testing"
)

func TestMainCompiles(t *testing.T) {
	if exitFunc == nil {
		t.Fatal("exitFunc nil")
	}
}

func TestMainInvokesExit(t *testing.T) {
	oldExit := exitFunc
	oldArgs := os.Args
	defer func() {
		exitFunc = oldExit
		os.Args = oldArgs
	}()
	var got int
	exitFunc = func(code int) { got = code }
	os.Args = []string{"intentci", "version"}
	main()
	if got != 0 {
		t.Fatalf("exit code=%d", got)
	}
}

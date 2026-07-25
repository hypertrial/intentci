package contractdiff

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func runGit(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), gitErrMessage(stderr.String(), err))
	}
	return stdout.Bytes(), nil
}

func gitErrMessage(stderr string, err error) string {
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		return err.Error()
	}
	return msg
}

// gitShow is overridable for tests.
var gitShow = func(root, object string) func() ([]byte, error) {
	return func() ([]byte, error) {
		return runGit(root, "show", object)
	}
}

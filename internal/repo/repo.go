package repo

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func Root(start string) (string, error) {
	output, err := git(start, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a Git repository: %w", err)
	}
	root := strings.TrimSpace(string(output))
	if root == "" {
		return "", fmt.Errorf("Git returned an empty repository root")
	}
	return filepath.Clean(root), nil
}

func Changed(root string) ([]string, error) {
	files := map[string]bool{}
	hasHead, err := headExists(root)
	if err != nil {
		return nil, err
	}
	if hasHead {
		for _, args := range [][]string{
			{"diff", "--cached", "--name-only", "--no-renames", "-z", "HEAD", "--"},
			{"diff", "--name-only", "--no-renames", "-z", "--"},
		} {
			output, err := git(root, args...)
			if err != nil {
				return nil, err
			}
			add(files, output)
		}
	} else {
		output, err := git(root, "ls-files", "--cached", "-z")
		if err != nil {
			return nil, err
		}
		add(files, output)
	}
	output, err := git(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	add(files, output)

	result := make([]string, 0, len(files))
	for file := range files {
		result = append(result, file)
	}
	sort.Strings(result)
	return result, nil
}

func headExists(root string) (bool, error) {
	command := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", "HEAD")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 && stderr.Len() == 0 {
		return false, nil
	}
	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		detail = err.Error()
	}
	return false, fmt.Errorf("git rev-parse --verify --quiet HEAD: %s", detail)
}

func add(files map[string]bool, output []byte) {
	for _, raw := range bytes.Split(output, []byte{0}) {
		if len(raw) != 0 {
			files[filepath.ToSlash(string(raw))] = true
		}
	}
}

func git(root string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return output, nil
}

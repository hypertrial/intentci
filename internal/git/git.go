package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// State describes the Git comparison for a verification run.
type State struct {
	Root             string
	BaseRef          string
	BaseCommit       string
	HeadCommit       string
	MergeBase        string
	MergeBaseFull    string
	ChangedFiles     []string
	WorkingTreeDirty bool
}

var run = runGit
var absPath = filepath.Abs

var execCommand = exec.Command

func runGit(root string, args ...string) (string, error) {
	cmd := execCommand("git", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Resolve computes merge-base, head, dirty status, and changed files.
func Resolve(root, baseRef string) (*State, error) {
	abs, err := absPath(root)
	if err != nil {
		return nil, err
	}
	if _, err := run(abs, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, fmt.Errorf("not a git repository: %s", abs)
	}
	head, err := run(abs, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD: %w", err)
	}
	if baseRef == "" {
		baseRef = "origin/main"
	}
	if _, err := run(abs, "rev-parse", "--verify", baseRef); err != nil {
		return nil, fmt.Errorf("missing base reference %q: %w", baseRef, err)
	}
	baseCommit, err := run(abs, "rev-parse", baseRef)
	if err != nil {
		return nil, err
	}
	mergeBase, err := run(abs, "merge-base", baseCommit, head)
	if err != nil {
		return nil, fmt.Errorf("merge-base: %w", err)
	}
	diffOut, err := run(abs, "diff", "--name-only", mergeBase)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(diffOut, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, filepath.ToSlash(line))
		}
	}
	// unstaged + untracked
	status, err := run(abs, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	dirty := status != ""
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if path == "" {
			continue
		}
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		path = filepath.ToSlash(path)
		if !contains(files, path) {
			files = append(files, path)
		}
	}
	shortMB := mergeBase
	if len(shortMB) > 12 {
		shortMB = shortMB[:12]
	}
	return &State{
		Root:             abs,
		BaseRef:          baseRef,
		BaseCommit:       baseCommit,
		HeadCommit:       head,
		MergeBase:        shortMB,
		MergeBaseFull:    mergeBase,
		ChangedFiles:     files,
		WorkingTreeDirty: dirty,
	}, nil
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// IsRepo reports whether root is a git work tree.
func IsRepo(root string) bool {
	_, err := run(root, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

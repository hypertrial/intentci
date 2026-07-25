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
	ChangedFiles     []string
	WorkingTreeDirty bool
}

// Resolve computes merge-base, head, dirty status, and changed files.
// Missing base references return an error (CLI exit code 21).
func Resolve(root, baseRef string) (*State, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if !isGitRepo(root) {
		return nil, fmt.Errorf("not a git repository: %s", root)
	}

	head, err := run(root, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD: %w", err)
	}

	if baseRef == "" {
		baseRef = "origin/main"
	}
	if err := ensureRef(root, baseRef); err != nil {
		return nil, fmt.Errorf("missing base reference %q (set policy.default_base or pass --base): %w", baseRef, err)
	}

	baseCommit, err := run(root, "rev-parse", baseRef)
	if err != nil {
		return nil, fmt.Errorf("resolve base commit: %w", err)
	}

	mergeBase, err := run(root, "merge-base", baseCommit, head)
	if err != nil {
		// Unrelated histories / single commit: use baseCommit.
		mergeBase = baseCommit
	}

	dirty, err := isDirty(root)
	if err != nil {
		return nil, err
	}

	changed, err := changedFiles(root, mergeBase, dirty)
	if err != nil {
		return nil, err
	}

	return &State{
		Root:             root,
		BaseRef:          baseRef,
		BaseCommit:       short(baseCommit),
		HeadCommit:       short(head),
		MergeBase:        short(mergeBase),
		ChangedFiles:     changed,
		WorkingTreeDirty: dirty,
	}, nil
}

func isGitRepo(root string) bool {
	_, err := run(root, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

func ensureRef(root, ref string) error {
	_, err := run(root, "rev-parse", "--verify", ref)
	return err
}

func isDirty(root string) (bool, error) {
	out, err := run(root, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func changedFiles(root, mergeBase string, dirty bool) ([]string, error) {
	seen := map[string]struct{}{}
	var files []string
	add := func(list []string) {
		for _, f := range list {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			files = append(files, f)
		}
	}

	committed, err := run(root, "diff", "--name-only", "--diff-filter=ACDMRTUXB", mergeBase, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	add(splitLines(committed))

	if dirty {
		staged, err := run(root, "diff", "--name-only", "--cached")
		if err != nil {
			return nil, err
		}
		add(splitLines(staged))

		unstaged, err := run(root, "diff", "--name-only")
		if err != nil {
			return nil, err
		}
		add(splitLines(unstaged))

		untracked, err := run(root, "ls-files", "--others", "--exclude-standard")
		if err != nil {
			return nil, err
		}
		add(splitLines(untracked))
	}

	return files, nil
}

func run(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
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

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

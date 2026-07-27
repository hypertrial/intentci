package git

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// State describes the Git comparison for a verification run.
type State struct {
	Root                   string   `json:"root"`
	BaseRef                string   `json:"base_ref"`
	BaseCommit             string   `json:"base_commit"`
	HeadCommit             string   `json:"head_commit"`
	MergeBase              string   `json:"merge_base"`
	MergeBaseFull          string   `json:"merge_base_full"`
	ChangedFiles           []string `json:"changed_files"`
	WorkingTreeDirty       bool     `json:"working_tree_dirty"`
	WorkingTreeFingerprint string   `json:"working_tree_fingerprint"`
	DiffHash               string   `json:"diff_hash"`
	DiffPatch              string   `json:"-"`
	Changes                []Change `json:"changes"`
}

// Change describes one changed repository path.
type Change struct {
	Path      string `json:"path"`
	OldPath   string `json:"old_path,omitempty"`
	Status    string `json:"status"`
	Additions int    `json:"additions,omitempty"`
	Deletions int    `json:"deletions,omitempty"`
	Binary    bool   `json:"binary,omitempty"`
	OldMode   string `json:"old_mode,omitempty"`
	NewMode   string `json:"new_mode,omitempty"`
}

// ResolveOptions configures a detailed repository comparison.
type ResolveOptions struct {
	BaseRef          string
	HeadRef          string
	IncludeUntracked bool
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

// ResolveWithOptions computes complete v1 repository provenance.
func ResolveWithOptions(root string, options ResolveOptions) (*State, error) {
	headRef := options.HeadRef
	if headRef == "" {
		headRef = "HEAD"
	}
	if headRef == "HEAD" {
		state, err := Resolve(root, options.BaseRef)
		if err != nil {
			return nil, err
		}
		if err := enrich(state, options.IncludeUntracked, ""); err != nil {
			return nil, err
		}
		return state, nil
	}
	abs, err := absPath(root)
	if err != nil {
		return nil, err
	}
	head, err := run(abs, "rev-parse", headRef)
	if err != nil {
		return nil, fmt.Errorf("resolve head %q: %w", headRef, err)
	}
	baseRef := options.BaseRef
	if baseRef == "" {
		baseRef = "origin/main"
	}
	base, err := run(abs, "rev-parse", baseRef)
	if err != nil {
		return nil, fmt.Errorf("resolve base %q: %w", baseRef, err)
	}
	mergeBase, err := run(abs, "merge-base", base, head)
	if err != nil {
		return nil, err
	}
	state := &State{
		Root: abs, BaseRef: baseRef, BaseCommit: base, HeadCommit: head,
		MergeBase: short(mergeBase), MergeBaseFull: mergeBase,
	}
	if err := enrich(state, options.IncludeUntracked, mergeBase+".."+head); err != nil {
		return nil, err
	}
	return state, nil
}

func enrich(state *State, includeUntracked bool, comparison string) error {
	if comparison == "" {
		comparison = state.MergeBaseFull
	}
	nameStatus, err := run(state.Root, "diff", "--name-status", "--find-renames", comparison)
	if err != nil {
		return err
	}
	changes := parseNameStatus(nameStatus)
	byPath := map[string]*Change{}
	for index := range changes {
		byPath[changes[index].Path] = &changes[index]
	}
	numstat, err := run(state.Root, "diff", "--numstat", "--find-renames", comparison)
	if err != nil {
		return err
	}
	for lineIndex, line := range strings.Split(numstat, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			continue
		}
		path := filepath.ToSlash(fields[2])
		change := byPath[path]
		if change == nil && lineIndex < len(changes) {
			change = &changes[lineIndex]
		}
		if change == nil {
			continue
		}
		if fields[0] == "-" || fields[1] == "-" {
			change.Binary = true
			continue
		}
		change.Additions, _ = strconv.Atoi(fields[0])
		change.Deletions, _ = strconv.Atoi(fields[1])
	}
	raw, err := run(state.Root, "diff", "--raw", "--find-renames", comparison)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		path := filepath.ToSlash(fields[len(fields)-1])
		if change := byPath[path]; change != nil {
			change.OldMode = strings.TrimPrefix(fields[0], ":")
			change.NewMode = fields[1]
		}
	}
	patch, err := run(state.Root, "diff", "--binary", "--no-ext-diff", comparison)
	if err != nil {
		return err
	}
	status, err := run(state.Root, "status", "--porcelain")
	if err != nil {
		return err
	}
	var untrackedRecords []string
	var untrackedPatches []string
	if includeUntracked {
		for _, line := range strings.Split(status, "\n") {
			if !strings.HasPrefix(line, "?? ") {
				continue
			}
			path := filepath.ToSlash(strings.TrimSpace(line[3:]))
			if path == "" {
				continue
			}
			content, _ := os.ReadFile(filepath.Join(state.Root, filepath.FromSlash(path)))
			untrackedRecords = append(untrackedRecords, path+"\x00"+hash(content))
			info, _ := os.Lstat(filepath.Join(state.Root, filepath.FromSlash(path)))
			change := Change{Path: path, Status: "untracked", NewMode: "100644"}
			if info != nil && info.Mode()&0o111 != 0 {
				change.NewMode = "100755"
			}
			if bytes.IndexByte(content, 0) >= 0 {
				change.Binary = true
			} else {
				change.Additions = bytes.Count(content, []byte{'\n'})
				if len(content) > 0 && content[len(content)-1] != '\n' {
					change.Additions++
				}
			}
			if untrackedPatch, patchErr := diffUntracked(state.Root, path); patchErr == nil {
				untrackedPatches = append(untrackedPatches, untrackedPatch)
			}
			changes = append(changes, change)
			byPath[path] = &changes[len(changes)-1]
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	state.Changes = changes
	state.ChangedFiles = state.ChangedFiles[:0]
	for _, change := range changes {
		state.ChangedFiles = append(state.ChangedFiles, change.Path)
	}
	sort.Strings(untrackedRecords)
	if len(untrackedPatches) > 0 {
		patch += "\n" + strings.Join(untrackedPatches, "\n")
	}
	state.DiffPatch = patch
	state.DiffHash = hash([]byte(patch + "\x00" + strings.Join(untrackedRecords, "\x00")))
	state.WorkingTreeDirty = status != ""
	state.WorkingTreeFingerprint = hash([]byte(status + "\x00" + patch + "\x00" + strings.Join(untrackedRecords, "\x00")))
	return nil
}

func diffUntracked(root, path string) (string, error) {
	command := execCommand("git", "diff", "--binary", "--no-index", "--", "/dev/null", filepath.FromSlash(path))
	command.Dir = root
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			return "", err
		}
	}
	return strings.TrimSpace(stdout.String()), nil
}

func parseNameStatus(raw string) []Change {
	var changes []Change
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		status := fields[0]
		change := Change{Path: filepath.ToSlash(fields[len(fields)-1]), Status: statusName(status)}
		if strings.HasPrefix(status, "R") && len(fields) == 3 {
			change.OldPath = filepath.ToSlash(fields[1])
		}
		changes = append(changes, change)
	}
	return changes
}

func statusName(status string) string {
	switch status[0] {
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	case 'T':
		return "type_changed"
	default:
		return "modified"
	}
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func hash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
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

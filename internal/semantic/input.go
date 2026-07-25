package semantic

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hypertrial/intentci/internal/changespec"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/runner"
)

// BuildOptions configures semantic request construction.
type BuildOptions struct {
	Root         string
	Profile      string
	BaseCommit   string
	HeadCommit   string
	ChangedFiles []string
	Contract     *contract.Contract
	Change       *changespec.Spec
	Selection    impact.Selection
	CheckResults map[string]runner.Result
	EnvInclude   []string
	// DiffFn returns a unified diff; overridable in tests.
	DiffFn func(root, mergeBase string) (string, error)
	// ReadFile is overridable in tests.
	ReadFile func(path string) ([]byte, error)
}

// BuildRequest constructs a bounded semantic provider request.
func BuildRequest(opt BuildOptions) (Request, error) {
	if opt.Contract == nil {
		return Request{}, fmt.Errorf("contract is required")
	}
	diffFn := opt.DiffFn
	if diffFn == nil {
		diffFn = defaultUnifiedDiff
	}
	readFile := opt.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}

	diff, err := diffFn(opt.Root, opt.BaseCommit)
	if err != nil {
		diff = ""
	}
	diff = truncateString(diff, MaxInputBytes)

	req := Request{
		ProtocolVersion: ProtocolVersion,
		Profile:         opt.Profile,
		BaseCommit:      opt.BaseCommit,
		HeadCommit:      opt.HeadCommit,
		Product: ProductContext{
			Name:     opt.Contract.Product.Name,
			Purpose:  opt.Contract.Product.Purpose,
			NonGoals: append([]string{}, opt.Contract.Product.NonGoals...),
		},
		ChangedFiles: append([]string{}, opt.ChangedFiles...),
		Diff:         redactSecrets(diff, opt.EnvInclude),
		CheckResults: checkSummaries(opt.CheckResults),
	}
	if opt.Change != nil {
		req.Change = &ChangeContext{
			ID:       opt.Change.ID,
			Goals:    append([]string{}, opt.Change.Goals...),
			NonGoals: append([]string{}, opt.Change.NonGoals...),
		}
	}

	budget := MaxInputBytes - len(req.Diff)
	for _, sr := range opt.Selection.Requirements {
		r := sr.Requirement
		if r.Status != "" && r.Status != "approved" {
			continue
		}
		sem := r.Verification.Semantic
		if sem == "" {
			sem = "optional"
		}
		if sem == "off" {
			continue
		}
		var sources []string
		for _, s := range r.Sources {
			sources = append(sources, s.Path)
		}
		req.Requirements = append(req.Requirements, RequirementContext{
			ID:          r.ID,
			Title:       r.Title,
			Statement:   r.Statement,
			Status:      r.Status,
			Severity:    r.Severity,
			Semantic:    sem,
			Checks:      append([]string{}, r.Verification.Checks...),
			SourcePaths: sources,
		})
	}
	if req.Requirements == nil {
		req.Requirements = []RequirementContext{}
	}

	// Snippets from changed files (bounded).
	for _, rel := range opt.ChangedFiles {
		if budget <= 0 {
			break
		}
		path := filepath.Join(opt.Root, rel)
		data, err := readFile(path)
		if err != nil {
			continue
		}
		content := redactSecrets(string(data), opt.EnvInclude)
		content = truncateString(content, min(16*1024, budget))
		if content == "" {
			continue
		}
		req.Snippets = append(req.Snippets, FileSnippet{Path: rel, Content: content})
		budget -= len(content)
	}
	return req, nil
}

func checkSummaries(results map[string]runner.Result) []CheckSummary {
	if len(results) == 0 {
		return []CheckSummary{}
	}
	ids := make([]string, 0, len(results))
	for id := range results {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]CheckSummary, 0, len(ids))
	for _, id := range ids {
		r := results[id]
		out = append(out, CheckSummary{ID: id, Status: r.Status, Reason: firstLine(r.Reason)})
	}
	return out
}

var gitOutput = func(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	return cmd.Output()
}

func defaultUnifiedDiff(root, mergeBase string) (string, error) {
	if mergeBase == "" {
		return "", nil
	}
	out, err := gitOutput(root, "diff", mergeBase, "HEAD")
	if err != nil {
		// Include unstaged when dirty.
		out2, err2 := gitOutput(root, "diff", mergeBase)
		if err2 != nil {
			return "", err
		}
		return string(out2), nil
	}
	return string(out), nil
}

func truncateString(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	const marker = "\n...[truncated]...\n"
	if n <= len(marker) {
		return s[:n]
	}
	return s[:n-len(marker)] + marker
}

func redactSecrets(s string, envKeys []string) string {
	out := s
	keys := append([]string{}, envKeys...)
	keys = append(keys, TokenEnv)
	for _, key := range keys {
		val := os.Getenv(key)
		if val == "" || len(val) < 4 {
			continue
		}
		out = strings.ReplaceAll(out, val, "[REDACTED]")
	}
	return out
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package explain

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hypertrial/intentci/internal/changespec"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/verify"
	"github.com/hypertrial/intentci/pkg/protocol"
)

// Options configures explain.
type Options struct {
	Root     string
	ID       string
	ChangeID string
	Base     string
	Out      io.Writer
}

// Run prints an explanation for a requirement or acceptance criterion.
func Run(opt Options) error {
	if opt.Out == nil {
		opt.Out = os.Stdout
	}
	c, _, err := contract.LoadFromRoot(opt.Root)
	if err != nil {
		return err
	}
	if err := contract.Validate(c); err != nil {
		return err
	}

	base := opt.Base
	if base == "" {
		base = c.Policy.DefaultBaseOr("origin/main")
	}
	state, err := git.Resolve(opt.Root, base)
	if err != nil {
		// Explain can still show static contract data without git.
		state = &git.State{}
	}

	var last *protocol.Result
	if data, err := os.ReadFile(verify.LastResultPath(opt.Root)); err == nil {
		var r protocol.Result
		if json.Unmarshal(data, &r) == nil {
			last = &r
		}
	}

	// Prefer Product Contract requirements when the id exists there, including AC-* ids.
	for i := range c.Requirements {
		if c.Requirements[i].ID == opt.ID {
			return explainRequirement(opt, c, state, last)
		}
	}
	if strings.HasPrefix(opt.ID, "AC-") {
		return explainAcceptance(opt, c, state, last)
	}
	return explainRequirement(opt, c, state, last)
}

func explainRequirement(opt Options, c *contract.Contract, state *git.State, last *protocol.Result) error {
	var req *contract.Requirement
	for i := range c.Requirements {
		if c.Requirements[i].ID == opt.ID {
			req = &c.Requirements[i]
			break
		}
	}
	if req == nil {
		return fmt.Errorf("unknown requirement %q", opt.ID)
	}
	w := opt.Out
	fmt.Fprintf(w, "Requirement %s\n", req.ID)
	fmt.Fprintf(w, "  Title:     %s\n", req.Title)
	fmt.Fprintf(w, "  Statement: %s\n", req.Statement)
	fmt.Fprintf(w, "  Type:      %s\n", req.Type)
	fmt.Fprintf(w, "  Status:    %s\n", req.Status)
	fmt.Fprintf(w, "  Severity:  %s\n", req.Severity)
	fmt.Fprintln(w, "  Sources:")
	if len(req.Sources) == 0 {
		fmt.Fprintln(w, "    (none)")
	}
	for _, s := range req.Sources {
		fmt.Fprintf(w, "    - %s", s.Path)
		if s.Description != "" {
			fmt.Fprintf(w, " (%s)", s.Description)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "  Applies to:")
	for _, g := range req.AppliesTo.Include {
		fmt.Fprintf(w, "    include: %s\n", g)
	}
	for _, g := range req.AppliesTo.Exclude {
		fmt.Fprintf(w, "    exclude: %s\n", g)
	}
	if len(req.AppliesTo.Include) == 0 {
		fmt.Fprintln(w, "    (no path rules)")
	}
	fmt.Fprintln(w, "  Checks:")
	for _, id := range req.Verification.Checks {
		ch, ok := c.CheckByID(id)
		if !ok {
			fmt.Fprintf(w, "    - %s (missing)\n", id)
			continue
		}
		fmt.Fprintf(w, "    - %s: %s\n", id, ch.Command)
	}
	fmt.Fprintln(w, "  Changed files (current worktree vs base):")
	matched := false
	for _, f := range state.ChangedFiles {
		fmt.Fprintf(w, "    - %s\n", f)
		matched = true
	}
	if !matched {
		fmt.Fprintln(w, "    (none)")
	}
	writeRecent(w, last, req.ID)
	writeSemanticFindings(w, last, req.ID)
	return nil
}

func explainAcceptance(opt Options, c *contract.Contract, state *git.State, last *protocol.Result) error {
	if opt.ChangeID == "" {
		return fmt.Errorf("acceptance criteria require --change <id>")
	}
	spec, _, err := changespec.Load(opt.Root, opt.ChangeID)
	if err != nil {
		return err
	}
	var ac *changespec.Acceptance
	for i := range spec.Acceptance {
		if spec.Acceptance[i].ID == opt.ID {
			ac = &spec.Acceptance[i]
			break
		}
	}
	if ac == nil {
		return fmt.Errorf("unknown acceptance criterion %q in change %s", opt.ID, opt.ChangeID)
	}
	w := opt.Out
	fmt.Fprintf(w, "Acceptance %s (change %s)\n", ac.ID, spec.ID)
	fmt.Fprintf(w, "  Statement: %s\n", ac.Statement)
	fmt.Fprintf(w, "  Severity:  %s\n", ac.Severity)
	fmt.Fprintln(w, "  Checks:")
	for _, id := range ac.Verification.Checks {
		ch, ok := c.CheckByID(id)
		if !ok {
			fmt.Fprintf(w, "    - %s (missing)\n", id)
			continue
		}
		fmt.Fprintf(w, "    - %s: %s\n", id, ch.Command)
	}
	fmt.Fprintln(w, "  Changed files:")
	if len(state.ChangedFiles) == 0 {
		fmt.Fprintln(w, "    (none)")
	}
	for _, f := range state.ChangedFiles {
		fmt.Fprintf(w, "    - %s\n", f)
	}
	writeRecent(w, last, ac.ID)
	writeSemanticFindings(w, last, ac.ID)
	return nil
}

func writeRecent(w io.Writer, last *protocol.Result, id string) {
	fmt.Fprintln(w, "  Recent local result:")
	if last == nil {
		fmt.Fprintln(w, "    (none)")
		return
	}
	for _, r := range last.Requirements {
		if r.ID == id {
			fmt.Fprintf(w, "    status: %s\n", r.Status)
			if r.Reason != "" {
				fmt.Fprintf(w, "    reason: %s\n", r.Reason)
			}
			for _, f := range r.Findings {
				fmt.Fprintf(w, "    finding: %s\n", f.Summary)
			}
			return
		}
	}
	fmt.Fprintln(w, "    (not present in last result)")
}

func writeSemanticFindings(w io.Writer, last *protocol.Result, id string) {
	fmt.Fprintln(w, "  Semantic findings:")
	if last == nil {
		fmt.Fprintln(w, "    (none)")
		return
	}
	if last.Semantic != nil && last.Semantic.Skipped != "" && last.Semantic.FindingCount == 0 {
		fmt.Fprintf(w, "    (skipped: %s)\n", last.Semantic.Skipped)
	}
	found := false
	for _, r := range last.Requirements {
		if r.ID != id {
			continue
		}
		for _, f := range r.Findings {
			if strings.HasPrefix(f.Type, "semantic_") {
				fmt.Fprintf(w, "    - %s: %s\n", f.Type, f.Summary)
				found = true
			}
		}
		for _, e := range r.Evidence {
			if e.Type == "semantic" {
				loc := e.Path
				if e.LineStart > 0 {
					loc = fmt.Sprintf("%s:%d", e.Path, e.LineStart)
					if e.LineEnd > e.LineStart {
						loc = fmt.Sprintf("%s:%d-%d", e.Path, e.LineStart, e.LineEnd)
					}
				}
				fmt.Fprintf(w, "    - evidence: %s\n", loc)
				found = true
			}
		}
	}
	if !found {
		fmt.Fprintln(w, "    (none)")
	}
}

package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/hypertrial/intentci/pkg/protocol"
	appschema "github.com/hypertrial/intentci/pkg/schema"
)

// WriteJSON writes the machine-readable result.
func WriteJSON(w io.Writer, res *protocol.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

// WriteText writes the human-readable report.
func WriteText(w io.Writer, res *protocol.Result) error {
	var b strings.Builder
	title := "IntentCI verification passed"
	switch res.Status {
	case protocol.StatusFail:
		title = "IntentCI verification failed"
	case protocol.StatusUnverified:
		title = "IntentCI verification incomplete"
	case protocol.StatusUnknown:
		title = "IntentCI verification unknown"
	}
	fmt.Fprintln(&b, title)
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Base:   %s\n", res.BaseCommit)
	fmt.Fprintf(&b, "Head:   %s\n", res.HeadCommit)
	fmt.Fprintf(&b, "Profile: %s\n", res.Profile)
	if res.ChangeSpec != nil {
		fmt.Fprintf(&b, "Change: %s\n", res.ChangeSpec.ID)
	}
	if res.WorkingTreeDirty {
		fmt.Fprintln(&b, "Worktree: dirty")
	}
	if len(res.ChangeFindings) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Change Spec findings")
		for _, f := range res.ChangeFindings {
			fmt.Fprintf(&b, "  %s: %s\n", f.Type, f.Summary)
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Requirements")
	if len(res.Requirements) == 0 {
		fmt.Fprintln(&b, "  (none affected)")
	}
	for _, r := range res.Requirements {
		fmt.Fprintf(&b, "  %-11s %s  %s\n", strings.ToUpper(r.Status), r.ID, r.Title)
	}
	for _, r := range res.Requirements {
		if r.Status == protocol.ReqPass || r.Status == protocol.ReqNotAffected {
			continue
		}
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "%s — %s\n", r.ID, strings.ToUpper(r.Status))
		if len(r.AffectedBy) > 0 {
			fmt.Fprintln(&b, "  Changed:")
			for _, f := range r.AffectedBy {
				fmt.Fprintf(&b, "    %s\n", f)
			}
		}
		for _, f := range r.Findings {
			fmt.Fprintf(&b, "  %s: %s\n", f.Type, f.Summary)
		}
		if r.Reason != "" {
			fmt.Fprintf(&b, "  Reason: %s\n", r.Reason)
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Summary")
	s := res.Summary
	fmt.Fprintf(&b, "  %d passed\n", s.Pass)
	fmt.Fprintf(&b, "  %d failed\n", s.Fail)
	fmt.Fprintf(&b, "  %d unverified\n", s.Unverified)
	fmt.Fprintf(&b, "  %d unknown\n", s.Unknown)
	fmt.Fprintf(&b, "  %d checks executed\n", s.ChecksExecuted)
	fmt.Fprintf(&b, "  %d checks cached\n", s.ChecksCached)
	_, err := io.WriteString(w, b.String())
	return err
}

// Write writes according to format and optional output path.
func Write(format, output string, res *protocol.Result, stdout io.Writer) error {
	var w io.Writer = stdout
	var f *os.File
	if output != "" {
		var err error
		f, err = os.Create(output)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	switch format {
	case "json":
		return WriteJSON(w, res)
	case "text", "":
		return WriteText(w, res)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

var (
	resultSchemaJSON = func() []byte { return appschema.ResultJSON }
	addSchemaResource = func(c *jsonschema.Compiler, url string, doc any) error {
		return c.AddResource(url, doc)
	}
	compileSchemaURL = func(c *jsonschema.Compiler, url string) (*jsonschema.Schema, error) {
		return c.Compile(url)
	}
)

// ValidateResultSchema validates a result against the published schema.
func ValidateResultSchema(res *protocol.Result) error {
	b, _ := json.Marshal(res)
	compiler := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(resultSchemaJSON()))
	if err != nil {
		return err
	}
	if err := addSchemaResource(compiler, "https://intentci.dev/schemas/result-v1.json", doc); err != nil {
		return err
	}
	sch, err := compileSchemaURL(compiler, "https://intentci.dev/schemas/result-v1.json")
	if err != nil {
		return err
	}
	var m any
	_ = json.Unmarshal(b, &m)
	return sch.Validate(m)
}

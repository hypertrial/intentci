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
	title := "IntentCI verification passed"
	switch res.Status {
	case protocol.StatusFail:
		title = "IntentCI verification failed"
	case protocol.StatusUnverified:
		title = "IntentCI verification incomplete"
	case protocol.StatusUnknown:
		title = "IntentCI verification unknown"
	}
	if _, err := fmt.Fprintln(w, title); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Base:   %s\n", res.BaseCommit); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Head:   %s\n", res.HeadCommit); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Profile: %s\n", res.Profile); err != nil {
		return err
	}
	if res.WorkingTreeDirty {
		if _, err := fmt.Fprintln(w, "Worktree: dirty"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Requirements"); err != nil {
		return err
	}
	if len(res.Requirements) == 0 {
		if _, err := fmt.Fprintln(w, "  (none affected)"); err != nil {
			return err
		}
	}
	for _, r := range res.Requirements {
		if _, err := fmt.Fprintf(w, "  %-11s %s  %s\n", strings.ToUpper(r.Status), r.ID, r.Title); err != nil {
			return err
		}
	}

	for _, r := range res.Requirements {
		if r.Status == protocol.ReqPass || r.Status == protocol.ReqNotAffected {
			continue
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%s — %s\n", r.ID, strings.ToUpper(r.Status)); err != nil {
			return err
		}
		if len(r.AffectedBy) > 0 {
			if _, err := fmt.Fprintln(w, "  Changed:"); err != nil {
				return err
			}
			for _, f := range r.AffectedBy {
				if _, err := fmt.Fprintf(w, "    %s\n", f); err != nil {
					return err
				}
			}
		}
		for _, f := range r.Findings {
			if _, err := fmt.Fprintf(w, "  %s: %s\n", f.Type, f.Summary); err != nil {
				return err
			}
		}
		if r.Reason != "" {
			if _, err := fmt.Fprintf(w, "  Reason: %s\n", r.Reason); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Summary"); err != nil {
		return err
	}
	s := res.Summary
	if _, err := fmt.Fprintf(w, "  %d passed\n", s.Pass); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  %d failed\n", s.Fail); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  %d unverified\n", s.Unverified); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  %d unknown\n", s.Unknown); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  %d checks executed\n", s.ChecksExecuted); err != nil {
		return err
	}
	return nil
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

// ValidateResultSchema validates a result against the published schema.
func ValidateResultSchema(res *protocol.Result) error {
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(appschema.ResultJSON))
	if err != nil {
		return err
	}
	if err := compiler.AddResource("https://intentci.dev/schemas/result-v1.json", doc); err != nil {
		return err
	}
	sch, err := compiler.Compile("https://intentci.dev/schemas/result-v1.json")
	if err != nil {
		return err
	}
	var m any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	return sch.Validate(m)
}

package provider

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/hypertrial/intentci/internal/ir"
)

// CommandProvider runs a shell command.
type CommandProvider struct {
	Exec func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func (p *CommandProvider) Name() string    { return "command" }
func (p *CommandProvider) Version() string { return "1.0.0" }

func (p *CommandProvider) Validate(spec ir.ProviderSpec) []Diagnostic {
	if spec.Run == "" {
		return []Diagnostic{{Message: "run is required"}}
	}
	known := map[string]bool{"type": true, "equals": true, "stdout": true, "stderr": true}
	for key := range spec.Result {
		if !known[key] {
			return []Diagnostic{{Message: fmt.Sprintf("unsupported result field %q", key)}}
		}
	}
	if value, ok := spec.Result["type"]; ok && fmt.Sprint(value) != "exit_code" {
		return []Diagnostic{{Message: "result.type must be exit_code"}}
	}
	if value, ok := spec.Result["equals"]; ok {
		switch typed := value.(type) {
		case int:
		case float64:
			if typed != float64(int(typed)) {
				return []Diagnostic{{Message: "result.equals must be an integer"}}
			}
		default:
			return []Diagnostic{{Message: "result.equals must be an integer"}}
		}
	}
	for _, stream := range []string{"stdout", "stderr"} {
		raw, ok := spec.Result[stream]
		if !ok {
			continue
		}
		rules, ok := raw.(map[string]any)
		if !ok {
			return []Diagnostic{{Message: stream + " expectation must be a mapping"}}
		}
		for key, value := range rules {
			if key != "equals" && key != "contains" && key != "matches" {
				return []Diagnostic{{Message: fmt.Sprintf("unsupported %s matcher %q", stream, key)}}
			}
			if key == "matches" {
				if _, err := regexp.Compile(fmt.Sprint(value)); err != nil {
					return []Diagnostic{{Message: fmt.Sprintf("%s matcher: %v", stream, err)}}
				}
			}
		}
	}
	return nil
}

func (p *CommandProvider) Execute(ctx context.Context, req Request) Result {
	process := runProcess(ctx, req, "sh", []string{"-c", req.Spec.Run}, nil, p.Exec)
	res := Result{
		Provider:          p.Name(),
		ProviderVersion:   p.Version(),
		Status:            "completed",
		DurationMS:        process.EndedAt.Sub(process.StartedAt).Milliseconds(),
		ExitCode:          process.ExitCode,
		SecurityViolation: process.SecurityViolation,
	}
	if req.RetainStdout {
		res.Stdout = process.Stdout
	}
	if req.RetainStderr {
		res.Stderr = process.Stderr
	}
	if process.TimedOut {
		res.Status = "error"
		res.Diagnostics = []string{fmt.Sprintf("timed out after %s", req.Timeout)}
		res.Evidence = []Evidence{{
			ID: req.Spec.ID, Class: "deterministic", Summary: "command timed out", Passed: boolPtr(false),
		}}
		return res
	}
	if process.Err != nil && process.ExitCode == nil {
		res.Status = "error"
		res.Diagnostics = []string{process.Err.Error()}
		res.Evidence = []Evidence{{
			ID: req.Spec.ID, Class: "deterministic", Summary: process.Err.Error(), Passed: boolPtr(false),
		}}
		return res
	}
	expectCode := 0
	if req.Spec.Result != nil {
		if v, ok := req.Spec.Result["equals"]; ok {
			switch t := v.(type) {
			case int:
				expectCode = t
			case float64:
				expectCode = int(t)
			}
		}
	}
	code := *process.ExitCode
	passed := code == expectCode
	summary := fmt.Sprintf("command exited %d", code)
	if !passed {
		summary = fmt.Sprintf("command exited %d, want %d", code, expectCode)
	}
	if passed {
		if ok, detail := matchOutput("stdout", process.Stdout, req.Spec.Result["stdout"]); !ok {
			passed, summary = false, detail
		} else if ok, detail := matchOutput("stderr", process.Stderr, req.Spec.Result["stderr"]); !ok {
			passed, summary = false, detail
		}
	}
	res.Evidence = []Evidence{{
		ID:      firstNonEmpty(req.Spec.ID, "command"),
		Class:   firstNonEmpty(req.Spec.EvidenceClass, req.EvidenceClass, "deterministic"),
		Summary: summary, Passed: boolPtr(passed),
		Data:      map[string]any{"exit_code": code, "run": req.Spec.Run},
		StartedAt: process.StartedAt, CompletedAt: process.EndedAt,
	}}
	return res
}

func matchOutput(name, got string, raw any) (bool, string) {
	if raw == nil {
		return true, ""
	}
	rules, ok := raw.(map[string]any)
	if !ok {
		return false, name + " expectation must be a mapping"
	}
	if want, ok := rules["equals"]; ok && got != fmt.Sprint(want) {
		return false, fmt.Sprintf("%s did not equal %q", name, want)
	}
	if want, ok := rules["contains"]; ok && !strings.Contains(got, fmt.Sprint(want)) {
		return false, fmt.Sprintf("%s did not contain %q", name, want)
	}
	if pattern, ok := rules["matches"]; ok {
		matched, err := regexp.MatchString(fmt.Sprint(pattern), got)
		if err != nil || !matched {
			return false, fmt.Sprintf("%s did not match %q", name, pattern)
		}
	}
	return true, ""
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

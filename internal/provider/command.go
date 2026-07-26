package provider

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

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
	return nil
}

func (p *CommandProvider) Execute(ctx context.Context, req Request) Result {
	start := time.Now()
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	run := p.Exec
	if run == nil {
		run = exec.CommandContext
	}
	cmd := run(ctx, "sh", "-c", req.Spec.Run)
	cmd.Dir = req.Root
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	dur := time.Since(start).Milliseconds()

	res := Result{
		Provider:        p.Name(),
		ProviderVersion: p.Version(),
		Status:          "completed",
		DurationMS:      dur,
	}
	if req.RetainStdout {
		res.Stdout = stdout.String()
	}
	if req.RetainStderr {
		res.Stderr = stderr.String()
	}
	if ctx.Err() == context.DeadlineExceeded {
		res.Status = "error"
		res.Diagnostics = []string{fmt.Sprintf("timed out after %s", timeout)}
		res.Evidence = []Evidence{{
			ID: req.Spec.ID, Class: "deterministic", Summary: "command timed out", Passed: boolPtr(false),
		}}
		return res
	}
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			res.Status = "error"
			res.Diagnostics = []string{err.Error()}
			res.Evidence = []Evidence{{
				ID: req.Spec.ID, Class: "deterministic", Summary: err.Error(), Passed: boolPtr(false),
			}}
			return res
		}
	}
	res.ExitCode = &code

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
	passed := code == expectCode
	summary := fmt.Sprintf("command exited %d", code)
	if !passed {
		summary = fmt.Sprintf("command exited %d, want %d", code, expectCode)
	}
	res.Evidence = []Evidence{{
		ID:      firstNonEmpty(req.Spec.ID, "command"),
		Class:   "deterministic",
		Summary: summary,
		Passed:  boolPtr(passed),
		Data:    map[string]any{"exit_code": code, "run": req.Spec.Run},
	}}
	return res
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

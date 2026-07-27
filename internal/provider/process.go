package provider

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"time"

	"github.com/hypertrial/intentci/internal/security"
)

type commandFactory func(context.Context, string, ...string) *exec.Cmd

type processResult struct {
	Stdout            string
	Stderr            string
	ExitCode          *int
	Err               error
	TimedOut          bool
	SecurityViolation bool
	StartedAt         time.Time
	EndedAt           time.Time
}

func runProcess(ctx context.Context, req Request, name string, args []string, stdin io.Reader, factory commandFactory) processResult {
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if factory == nil {
		factory = exec.CommandContext
	}
	dir, err := security.ResolveInside(req.Root, firstNonEmpty(req.Spec.WorkingDirectory, "."))
	if err != nil {
		return processResult{Err: err, SecurityViolation: security.IsPathViolation(err)}
	}
	cmd := factory(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = minimalEnvironment(req)
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	started := time.Now().UTC()
	err = cmd.Run()
	ended := time.Now().UTC()
	result := processResult{
		Stdout: stdout.String(), Stderr: stderr.String(), Err: err,
		TimedOut:  ctx.Err() == context.DeadlineExceeded,
		StartedAt: started, EndedAt: ended,
	}
	code := 0
	if err == nil {
		result.ExitCode = &code
	} else if exitError, ok := err.(*exec.ExitError); ok {
		code = exitError.ExitCode()
		result.ExitCode = &code
	}
	return result
}

package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/pkg/protocol"
)

// Result is the outcome of running one check.
type Result struct {
	Check      contract.Check
	Status     string
	ExitCode   *int
	DurationMS int64
	Stdout     string
	Stderr     string
	Reason     string
	FromCache  bool
}

// Options configures check execution.
type Options struct {
	Dir            string
	Stdout         io.Writer
	Stderr         io.Writer
	ExtraEnv       []string
}

// Run executes a single check with timeout.
func Run(ctx context.Context, check contract.Check, opt Options) Result {
	start := time.Now()
	timeout, err := contract.ParseTimeout(check.Timeout)
	if err != nil {
		return Result{
			Check:  check,
			Status: protocol.CheckUnknown,
			Reason: fmt.Sprintf("invalid timeout: %v", err),
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "sh", "-c", check.Command)
	cmd.Dir = opt.Dir
	cmd.Env = append(os.Environ(), opt.ExtraEnv...)

	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutW := io.Writer(&stdoutBuf)
	stderrW := io.Writer(&stderrBuf)
	if opt.Stdout != nil {
		stdoutW = io.MultiWriter(&stdoutBuf, opt.Stdout)
	}
	if opt.Stderr != nil {
		stderrW = io.MultiWriter(&stderrBuf, opt.Stderr)
	}
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	err = cmd.Run()
	dur := time.Since(start).Milliseconds()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return Result{
			Check:      check,
			Status:     protocol.CheckUnknown,
			DurationMS: dur,
			Stdout:     stdout,
			Stderr:     stderr,
			Reason:     fmt.Sprintf("check timed out after %s", timeout),
		}
	}
	if errors.Is(runCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return Result{
			Check:      check,
			Status:     protocol.CheckUnknown,
			DurationMS: dur,
			Stdout:     stdout,
			Stderr:     stderr,
			Reason:     "check canceled",
		}
	}
	if err != nil {
		code := exitCode(err)
		return Result{
			Check:      check,
			Status:     protocol.CheckFail,
			ExitCode:   &code,
			DurationMS: dur,
			Stdout:     stdout,
			Stderr:     stderr,
			Reason:     fmt.Sprintf("command exited with code %d", code),
		}
	}
	zero := 0
	return Result{
		Check:      check,
		Status:     protocol.CheckPass,
		ExitCode:   &zero,
		DurationMS: dur,
		Stdout:     stdout,
		Stderr:     stderr,
	}
}

func exitCode(err error) int {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 1
}

// Truncate keeps output bounded for reports.
func Truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}

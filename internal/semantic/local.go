package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var localJSONMarshal = json.Marshal

// LocalProvider runs a repository-local executable with JSON on stdin/stdout.
type LocalProvider struct {
	Command string
	Timeout time.Duration
	Dir     string
	// execCommand is overridable in tests.
	execCommand func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func (p *LocalProvider) Analyze(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(p.Command) == "" {
		return Response{}, fmt.Errorf("local semantic provider command is empty")
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload, err := localJSONMarshal(req)
	if err != nil {
		return Response{}, err
	}

	name, args := splitCommand(p.Command)
	run := p.execCommand
	if run == nil {
		run = exec.CommandContext
	}
	cmd := run(ctx, name, args...)
	if p.Dir != "" {
		cmd.Dir = p.Dir
	}
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return Response{}, fmt.Errorf("local semantic provider failed: %s", msg)
	}
	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return Response{}, fmt.Errorf("local semantic provider returned invalid JSON: %w", err)
	}
	if resp.Findings == nil {
		resp.Findings = []Finding{}
	}
	return resp, nil
}

func splitCommand(command string) (string, []string) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return command, nil
	}
	return fields[0], fields[1:]
}

package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunResultsAndCanceledStart(t *testing.T) {
	if err := Run(context.Background(), exec.Command("/bin/zsh", "-c", "exit 0"), time.Second); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), exec.Command("/bin/zsh", "-c", "exit 7"), time.Second)
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 7 {
		t.Fatalf("exit error = %v", err)
	}
	if err := Run(context.Background(), exec.Command("/missing-intentci-command"), time.Second); err == nil {
		t.Fatal("missing command started")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	target := filepath.Join(t.TempDir(), "started")
	err = Run(ctx, exec.Command("/usr/bin/touch", target), time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled start = %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("canceled command started: %v", err)
	}
}

func TestRunStopsCooperativeGroup(t *testing.T) {
	root := t.TempDir()
	command := exec.Command("/bin/zsh", "-c", `echo $$ > leader.pid; sleep 30`)
	command.Dir = root
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- Run(ctx, command, 500*time.Millisecond) }()

	leader := waitPID(t, filepath.Join(root, "leader.pid"))
	started := time.Now()
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() = %v", err)
	}
	if time.Since(started) >= 500*time.Millisecond {
		t.Fatal("cooperative group used the full grace period")
	}
	if processExists(leader) {
		t.Fatalf("leader %d survived", leader)
	}
}

func TestRunKillsStubbornGroup(t *testing.T) {
	root := t.TempDir()
	writeScript(t, root, "child.zsh", `trap '' TERM
echo $$ > child.pid
sleep 30
`)
	writeScript(t, root, "outer.zsh", `trap '' TERM
./child.zsh &
echo $$ > leader.pid
wait
`)
	command := exec.Command("/bin/zsh", filepath.Join(root, "outer.zsh"))
	command.Dir = root
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- Run(ctx, command, 100*time.Millisecond) }()

	leader := waitPID(t, filepath.Join(root, "leader.pid"))
	child := waitPID(t, filepath.Join(root, "child.pid"))
	defer syscall.Kill(-leader, syscall.SIGKILL)
	started := time.Now()
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() = %v", err)
	}
	if time.Since(started) < 90*time.Millisecond {
		t.Fatal("stubborn group did not receive its grace period")
	}
	if processExists(leader) || processExists(child) || groupExists(leader) {
		t.Fatalf("process group survived: leader=%d child=%d", leader, child)
	}
}

func writeScript(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte("#!/bin/zsh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func waitPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatal(err)
			}
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
	return 0
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

package process

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"
)

const pollInterval = 10 * time.Millisecond

func Run(ctx context.Context, command *exec.Cmd, grace time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return err
	}

	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	select {
	case err := <-wait:
		return err
	case <-ctx.Done():
	}
	select {
	case err := <-wait:
		return err
	default:
	}

	group := command.Process.Pid
	signalGroup(group, syscall.SIGTERM)
	timer := time.NewTimer(grace)
	ticker := time.NewTicker(pollInterval)
	defer timer.Stop()
	defer ticker.Stop()

	leaderDone := false
	for {
		select {
		case <-wait:
			leaderDone = true
			wait = nil
		case <-ticker.C:
			if !groupExists(group) {
				if !leaderDone {
					<-wait
				}
				return ctx.Err()
			}
		case <-timer.C:
			signalGroup(group, syscall.SIGKILL)
			if !leaderDone {
				<-wait
			}
			deadline := time.Now().Add(grace)
			for groupExists(group) && time.Now().Before(deadline) {
				time.Sleep(pollInterval)
			}
			return ctx.Err()
		}
	}
}

func signalGroup(group int, signal syscall.Signal) {
	err := syscall.Kill(-group, signal)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		_ = syscall.Kill(group, signal)
	}
}

func groupExists(group int) bool {
	err := syscall.Kill(-group, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

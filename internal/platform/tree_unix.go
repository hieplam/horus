//go:build unix

// The process tree on macOS and Linux: a new session, so the root leads a
// new process group, and every process that it starts is in that group.
package platform

import (
	"errors"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

func startTree(cmd *exec.Cmd) (string, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	if cmd.Cancel != nil { // only exec.CommandContext sets Cancel
		// exec sets cmd.Process before it can call Cancel.
		cmd.Cancel = func() error { return endGroup(cmd.Process.Pid) }
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	return strconv.Itoa(cmd.Process.Pid), nil
}

func endTree(id string) error {
	pgid, err := strconv.Atoi(id)
	if err != nil {
		return ErrBadTree
	}
	return endGroup(pgid)
}

// endGroup sends SIGTERM to the process group pgid, waits up to endWait for
// the group to be empty, then sends SIGKILL.
func endGroup(pgid int) error {
	if pgid <= 1 {
		// kill(-1) reaches every process of the user. Never send it.
		return ErrBadTree
	}
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	deadline := time.Now().Add(endWait)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(-pgid, 0), syscall.ESRCH) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

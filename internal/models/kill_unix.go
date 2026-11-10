//go:build darwin || linux

// Starting and killing the `horus workflow` child on macOS and Linux. Part of
// the port of horus/models.py (Run.cancel).
package models

import (
	"os"
	"os/exec"
	"syscall"
)

// prepare puts the child in a process group of its own, so that killTree
// reaches every process that it starts.
func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTree sends SIGKILL to the process group of the child, as models.py
// does with `taskkill /T /F` on Windows.
func killTree(p *os.Process) error {
	if p == nil {
		return nil
	}
	_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
	_ = p.Kill()
	return nil
}

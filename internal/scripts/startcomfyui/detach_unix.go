//go:build darwin || linux

package startcomfyui

import (
	"os/exec"
	"syscall"
)

// detach puts the child process in its own session, so it keeps running after
// this command ends.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

//go:build unix

package platform

import (
	"os/exec"
	"syscall"
)

// ownGroup makes the starter lead a process group of its own, as a shell
// does for a command.
func ownGroup(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

// groupOfStarterHolds sends SIGKILL to the process group of the ended
// starter, as a terminal does when it closes. It tells if a process was
// still in that group. A detached tree is in its own group, so the signal
// reaches no process.
func groupOfStarterHolds(starterPid int) bool {
	return syscall.Kill(-starterPid, syscall.SIGKILL) == nil
}

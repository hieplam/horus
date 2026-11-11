//go:build unix

package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

// alive tells if the process pid runs. A zombie (ended, not yet reaped) does
// not run.
func alive(pid int) bool {
	if runtime.GOOS == "linux" {
		if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
			s := string(b)
			if i := strings.LastIndex(s, ") "); i >= 0 && i+2 < len(s) && s[i+2] == 'Z' {
				return false
			}
		}
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

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

package platform

import (
	"os/exec"
	"syscall"
)

const (
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
)

// alive tells if the process pid runs.
func alive(pid int) bool {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}

// ownGroup does nothing on Windows: the starter has no process group to
// check. Check 1 still proves that the tree lives after its starter ends,
// and that the name of the job lives after the starter closed its handle.
func ownGroup(*exec.Cmd) {}

func groupOfStarterHolds(int) bool { return false }

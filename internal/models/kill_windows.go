// Starting and killing the `horus workflow` child on Windows. Part of the port
// of horus/models.py (creationflags=CREATE_NO_WINDOW, Run.cancel).
package models

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW of the Windows API.
const createNoWindow = 0x08000000

// prepare keeps the child from opening a console window, as models.py does.
func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}

// killTree stops the child and every process that it started, as models.py
// does with `taskkill /T /F`. Then it kills the child itself, in case
// taskkill is not there.
func killTree(p *os.Process) error {
	if p == nil {
		return nil
	}
	taskkill := "taskkill.exe"
	if root := os.Getenv("SystemRoot"); root != "" {
		taskkill = filepath.Join(root, "System32", "taskkill.exe")
	}
	kill := exec.Command(taskkill, "/PID", strconv.Itoa(p.Pid), "/T", "/F")
	prepare(kill)
	_ = kill.Run()
	_ = p.Kill()
	return nil
}

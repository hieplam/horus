// Starting `horus start-comfyui` on Windows. Part of the port of
// horus/comfy.py (ensure_up, creationflags=CREATE_NO_WINDOW).
package comfy

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW of the Windows API.
const createNoWindow = 0x08000000

// hideWindow keeps the child from opening a console window, as comfy.py does.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}

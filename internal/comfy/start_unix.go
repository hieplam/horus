//go:build darwin || linux

// Starting `horus start-comfyui` on macOS and Linux. Part of the port of
// horus/comfy.py (ensure_up).
package comfy

import "os/exec"

// hideWindow does nothing: macOS and Linux open no console window for a child.
func hideWindow(cmd *exec.Cmd) {}

//go:build unix

package platform

import (
	"os"
	"syscall"
)

// StopSignals are the signals that stop a long command such as horus worker:
// Ctrl+C, and SIGTERM, which docker stop sends.
var StopSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

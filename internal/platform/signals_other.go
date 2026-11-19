//go:build !unix

package platform

import "os"

// StopSignals are the signals that stop a long command such as horus worker:
// Ctrl+C.
var StopSignals = []os.Signal{os.Interrupt}

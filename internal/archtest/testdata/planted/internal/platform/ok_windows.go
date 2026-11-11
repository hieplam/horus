// Not a violation: internal/platform can hold code for one OS.
package platform

import "syscall"

var _ = syscall.Getpid

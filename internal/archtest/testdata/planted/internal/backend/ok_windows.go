//go:build windows

// Not a violation: internal/backend can hold code for one OS.
package backend

import "runtime"

var goos = runtime.GOOS

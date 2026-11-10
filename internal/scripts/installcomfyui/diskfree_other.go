//go:build !windows

package installcomfyui

import "errors"

// freeSpace is not needed: the command refuses to run on this OS.
func freeSpace(path string) (int64, error) {
	return 0, errors.New("not supported on this OS yet")
}

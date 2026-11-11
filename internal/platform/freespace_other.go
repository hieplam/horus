//go:build !windows && !darwin && !linux

// Other systems: the free space is not read, and the download goes on with a
// warning.
package platform

import "errors"

func freeSpace(string) (uint64, error) {
	return 0, errors.New("free space is not known on this system")
}

//go:build !windows && !darwin && !linux

// Other systems: the free space is not read, and the download goes on with a
// warning, as download-model.ps1 does for a network path.
package platform

import "errors"

func freeSpace(string) (uint64, error) {
	return 0, errors.New("free space is not known on this system")
}

//go:build darwin || linux

// The free space of a file system on macOS and Linux. Ports the DriveInfo
// check of scripts/download-model.ps1.
package platform

import "syscall"

// freeSpace returns the bytes that a user without root can write in dir.
func freeSpace(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}

// The free space of a folder on Windows. Ports the DriveInfo check of
// scripts/download-model.ps1.
package platform

import (
	"syscall"
	"unsafe"
)

var procGetDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")

// freeSpace returns the bytes that the caller can write on the drive of dir.
func freeSpace(dir string) (uint64, error) {
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var avail, total, free uint64
	r, _, e := procGetDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&avail)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&free)))
	if r == 0 {
		return 0, e
	}
	return avail, nil
}

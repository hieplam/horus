package installcomfyui

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

// freeSpace returns the free bytes that the current user can use on the drive of path.
func freeSpace(path string) (int64, error) {
	root := filepath.VolumeName(path) + `\`
	p, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return 0, err
	}
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")
	var avail, total, free uint64
	r, _, e := proc.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&avail)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&free)))
	if r == 0 {
		return 0, e
	}
	return int64(avail), nil
}

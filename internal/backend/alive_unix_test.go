//go:build unix

package backend

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
)

// alive tells if the process pid runs. A zombie (ended, not yet reaped) does
// not run.
func alive(pid int) bool {
	if runtime.GOOS == "linux" {
		if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
			s := string(b)
			if i := strings.LastIndex(s, ") "); i >= 0 && i+2 < len(s) && s[i+2] == 'Z' {
				return false
			}
		}
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

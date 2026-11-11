//go:build !windows

package platform

import (
	"os"
	"path/filepath"
)

func blockDelete(path string) (func(), error) {
	if os.Geteuid() == 0 {
		return nil, ErrCannotBlock
	}
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o555); err != nil {
		return nil, err
	}
	return func() { os.Chmod(dir, 0o755) }, nil
}

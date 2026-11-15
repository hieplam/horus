//go:build !windows

package platform

import "os"

func pathEntry(_, home string) ([]string, error) { return profilePathEntry(home) }

func removePathEntry(_, home string) error { return removeProfileLines(home) }

// removeSelf deletes the file at once: a running program can delete its own
// file on Unix.
func removeSelf(self string) error { return os.Remove(self) }

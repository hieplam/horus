//go:build !unix && !windows

// Other systems (js/wasm, plan9, wasip1): horus starts no process tree there.
package platform

import (
	"errors"
	"os/exec"
)

var errNoTree = errors.New("process trees are not supported on this system")

func startTree(*exec.Cmd) (string, error) { return "", errNoTree }

func endTree(string) error { return errNoTree }

// alive reports false: horus starts no process on these systems.
func alive(int) bool { return false }

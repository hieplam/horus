// Package platform holds the code of horus that differs between operating
// systems. A caller gets a few plain functions. It never sees a SysProcAttr,
// a handle or a signal.
//
//   - StartTree starts a process tree that keeps running after its starter
//     ends. EndTree ends that tree, also from another process.
//   - NoConsole starts a child without a console window.
//   - FreeSpace gives the free bytes of a folder.
//   - IsRooted, VenvBin, ExeName and PythonLauncher give the paths and the
//     program names of this OS.
//   - BlockDelete makes the delete of a file fail, for a test.
//   - ErrNotSupported is the error of a step that this OS does not have yet.
//
// This package is a base package: it imports no other horus package.
package platform

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// StartTree starts cmd as the root of a new process tree, and returns the id
// of the tree. Every process that the root starts joins the tree.
//
// The tree has no console window, it does not get the Ctrl+C of this
// console, and it keeps running after this process ends. Any process can
// pass the id to EndTree. When cmd comes from exec.CommandContext, the end of
// the context ends the whole tree. StartTree sets cmd.SysProcAttr and
// cmd.Cancel. The caller calls cmd.Wait as after cmd.Start.
//
// Windows: the tree is a named Job Object without the "end on close" flag.
// Unix: the tree is a new session and its process group.
func StartTree(cmd *exec.Cmd) (string, error) { return startTree(cmd) }

// EndTree ends every process of the tree that StartTree started. A tree
// that has no process left is not an error.
//
// Windows: it opens the Job Object by its name and ends it. Unix: it sends
// SIGTERM to the process group, waits up to EndWait, then sends SIGKILL.
func EndTree(id string) error { return endTree(id) }

// EndWait is the time that EndTree waits on Unix between SIGTERM and SIGKILL.
var EndWait = 3 * time.Second

// ErrBadTree is the error of EndTree for an id that StartTree cannot make.
var ErrBadTree = errors.New("not the id of a process tree")

// ErrNotSupported is the error of a step that horus cannot do on this OS
// yet, for example the ComfyUI backend on macOS (Phase 3).
var ErrNotSupported = errors.New("not supported on this OS yet")

// NoConsole makes cmd start without a console window. It does nothing on
// macOS and Linux, because they open no window for a child.
func NoConsole(cmd *exec.Cmd) { noConsole(cmd) }

// FreeSpace returns the bytes that the current user can write in the folder
// dir. The folder must exist.
func FreeSpace(dir string) (uint64, error) { return freeSpace(dir) }

// BlockDelete makes a delete of the file path fail until undo is called. It
// is for tests of a delete that fails. Windows: it keeps the file open,
// because Go opens a file without FILE_SHARE_DELETE. Elsewhere: it makes the
// folder of the file read-only.
func BlockDelete(path string) (undo func(), err error) { return blockDelete(path) }

// ErrCannotBlock is the error of BlockDelete when this process can delete
// the file in all cases (root on Unix).
var ErrCannotBlock = errors.New("root can delete in a read-only folder")

// IsRooted tells if p starts at a root of this OS, as
// [IO.Path]::IsPathRooted of .NET does: "/x", "C:\x", "C:x", and on Windows
// also "\x".
func IsRooted(p string) bool {
	if filepath.IsAbs(p) || filepath.VolumeName(p) != "" {
		return true
	}
	return strings.HasPrefix(p, "/") || (runtime.GOOS == "windows" && strings.HasPrefix(p, `\`))
}

// ExeName returns the file name of the program name on this OS: name.exe on
// Windows, name elsewhere.
func ExeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// VenvBin returns the path of the program name in the Python venv venv:
// <venv>\Scripts\<name>.exe on Windows, <venv>/bin/<name> elsewhere.
func VenvBin(venv, name string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venv, "Scripts", ExeName(name))
	}
	return filepath.Join(venv, "bin", name)
}

// PythonLauncher returns the command that starts Python version (for
// example "3.12") on this OS: `py.exe -3.12` on Windows, `python3.12`
// elsewhere. It returns an error that tells the user what to install when
// the program is not on the PATH.
func PythonLauncher(version string) ([]string, error) {
	if runtime.GOOS == "windows" {
		p, err := exec.LookPath("py.exe")
		if err != nil {
			return nil, fmt.Errorf("The Python Launcher (py.exe) was not found. Install Python %s from python.org.", version)
		}
		return []string{p, "-" + version}, nil
	}
	p, err := exec.LookPath("python" + version)
	if err != nil {
		return nil, fmt.Errorf("python%s was not found. Install Python %s.", version, version)
	}
	return []string{p}, nil
}

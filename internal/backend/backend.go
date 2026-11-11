// Package backend is the adapter for the ComfyUI install of this machine: it
// installs, starts and stops ComfyUI, and it knows the folders of the
// install. With internal/platform, it is the only place for code of one OS.
//
// Phase 2 has one backend: the official Windows portable build (portable.go).
// On another OS, New returns a backend whose Install, Start and Stop answer
// platform.ErrNotSupported. Its Installed and Layout still work, because the
// layout of the ComfyUI folder is the same on every OS.
//
// The interface has no pid, no job name and no archive in it, so a backend
// that starts ComfyUI through another program (comfy-cli on macOS, Phase 3)
// fits it without a change.
package backend

import (
	"context"
	"io"
	"runtime"

	"github.com/hieplam/horus/internal/platform"
)

// Backend installs, starts and stops ComfyUI. Each call that runs for a
// while writes its text lines to out.
type Backend interface {
	// Install installs ComfyUI. It does nothing when ComfyUI is already
	// installed.
	Install(ctx context.Context, out io.Writer) error
	// Start starts ComfyUI when its server does not answer, then waits until
	// it answers. ComfyUI keeps running after the caller ends. An error for
	// a ComfyUI that is not installed has the method NotInstalled.
	Start(ctx context.Context, out io.Writer) error
	// Stop ends the ComfyUI that Start started, with every process that it
	// started. It refuses a ComfyUI that horus did not start.
	Stop(ctx context.Context, out io.Writer) error
	// Installed gives the install folder, and reports if ComfyUI is
	// installed there.
	Installed() (dir string, ok bool)
	// Layout gives the folders and files of the install.
	Layout() Layout
}

// Layout is the folders and files of one install, as full paths.
type Layout struct {
	Dir     string   // the install folder (HORUS_COMFYUI_DIR)
	ComfyUI string   // the ComfyUI folder: main.py, models/, user/
	Python  string   // the Python that runs ComfyUI
	Logs    []string // the log files of a ComfyUI that horus started
}

// notInstalledError is the error of Start when ComfyUI is not installed.
type notInstalledError struct{ msg string }

func (e *notInstalledError) Error() string { return e.msg }

// NotInstalled marks the error, so that a caller can test for it with
// errors.As and an interface of its own.
func (e *notInstalledError) NotInstalled() bool { return true }

// New gives the backend of this OS: the portable build on Windows, else a
// backend that answers "not supported on this OS yet".
func New(p *Portable) Backend { return newFor(runtime.GOOS, p) }

func newFor(goos string, p *Portable) Backend {
	if goos == "windows" {
		return p
	}
	return unsupported{p}
}

// unsupported is the backend of an OS that horus cannot set up yet.
type unsupported struct{ *Portable }

func (unsupported) Install(context.Context, io.Writer) error { return platform.ErrNotSupported }
func (unsupported) Start(context.Context, io.Writer) error   { return platform.ErrNotSupported }
func (unsupported) Stop(context.Context, io.Writer) error    { return platform.ErrNotSupported }

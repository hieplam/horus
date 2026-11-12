// Package backend is the adapter for the ComfyUI install of this machine: it
// installs, starts and stops ComfyUI, and it knows the folders of the
// install. With internal/platform, it is the only place for code of one OS.
//
// New gives the backend of this OS:
//
//   - Windows: the official Windows portable build (portable.go).
//   - macOS: the comfy-cli backend (comfycli.go). It runs the program comfy
//     of Comfy Org as a separate process.
//   - every other OS: a backend whose Install, Start and Stop answer
//     platform.ErrNotSupported. Its Installed and Layout still work.
//
// Every backend has the same rule for "installed": <dir>/ComfyUI/main.py is a
// file. The interface has no pid, no job name and no archive in it.
package backend

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
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

// Place is what every backend needs: the install folder, the address of the
// server, and the waits of Start and Stop. An empty field has its default;
// Dir and URL are then read from the settings at each call.
type Place struct {
	Dir string // the install folder (default: HORUS_COMFYUI_DIR, then platform.DefaultComfyDir)
	URL string // the address that the server answers on; the port comes from it (default: HORUS_COMFYUI_URL)

	Timeout      time.Duration // the wait of Start (default: startTimeout)
	StopTimeout  time.Duration // the wait of Stop (default: stopTimeout)
	PollInterval time.Duration // the time between two checks of the server (default: 2 s)
}

// Archive is what only the Windows portable build needs: the archive of its
// install. The other backends do not take it.
type Archive struct {
	ArchivePath string // where the .7z file stays during the install (default: next to Dir)
	SevenZip    string // the full path of 7z.exe (default: the PATH, then Program Files\7-Zip)
	KeepArchive bool   // do not delete the archive after a successful install
}

// New gives the backend of this OS for the place p. Only the Windows
// portable build reads a.
func New(p Place, a Archive) Backend { return newFor(runtime.GOOS, p, a) }

// TakesArchive reports if the backend of this OS installs from an archive
// (the Windows portable build), so that the values of Archive mean something.
func TakesArchive() bool { return takesArchive(runtime.GOOS) }

func takesArchive(goos string) bool { return goos == "windows" }

func newFor(goos string, p Place, a Archive) Backend {
	switch {
	case takesArchive(goos):
		return &portable{Place: p, Archive: a}
	case goos == "darwin":
		return &comfyCLI{Place: p}
	}
	return unsupported{&portable{Place: p}}
}

// unsupported is the backend of an OS that horus cannot set up yet.
type unsupported struct{ *portable }

func (unsupported) Install(context.Context, io.Writer) error { return platform.ErrNotSupported }
func (unsupported) Start(context.Context, io.Writer) error   { return platform.ErrNotSupported }
func (unsupported) Stop(context.Context, io.Writer) error    { return platform.ErrNotSupported }

// The pinned ComfyUI version of every backend.
const version = "v0.44.0"

// -- what every backend shares -------------------------------------------------

func (p Place) dir() string {
	if p.Dir != "" {
		return p.Dir
	}
	return settings.ComfyDir()
}

func (p Place) serverURL() string {
	if p.URL != "" {
		return strings.TrimRight(p.URL, "/")
	}
	return settings.ComfyURL()
}

// comfyUIDir is the ComfyUI folder of the install: <dir>/ComfyUI.
func comfyUIDir(dir string) string { return filepath.Join(dir, "ComfyUI") }

// isInstalled reports if ComfyUI is installed in dir: its ComfyUI/main.py
// is a file.
func isInstalled(dir string) bool { return platform.IsFile(filepath.Join(comfyUIDir(dir), "main.py")) }

// sayInstalled writes the last lines of an install.
func sayInstalled(out io.Writer, dir string) {
	fmt.Fprintf(out, "OK: ComfyUI %s is installed in %s\n", version, dir)
	fmt.Fprintf(out, "Next: horus start-comfyui -comfyui-dir '%s'\n", dir)
}

// saySkip writes the line of an install that finds ComfyUI in dir.
func saySkip(out io.Writer, dir string) {
	fmt.Fprintf(out, "SKIP: ComfyUI is already installed in %s\n", dir)
}

// notInstalledError is the error of Start when ComfyUI is not installed.
type notInstalledError struct{ msg string }

func (e *notInstalledError) Error() string { return e.msg }

// NotInstalled marks the error, so that a caller can test for it with
// errors.As and an interface of its own.
func (e *notInstalledError) NotInstalled() bool { return true }

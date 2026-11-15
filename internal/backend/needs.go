// What the install of ComfyUI needs on each OS before Install runs: the
// system package and the program comfy. The setup plan of internal/provision
// reads it (spec docs/specs/2026-11-15-release-install.md, "The setup plan").
package backend

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/hieplam/horus/internal/platform"
)

// Needs is what the install of ComfyUI on one OS needs.
type Needs struct {
	// Package is the system package that the install needs, or nil.
	// Windows: 7-Zip, because Install unpacks the archive with 7z.exe.
	// macOS: Python 3.12, because comfy-cli runs in a venv of Python 3.12.
	Package *platform.Package
	// ComfyCLI is true when Install runs the program comfy (macOS). The
	// package is then only needed when comfy is not found.
	ComfyCLI bool
}

// SetupNeeds gives the needs of the backend of the OS goos (a value of
// runtime.GOOS; "" means this OS). An OS without a backend needs nothing.
func SetupNeeds(goos string) Needs {
	if goos == "" {
		goos = runtime.GOOS
	}
	switch {
	case takesArchive(goos):
		return Needs{Package: &platform.Package{
			Name:    "7-Zip",
			Program: "7z",
			Dirs:    sevenZipDirs(),
			Manager: "winget",
			Args:    []string{"install", "--id", "7zip.7zip", "-e", "--accept-source-agreements", "--accept-package-agreements"},
			URL:     "https://www.7-zip.org/",
		}}
	case goos == "darwin":
		return Needs{ComfyCLI: true, Package: &platform.Package{
			Name:         "Python 3.12",
			Program:      "python3.12",
			InManagerDir: true,
			Manager:      "brew",
			Args:         []string{"install", "python@3.12"},
			URL:          "https://www.python.org/downloads/",
		}}
	}
	return Needs{}
}

// sevenZipDirs are the folders where winget puts 7z.exe: the 7-Zip folder of
// %ProgramFiles%, then of %ProgramFiles(x86)%.
func sevenZipDirs() []string {
	var dirs []string
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
		if base := os.Getenv(env); base != "" {
			dirs = append(dirs, filepath.Join(base, "7-Zip"))
		}
	}
	return dirs
}

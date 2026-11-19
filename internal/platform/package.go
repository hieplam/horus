// Package: a system package that a setup plan needs (the ComfyUI install, the
// speech setup), and the command of the package manager that installs it.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Package is one system package: what it gives, where the package manager
// puts it, and the command that installs it. It is plain data. The backend of
// each OS gives the package of the ComfyUI install (internal/backend,
// SetupNeeds). SpeechPackagesFor gives the packages of the speech setup.
type Package struct {
	// Name is the name of the package for a person, for example "7-Zip".
	Name string
	// Program is the program that the package gives, without ".exe", for
	// example "7z" or "python3.12". A caller looks for it on the PATH.
	Program string
	// Exe is the file name of the program in Dirs when it is not Program or
	// Program.exe: python.exe in the folder of Python 3.12 on Windows.
	Exe string
	// Dirs are the folders where the package manager puts Program, in
	// order. A caller looks there after the install, because the PATH of a
	// running process does not change. A folder can be a pattern of
	// filepath.Glob, for a folder whose name has the version of the package.
	Dirs []string
	// InManagerDir is true when the package manager also puts Program in its
	// own folder (the bin folder of Homebrew).
	InManagerDir bool
	// Manager is the program of the package manager, for example "winget".
	Manager string
	// Args are the arguments of Manager that install the package.
	Args []string
	// URL is where a person gets the package by hand.
	URL string
}

// SpeechPackages are the system packages of the speech setup on one OS
// (spec docs/specs/2026-11-20-text-to-speech.md, "Install and uninstall").
// A nil field: horus has no package manager step for it on that OS.
type SpeechPackages struct {
	// Python is Python 3.12: the speech venv is made with it.
	Python *Package
	// FFmpeg gives the programs ffmpeg and ffprobe. The speech worker runs
	// them, so they must be on the PATH.
	FFmpeg *Package
}

// SpeechPackagesFor gives the speech packages of the OS goos (a value of
// runtime.GOOS; "" means this OS). Windows: winget, with the folders where
// winget puts each program for the current user. macOS: Homebrew. Another
// OS has none.
func SpeechPackagesFor(goos string) SpeechPackages {
	if goos == "" {
		goos = runtime.GOOS
	}
	return speechPackages(goos, os.Getenv)
}

// wingetArgs are the arguments of winget that install the package id with no
// question.
func wingetArgs(id string) []string {
	return []string{"install", "--id", id, "-e", "--accept-source-agreements", "--accept-package-agreements"}
}

// speechPackages is SpeechPackagesFor with the environment getenv. It is a
// pure function.
func speechPackages(goos string, getenv func(string) string) SpeechPackages {
	switch goos {
	case "windows":
		local := getenv("LOCALAPPDATA")
		var pyDirs, ffDirs []string
		if local != "" {
			pyDirs = append(pyDirs, filepath.Join(local, "Programs", "Python", "Python312"))
			// winget puts a link to a portable program in the folder Links. When it
			// cannot make a link, the program stays in the folder of the package,
			// whose name has the version of FFmpeg.
			ffDirs = append(ffDirs, filepath.Join(local, "Microsoft", "WinGet", "Links"),
				filepath.Join(local, "Microsoft", "WinGet", "Packages", "Gyan.FFmpeg_*", "ffmpeg-*", "bin"))
		}
		if pf := getenv("ProgramFiles"); pf != "" {
			pyDirs = append(pyDirs, filepath.Join(pf, "Python312"))
		}
		return SpeechPackages{
			Python: &Package{Name: "Python 3.12", Program: "python3.12", Exe: "python.exe", Dirs: pyDirs,
				Manager: "winget", Args: wingetArgs("Python.Python.3.12"), URL: "https://www.python.org/downloads/"},
			FFmpeg: &Package{Name: "FFmpeg", Program: "ffmpeg", Dirs: ffDirs,
				Manager: "winget", Args: wingetArgs("Gyan.FFmpeg"), URL: "https://ffmpeg.org/download.html"},
		}
	case "darwin":
		return SpeechPackages{
			Python: &Package{Name: "Python 3.12", Program: "python3.12", InManagerDir: true,
				Manager: "brew", Args: []string{"install", "python@3.12"}, URL: "https://www.python.org/downloads/"},
			FFmpeg: &Package{Name: "FFmpeg", Program: "ffmpeg", InManagerDir: true,
				Manager: "brew", Args: []string{"install", "ffmpeg"}, URL: "https://ffmpeg.org/download.html"},
		}
	}
	return SpeechPackages{}
}

// AddToPath puts the folder dir first on the PATH of this process, when the
// PATH does not have it. The programs that this process starts after the call
// get the new PATH. It changes no file and no setting of the user.
func AddToPath(dir string) {
	old := os.Getenv("PATH")
	for _, d := range filepath.SplitList(old) {
		if samePath(d, dir) {
			return
		}
	}
	if old == "" {
		os.Setenv("PATH", dir)
		return
	}
	os.Setenv("PATH", dir+string(os.PathListSeparator)+old)
}

// samePath tells if a and b name the same folder: without case on Windows
// and macOS, with case elsewhere.
func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

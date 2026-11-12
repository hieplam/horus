package platform

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVenvPathsFollowTheOS(t *testing.T) {
	want := filepath.Join("v", "bin", "python")
	if runtime.GOOS == "windows" {
		want = filepath.Join("v", "Scripts", "python.exe")
	}
	if got := VenvBin("v", "python"); got != want {
		t.Errorf("VenvBin = %s, want %s", got, want)
	}
}

func TestPythonCommandFollowsTheOS(t *testing.T) {
	cmd, notFound := PythonCommand("3.12")
	wantCmd, wantText := "python3.12", "python3.12 was not found. Install Python 3.12."
	if runtime.GOOS == "windows" {
		wantCmd, wantText = "py.exe -3.12", "The Python Launcher (py.exe) was not found. Install Python 3.12 from python.org."
	}
	if strings.Join(cmd, " ") != wantCmd || notFound != wantText {
		t.Errorf("PythonCommand = %q, %q, want %q, %q", cmd, notFound, wantCmd, wantText)
	}
}

func TestIsRootedIsIsPathRootedOfThisOS(t *testing.T) {
	cases := map[string]bool{"/x": true, "x": false, "./x": false, "": false}
	if runtime.GOOS == "windows" {
		cases[`\x`] = true
		cases[`C:\x`] = true
		cases[`C:x`] = true
	} else {
		cases[`\x`] = false
	}
	for p, want := range cases {
		if got := IsRooted(p); got != want {
			t.Errorf("IsRooted(%q) = %v, want %v", p, got, want)
		}
	}
}

// TestDefaultComfyDirOfEachOS: the default install folder is
// <home>/ComfyUI_windows_portable on Windows and <home>/horus on macOS and on
// every other OS. DefaultComfyDir follows the OS of this build.
func TestDefaultComfyDirOfEachOS(t *testing.T) {
	home := filepath.Join("h", "user")
	cases := map[string]string{
		"windows": filepath.Join(home, "ComfyUI_windows_portable"),
		"darwin":  filepath.Join(home, "horus"),
		"linux":   filepath.Join(home, "horus"),
	}
	for goos, want := range cases {
		if got := defaultComfyDir(goos, home); got != want {
			t.Errorf("%s: defaultComfyDir = %s, want %s", goos, got, want)
		}
	}
	if got, want := DefaultComfyDir(home), defaultComfyDir(runtime.GOOS, home); got != want {
		t.Errorf("DefaultComfyDir = %s, want the folder of %s: %s", got, runtime.GOOS, want)
	}
}

// TestInTCCFolder: on macOS a path is inside a TCC folder when it is
// <home>/Desktop, <home>/Documents or <home>/Downloads, or a path under one
// of them, in any case. A folder with the same start of name, a path outside
// the home folder and the default install folder are outside. On Windows and
// Linux no path is inside. InTCCFolder follows the OS of this build.
func TestInTCCFolder(t *testing.T) {
	home := filepath.FromSlash("/Users/u")
	desktop, docs, dls := filepath.Join(home, "Desktop"), filepath.Join(home, "Documents"), filepath.Join(home, "Downloads")
	cases := []struct {
		path, want string // want: the TCC folder, "" for outside
	}{
		{"/Users/u/Desktop", desktop},
		{"/Users/u/Desktop/horus", desktop},
		{"/Users/u/Desktop/horus/.scratch/outputs/", desktop},
		{"/Users/u/desktop/horus", desktop},
		{"/Users/u/Documents/models/vae", docs},
		{"/Users/u/Downloads/x/../horus", dls},
		{"/Users/u/DesktopFiles/horus", ""},
		{"/Users/u/horus", ""},
		{"/Users/u", ""},
		{"/Users/other/Desktop/horus", ""},
		{"/Users/u/Desktop/../horus", ""},
		{"", ""},
	}
	for _, c := range cases {
		p := filepath.FromSlash(c.path)
		got, ok := inTCCFolder("darwin", home, p)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("darwin %q: got %q %v, want %q", c.path, got, ok, c.want)
		}
		for _, goos := range []string{"windows", "linux"} {
			if f, ok := inTCCFolder(goos, home, p); ok || f != "" {
				t.Errorf("%s %q: got %q %v, want outside", goos, c.path, f, ok)
			}
		}
	}
	if _, ok := inTCCFolder("darwin", "", filepath.FromSlash("/Desktop/x")); ok {
		t.Error("an empty home folder has no TCC folder")
	}
	if _, ok := inTCCFolder("darwin", home, defaultComfyDir("darwin", home)); ok {
		t.Error("the default install folder of macOS is inside a TCC folder")
	}
	p := filepath.Join(desktop, "horus")
	f, ok := InTCCFolder(home, p)
	if wf, wok := inTCCFolder(runtime.GOOS, home, p); f != wf || ok != wok {
		t.Errorf("InTCCFolder = %q %v, want the answer of %s: %q %v", f, ok, runtime.GOOS, wf, wok)
	}
}

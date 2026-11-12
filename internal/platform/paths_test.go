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

package platform

import (
	"path/filepath"
	"runtime"
	"testing"
)

// (Moved from internal/scripts/installcomfymcp.)
func TestVenvPathsFollowTheOS(t *testing.T) {
	want := filepath.Join("v", "bin", "python")
	if runtime.GOOS == "windows" {
		want = filepath.Join("v", "Scripts", "python.exe")
	}
	if got := VenvBin("v", "python"); got != want {
		t.Errorf("VenvBin = %s, want %s", got, want)
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

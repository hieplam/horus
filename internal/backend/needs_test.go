package backend

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hieplam/horus/internal/platform"
)

// TestSetupNeedsOfEachOS: the package of each OS has the command of the spec
// (docs/specs/2026-11-15-release-install.md, "The setup plan"). Windows:
// 7-Zip with winget, in the 7-Zip folder of %ProgramFiles%. macOS: Python
// 3.12 with brew, in the bin folder of Homebrew, and the install needs
// comfy. Another OS needs nothing.
func TestSetupNeedsOfEachOS(t *testing.T) {
	pf, pf86 := filepath.Join(t.TempDir(), "pf"), filepath.Join(t.TempDir(), "pf86")
	t.Setenv("ProgramFiles", pf)
	t.Setenv("ProgramFiles(x86)", pf86)

	w := SetupNeeds("windows")
	if p := w.Package; w.ComfyCLI || p == nil || p.Name != "7-Zip" || p.Program != "7z" || p.Manager != "winget" ||
		!slices.Equal(p.Args, []string{"install", "--id", "7zip.7zip", "-e", "--accept-source-agreements", "--accept-package-agreements"}) ||
		!slices.Equal(p.Dirs, []string{filepath.Join(pf, "7-Zip"), filepath.Join(pf86, "7-Zip")}) || p.InManagerDir || p.URL == "" {
		t.Errorf("windows: %+v %+v", w, w.Package)
	}
	m := SetupNeeds("darwin")
	if p := m.Package; !m.ComfyCLI || p == nil || p.Name != "Python 3.12" || p.Program != "python3.12" || p.Manager != "brew" ||
		!slices.Equal(p.Args, []string{"install", "python@3.12"}) || !p.InManagerDir || len(p.Dirs) != 0 || p.URL == "" {
		t.Errorf("darwin: %+v %+v", m, m.Package)
	}
	if l := SetupNeeds("linux"); l.Package != nil || l.ComfyCLI {
		t.Errorf("linux: %+v", l)
	}
}

// TestFindSevenZipLooksInTheFolderOfWinget: 7z.exe that is not on the PATH
// is found in the 7-Zip folder of %ProgramFiles%, where winget puts it. So
// the install finds it after the package step of horus setup.
func TestFindSevenZipLooksInTheFolderOfWinget(t *testing.T) {
	pf := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	t.Setenv("ProgramFiles", pf)
	t.Setenv("ProgramFiles(x86)", "")
	if got := findSevenZip(); got != "" {
		t.Fatalf("found %q before the install", got)
	}
	seven := filepath.Join(pf, "7-Zip", "7z.exe")
	if err := os.MkdirAll(filepath.Dir(seven), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seven, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findSevenZip(); got != seven || !platform.IsFile(got) {
		t.Fatalf("found %q, want %q", got, seven)
	}
}

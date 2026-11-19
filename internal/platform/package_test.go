package platform

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestSpeechPackagesOfEachOS: winget on Windows with the folders of the
// current user, Homebrew on macOS, and no package on another OS.
func TestSpeechPackagesOfEachOS(t *testing.T) {
	env := map[string]string{"LOCALAPPDATA": `D:\Data\Local`, "ProgramFiles": `D:\Programs`}
	win := speechPackages("windows", func(k string) string { return env[k] })
	if py := win.Python; py == nil || py.Manager != "winget" || py.Exe != "python.exe" ||
		strings.Join(py.Args, " ") != "install --id Python.Python.3.12 -e --accept-source-agreements --accept-package-agreements" ||
		!slices.Equal(py.Dirs, []string{filepath.Join(env["LOCALAPPDATA"], "Programs", "Python", "Python312"), filepath.Join(env["ProgramFiles"], "Python312")}) {
		t.Errorf("Windows Python: %+v", win.Python)
	}
	if ff := win.FFmpeg; ff == nil || ff.Program != "ffmpeg" || strings.Join(ff.Args, " ") != "install --id Gyan.FFmpeg -e --accept-source-agreements --accept-package-agreements" ||
		len(ff.Dirs) != 2 || !strings.HasSuffix(ff.Dirs[1], filepath.Join("Gyan.FFmpeg_*", "ffmpeg-*", "bin")) {
		t.Errorf("Windows FFmpeg: %+v", win.FFmpeg)
	}
	mac := speechPackages("darwin", func(string) string { return "" })
	if mac.Python == nil || mac.Python.Manager != "brew" || strings.Join(mac.Python.Args, " ") != "install python@3.12" || !mac.Python.InManagerDir ||
		mac.FFmpeg == nil || strings.Join(mac.FFmpeg.Args, " ") != "install ffmpeg" || !mac.FFmpeg.InManagerDir {
		t.Errorf("macOS: %+v %+v", mac.Python, mac.FFmpeg)
	}
	if other := speechPackages("linux", func(string) string { return "" }); other.Python != nil || other.FFmpeg != nil {
		t.Errorf("Linux: %+v", other)
	}
}

// TestAddToPathPutsTheFolderFirstOneTime.
func TestAddToPathPutsTheFolderFirstOneTime(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	t.Setenv("PATH", a)
	AddToPath(b)
	AddToPath(b)
	if got := filepath.SplitList(os.Getenv("PATH")); !slices.Equal(got, []string{b, a}) {
		t.Errorf("PATH %v, want [%s %s]", got, b, a)
	}
}

// TestInstallRecordKeepsThePackages: the record of a venv lists the packages
// of its pip install; a record without packages has none.
func TestInstallRecordKeepsThePackages(t *testing.T) {
	dir := t.TempDir()
	if _, ok := InstallRecordPackages(dir, RecordSpeechVenv); ok {
		t.Fatal("a folder without a record has one")
	}
	if err := WriteInstallRecord(dir, RecordSpeechVenv); err != nil {
		t.Fatal(err)
	}
	if got, ok := InstallRecordPackages(dir, RecordSpeechVenv); !ok || len(got) != 0 || !HasInstallRecord(dir, RecordSpeechVenv) || HasInstallRecord(dir, RecordComfyMCP) {
		t.Errorf("a record without packages: %v %v", got, ok)
	}
	if err := WriteInstallRecord(dir, RecordSpeechVenv, "a==1", "b==2"); err != nil {
		t.Fatal(err)
	}
	if got, ok := InstallRecordPackages(dir, RecordSpeechVenv); !ok || !slices.Equal(got, []string{"a==1", "b==2"}) {
		t.Errorf("packages %v %v", got, ok)
	}
}

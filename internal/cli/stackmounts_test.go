package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/settings"
)

// TestStackOverride: one bind mount of each pair into horus-api and
// horus-worker, the settings of the shared folders, and $ as $$.
func TestStackOverride(t *testing.T) {
	got := StackOverride([]settings.SharedDir{{Host: `C:\Users\me\Pictures\horus`, Container: "/shared/pictures"},
		{Host: `D:\a$b`, Container: "/shared/ab"}}, "windows")
	want := `services:
  horus-api:
    environment:
      HORUS_SHARED_DIRS: "C:\\Users\\me\\Pictures\\horus=/shared/pictures;D:\\a$$b=/shared/ab"
      HORUS_OUTPUT_DIR: "/shared/pictures"
      HORUS_HOST_OS: "windows"
    volumes:
      - type: bind
        source: "C:\\Users\\me\\Pictures\\horus"
        target: "/shared/pictures"
      - type: bind
        source: "D:\\a$$b"
        target: "/shared/ab"
  horus-worker:
    environment:
      HORUS_SHARED_DIRS: "C:\\Users\\me\\Pictures\\horus=/shared/pictures;D:\\a$$b=/shared/ab"
      HORUS_OUTPUT_DIR: "/shared/pictures"
      HORUS_HOST_OS: "windows"
    volumes:
      - type: bind
        source: "C:\\Users\\me\\Pictures\\horus"
        target: "/shared/pictures"
      - type: bind
        source: "D:\\a$$b"
        target: "/shared/ab"
`
	if !strings.HasSuffix(got, want) || !strings.HasPrefix(got, "# ") {
		t.Fatalf("override:\n%s\nwant the comment, then:\n%s", got, want)
	}
}

// TestStackMountsWritesTheFile: horus stack-mounts makes the missing host
// folder, and writes the pairs of HORUS_SHARED_DIRS. Without the setting,
// the one pair is the output folder of horus. A relative host folder is
// refused.
func TestStackMountsWritesTheFile(t *testing.T) {
	dir := t.TempDir()
	shared := filepath.Join(dir, "new", "shared")
	t.Setenv("HORUS_SHARED_DIRS", shared+"=/shared/test")
	settings.Reset()
	t.Cleanup(settings.Reset)
	file := filepath.Join(dir, "override.yaml")
	var out, errOut bytes.Buffer
	if code := StackMounts([]string{"-o", file}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if fi, err := os.Stat(shared); err != nil || !fi.IsDir() {
		t.Errorf("the host folder was not made: %v", err)
	}
	data, err := os.ReadFile(file)
	if err != nil || !strings.Contains(string(data), `target: "/shared/test"`) || !strings.Contains(out.String(), "/shared/test") {
		t.Errorf("file %s %v, output %s", data, err, out.String())
	}

	t.Setenv("HORUS_SHARED_DIRS", "")
	t.Setenv("HORUS_OUTPUT_DIR", filepath.Join(dir, "outputs"))
	settings.Reset()
	if code := StackMounts([]string{"-o", file}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	data, _ = os.ReadFile(file)
	if !strings.Contains(string(data), `target: "/shared/outputs"`) || !strings.Contains(string(data), "outputs=/shared/outputs") {
		t.Errorf("the default pair: %s", data)
	}

	t.Setenv("HORUS_SHARED_DIRS", `relative\folder=/shared/x`)
	settings.Reset()
	if code := StackMounts([]string{"-o", file}, &out, &errOut); code != 1 {
		t.Errorf("a relative host folder: exit %d", code)
	}
}

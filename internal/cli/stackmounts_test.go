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
	got := StackOverride([]settings.SharedDir{{Host: `C:\Users\you\Pictures\horus`, Container: "/shared/pictures"},
		{Host: `D:\a$b`, Container: "/shared/ab"}}, "", "windows")
	want := `services:
  horus-api:
    environment:
      HORUS_SHARED_DIRS: "C:\\Users\\you\\Pictures\\horus=/shared/pictures;D:\\a$$b=/shared/ab"
      HORUS_OUTPUT_DIR: "/shared/pictures"
      HORUS_HOST_OS: "windows"
    volumes:
      - type: bind
        source: "C:\\Users\\you\\Pictures\\horus"
        target: "/shared/pictures"
      - type: bind
        source: "D:\\a$$b"
        target: "/shared/ab"
  horus-worker:
    environment:
      HORUS_SHARED_DIRS: "C:\\Users\\you\\Pictures\\horus=/shared/pictures;D:\\a$$b=/shared/ab"
      HORUS_OUTPUT_DIR: "/shared/pictures"
      HORUS_HOST_OS: "windows"
    volumes:
      - type: bind
        source: "C:\\Users\\you\\Pictures\\horus"
        target: "/shared/pictures"
      - type: bind
        source: "D:\\a$$b"
        target: "/shared/ab"
`
	if !strings.HasSuffix(got, want) || !strings.HasPrefix(got, "# ") {
		t.Fatalf("override:\n%s\nwant the comment, then:\n%s", got, want)
	}
}

// TestStackOverrideWithModels: with a models folder, horus-api also gets a
// bind mount of it at /models, HORUS_MODELS_DIR (the container folder) and
// HORUS_MODELS_HOST_DIR (the host folder). horus-worker gets none of them.
func TestStackOverrideWithModels(t *testing.T) {
	got := StackOverride([]settings.SharedDir{{Host: `C:\out`, Container: "/shared/out"}}, `C:\Users\you\ComfyUI-models`, "windows")
	want := `services:
  horus-api:
    environment:
      HORUS_SHARED_DIRS: "C:\\out=/shared/out"
      HORUS_OUTPUT_DIR: "/shared/out"
      HORUS_HOST_OS: "windows"
      HORUS_MODELS_DIR: "/models"
      HORUS_MODELS_HOST_DIR: "C:\\Users\\you\\ComfyUI-models"
    volumes:
      - type: bind
        source: "C:\\out"
        target: "/shared/out"
      - type: bind
        source: "C:\\Users\\you\\ComfyUI-models"
        target: "/models"
  horus-worker:
    environment:
      HORUS_SHARED_DIRS: "C:\\out=/shared/out"
      HORUS_OUTPUT_DIR: "/shared/out"
      HORUS_HOST_OS: "windows"
    volumes:
      - type: bind
        source: "C:\\out"
        target: "/shared/out"
`
	if !strings.HasSuffix(got, want) {
		t.Fatalf("override:\n%s\nwant:\n%s", got, want)
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
	t.Setenv("HORUS_MODELS_DIR", "")
	t.Setenv("HORUS_ROOT", dir) // no .env of a clone
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

	if strings.Contains(string(data), "/models") || !strings.Contains(out.String(), "HORUS_MODELS_DIR is not set") {
		t.Errorf("no HORUS_MODELS_DIR: the file has a models folder, or the output does not say why it has none: %s\n%s", data, out.String())
	}

	models := filepath.Join(dir, "new", "models")
	t.Setenv("HORUS_MODELS_DIR", models)
	settings.Reset()
	out.Reset()
	if code := StackMounts([]string{"-o", file}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	data, _ = os.ReadFile(file)
	if fi, err := os.Stat(models); err != nil || !fi.IsDir() {
		t.Errorf("the models folder was not made: %v", err)
	}
	if !strings.Contains(string(data), `target: "/models"`) || !strings.Contains(out.String(), models) {
		t.Errorf("HORUS_MODELS_DIR: file %s, output %s", data, out.String())
	}

	t.Setenv("HORUS_MODELS_DIR", `relative\models`)
	settings.Reset()
	if code := StackMounts([]string{"-o", file}, &out, &errOut); code != 1 {
		t.Errorf("a relative models folder: exit %d", code)
	}
	t.Setenv("HORUS_MODELS_DIR", "")

	t.Setenv("HORUS_SHARED_DIRS", `relative\folder=/shared/x`)
	settings.Reset()
	if code := StackMounts([]string{"-o", file}, &out, &errOut); code != 1 {
		t.Errorf("a relative host folder: exit %d", code)
	}
}

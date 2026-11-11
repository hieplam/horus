package backend

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

func useSettings(t *testing.T, env map[string]string) {
	t.Helper()
	t.Setenv("HORUS_ROOT", t.TempDir()) // no .env file
	for _, k := range []string{"HORUS_COMFYUI_DIR", "HORUS_COMFYUI_URL"} {
		t.Setenv(k, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	settings.Reset()
	t.Cleanup(settings.Reset)
}

// TestOtherOSAnswersNotSupported: off Windows, Install, Start and Stop answer
// "not supported on this OS yet" and touch nothing. Installed and Layout
// still give the folders.
func TestOtherOSAnswersNotSupported(t *testing.T) {
	dir := t.TempDir()
	for _, goos := range []string{"linux", "darwin"} {
		b := newFor(goos, &Portable{Dir: dir, URL: freeURL(t)})
		for name, call := range map[string]func(context.Context, io.Writer) error{"Install": b.Install, "Start": b.Start, "Stop": b.Stop} {
			if err := call(context.Background(), io.Discard); !errors.Is(err, platform.ErrNotSupported) || err.Error() != "not supported on this OS yet" {
				t.Errorf("%s: %s = %v", goos, name, err)
			}
		}
		if d, ok := b.Installed(); d != dir || ok {
			t.Errorf("%s: Installed = %s %v", goos, d, ok)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("%s: the calls wrote %d files", goos, len(entries))
		}
	}
	if _, ok := newFor("windows", &Portable{Dir: dir}).(*Portable); !ok {
		t.Error("on Windows, New does not give the portable backend")
	}
}

// TestInstalledAndLayout: ComfyUI is installed when ComfyUI/main.py is a
// file. The layout of the portable build. The folder follows
// HORUS_COMFYUI_DIR when Dir is empty.
func TestInstalledAndLayout(t *testing.T) {
	dir := t.TempDir()
	useSettings(t, map[string]string{"HORUS_COMFYUI_DIR": dir})
	p := &Portable{}
	if d, ok := p.Installed(); d != dir || ok {
		t.Errorf("without main.py: %s %v", d, ok)
	}
	os.MkdirAll(filepath.Join(dir, "ComfyUI"), 0o755)
	os.WriteFile(filepath.Join(dir, "ComfyUI", "main.py"), []byte("#"), 0o644)
	if d, ok := p.Installed(); d != dir || !ok {
		t.Errorf("with main.py: %s %v", d, ok)
	}
	l := p.Layout()
	want := Layout{Dir: dir, ComfyUI: filepath.Join(dir, "ComfyUI"), Python: filepath.Join(dir, "python_embeded", "python.exe"),
		Logs: []string{filepath.Join(dir, "horus-comfyui.out.log"), filepath.Join(dir, "horus-comfyui.err.log")}}
	if l.Dir != want.Dir || l.ComfyUI != want.ComfyUI || l.Python != want.Python || !equal(l.Logs, want.Logs) {
		t.Errorf("Layout = %+v, want %+v", l, want)
	}
}

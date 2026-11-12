// The image path on macOS through the wiring of cmd/horus: the MCP server,
// the image service, the ComfyUI adapter and the comfy-cli backend, with the
// fake comfy of comfyclitest and the fake ComfyUI of comfytest. It runs on
// every OS, because the backend runs comfy as a separate program.
package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/backend/comfyclitest"
	"github.com/hieplam/horus/internal/mcpserver"
	"github.com/hieplam/horus/internal/settings"
)

// freePort gives a free port on 127.0.0.1 that the OS picks. It is never
// 8188 or 8189.
func freePort(t *testing.T) string {
	t.Helper()
	for {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		_, port, _ := net.SplitHostPort(l.Addr().String())
		l.Close()
		if port != "8188" && port != "8189" {
			return port
		}
	}
}

// TestGenerateStartsComfyUIThroughTheComfyCLIBackend: ComfyUI does not
// answer at HORUS_COMFYUI_URL. generate_image starts it through the
// comfy-cli backend: the fake comfy gets exactly the launch of the spec for
// the workspace of HORUS_COMFYUI_DIR and the port of the address, its
// stand-in ComfyUI sends the job to the fake ComfyUI, and the result has a
// PNG and started_backend true.
func TestGenerateStartsComfyUIThroughTheComfyCLIBackend(t *testing.T) {
	f := imageEnv(t)
	tmp := t.TempDir()
	dir, bin := filepath.Join(tmp, "horus"), filepath.Join(tmp, "bin")
	ui := filepath.Join(dir, "ComfyUI")
	os.MkdirAll(ui, 0o755)
	os.WriteFile(filepath.Join(ui, "main.py"), []byte("# fake\n"), 0o644)
	port := freePort(t)
	down := "http://127.0.0.1:" + port
	t.Setenv("HORUS_COMFYUI_URL", down)
	t.Setenv("HORUS_COMFYUI_DIR", dir)
	t.Setenv("HORUS_AUTOSTART", "1")
	t.Setenv("PATH", bin) // only the fake comfy: never the real one
	settings.Reset()
	fake := comfyclitest.New(t)
	fake.Put(t, bin)
	fake.Proxy(t, f.URL())

	svc := imagesFor("", seams{goos: "darwin"})
	cs := connect(t, mcpserver.New(svc, nil), nil)
	out := filepath.Join(tmp, "images", "a.png")
	res := callTool(t, cs, "generate_image", map[string]any{"workflow": "it-illustration", "subject": testSubject, "out_path": out})
	env := envelopeOf(t, res)
	r, _ := env["result"].(map[string]any)
	if res.IsError || env["ok"] != true || r["path"] != out || r["started_backend"] != true {
		t.Fatalf("got %v", env)
	}
	want := "--workspace " + ui + " --json launch --background -- --listen 127.0.0.1 --port " + port + "\nstdin closed\n"
	if got := fake.Recorded(); got != want {
		t.Errorf("the fake comfy got %q, want %q", got, want)
	}
	if jobs := f.Jobs(); len(jobs) != 1 {
		t.Errorf("the fake ComfyUI got %d jobs, want 1", len(jobs))
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.HasPrefix(string(data), "\x89PNG\r\n\x1a\n") {
		t.Fatalf("no PNG: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "horus-comfyui.state.json")); err != nil || !strings.Contains(string(b), down) {
		t.Errorf("no state file of the start at %s: %v %s", down, err, b)
	}
}

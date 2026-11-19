// Tests of the tool generate_speech through the wiring: the speak service,
// the glue, the adapter internal/speechworker and the fake speech worker
// (internal/speechworker/fakeworker), built as the Python of a speech venv.
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/mcpserver"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
	"github.com/hieplam/horus/internal/speechsetup"
)

// speechEnv makes a workflows folder with the stand-in speech workflow, a
// speech folder whose venv Python is the fake speech worker, and a PATH with
// the programs ffmpeg and ffprobe (empty files: the fake worker runs neither).
// It gives the place of the speech setup and the record file of the fake
// worker.
func speechEnv(t *testing.T, mode string) (speechsetup.Place, string) {
	t.Helper()
	root := t.TempDir()
	flows := filepath.Join(root, "workflows", "stand-in-speech")
	if err := os.MkdirAll(flows, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "stand-in-speech", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flows, "manifest.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	place := speechsetup.At(filepath.Join(root, "speech"))
	build := exec.Command("go", "build", "-o", place.Python, "../../internal/speechworker/fakeworker")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the fake speech worker: %v\n%s", err, out)
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range speechsetup.Programs {
		if err := os.WriteFile(filepath.Join(bin, platform.ExeName(name)), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	record := filepath.Join(root, "worker.record")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HORUS_ROOT", root)
	t.Setenv("HORUS_WORKFLOWS_DIR", filepath.Join(root, "workflows"))
	t.Setenv("HORUS_SPEECH_DIR", place.Dir)
	t.Setenv("HORUS_OUTPUT_DIR", filepath.Join(root, "out"))
	t.Setenv("HORUS_FAKE_SPEECH", mode)
	t.Setenv("HORUS_FAKE_SPEECH_RECORD", record)
	settings.Reset()
	t.Cleanup(settings.Reset)
	return place, record
}

func TestGenerateSpeechThroughTheWiring(t *testing.T) {
	place, record := speechEnv(t, "")
	life, cancel := context.WithCancel(context.Background())
	defer cancel()
	speech := newSpeech(life)
	cs := connect(t, mcpserver.New(nil, nil, speech), nil)
	text := "Xin chào  [laugh] các bạn."
	env := envelopeOf(t, callTool(t, cs, "generate_speech", map[string]any{"workflow": "stand-in-speech", "text": text, "voice": "beta"}))
	r, _ := env["result"].(map[string]any)
	if env["ok"] != true || r["transcript"] != text || r["voice"] != "beta" || r["format"] != "wav" || r["started_worker"] != true {
		t.Fatalf("first call: %v", env)
	}
	path, _ := r["path"].(string)
	if data, err := os.ReadFile(path); err != nil || string(data[:4]) != "RIFF" || int64(len(data)) != int64(r["bytes"].(float64)) {
		t.Fatalf("the audio file %s: %v", path, err)
	}
	env = envelopeOf(t, callTool(t, cs, "generate_speech", map[string]any{"workflow": "stand-in-speech", "text": "hai"}))
	if r, _ := env["result"].(map[string]any); env["ok"] != true || r["started_worker"] != false || r["voice"] != "alpha" {
		t.Fatalf("second call: %v", env)
	}
	if script, err := os.ReadFile(place.Script); err != nil || !strings.Contains(string(script), "speech worker of horus") {
		t.Fatalf("the worker file %s: %v", place.Script, err)
	}
	if err := speech.Close(); err != nil {
		t.Fatal(err)
	}
	lines, _ := os.ReadFile(record)
	if n := strings.Count(string(lines), "start "); n != 1 || !strings.Contains(string(lines), "--models "+place.Models) {
		t.Fatalf("the fake worker started %d times, want 1, with --models %s:\n%s", n, place.Models, lines)
	}
}

func TestGenerateSpeechWithAWorkerThatStops(t *testing.T) {
	for _, mode := range []string{"die", "error", "load"} {
		t.Run(mode, func(t *testing.T) {
			speechEnv(t, mode)
			life, cancel := context.WithCancel(context.Background())
			defer cancel()
			speech := newSpeech(life)
			defer speech.Close()
			cs := connect(t, mcpserver.New(nil, nil, speech), nil)
			out := filepath.Join(t.TempDir(), "a.wav")
			env := envelopeOf(t, callTool(t, cs, "generate_speech", map[string]any{"workflow": "stand-in-speech", "text": "xin chào", "out_path": out}))
			e, _ := env["error"].(map[string]any)
			details, _ := e["details"].(map[string]any)
			if e["code"] != "GENERATION_FAILED" || e["action_by"] != "retry" || details["worker_message"] == "" {
				t.Fatalf("mode %s: %v", mode, env)
			}
			if _, err := os.Stat(out); err == nil {
				t.Fatalf("mode %s: a failed call left the file %s", mode, out)
			}
		})
	}
}

func TestGenerateSpeechWithoutTheVenv(t *testing.T) {
	place, _ := speechEnv(t, "")
	if err := os.Remove(place.Python); err != nil {
		t.Fatal(err)
	}
	cs := connect(t, mcpserver.New(nil, nil, newSpeech(context.Background())), nil)
	env := envelopeOf(t, callTool(t, cs, "generate_speech", map[string]any{"workflow": "stand-in-speech", "text": "xin chào"}))
	e, _ := env["error"].(map[string]any)
	if e["code"] != "WORKFLOW_NOT_INSTALLED" || !strings.Contains(e["next_step"].(string), "install_workflow") {
		t.Fatalf("got %v", env)
	}
}

package provision

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hieplam/horus/internal/settings"
	"github.com/hieplam/horus/internal/speechsetup"
)

var speechData = []byte("stand-in speech model")

// speechRig is a workflows folder with the image workflow "demo" and the
// speech workflow "talk" (one model file in the folder "tts"), and a speech
// folder in memory with each part of the speech setup.
type speechRig struct {
	dir    string
	files  *memFiles
	progs  *fakePrograms
	model  string // the path of the model file in the speech folder
	svc    *Service
	venvPy string
}

func newSpeechRig(t *testing.T) *speechRig {
	t.Helper()
	dir := workflows(t)
	talk := filepath.Join(dir, "talk")
	if err := os.MkdirAll(talk, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest, _ := json.Marshal(map[string]any{"kind": "speech"})
	models, _ := json.Marshal(map[string]any{"repo": "org/speech", "license": "test", "files": []any{
		map[string]any{"role": "speech", "repo_path": "model.bin", "folder": "tts", "bytes": len(speechData), "sha256": sum(speechData)}}})
	for name, b := range map[string][]byte{"manifest.json": manifest, "models.json": models} {
		if err := os.WriteFile(filepath.Join(talk, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HORUS_SPEECH_DIR", filepath.Join(t.TempDir(), "speech"))
	settings.Reset()
	t.Cleanup(settings.Reset)
	place := speechsetup.Here()
	files := newMemFiles()
	r := &speechRig{dir: dir, files: files, model: filepath.Join(place.Models, "tts", "model.bin"), venvPy: place.Python}
	files.files[r.model] = speechData
	files.files[place.Python] = []byte("python")
	r.progs = &fakePrograms{paths: map[string]string{"ffmpeg": "/bin/ffmpeg", "ffprobe": "/bin/ffprobe"}, log: &[]string{}}
	r.svc = New(files, nil, nil, files, "").WithSystem(System{Programs: r.progs})
	return r
}

func (r *speechRig) state(t *testing.T) string {
	t.Helper()
	rows, err := r.svc.List(context.Background(), r.dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := range rows {
		if rows[i].Name == "talk" {
			return rows[i].State()
		}
	}
	t.Fatal("no row for talk")
	return ""
}

func TestSpeechWorkflowIsInstalledWhenEachPartIsInTheSpeechFolder(t *testing.T) {
	r := newSpeechRig(t)
	if got := r.state(t); got != "installed" {
		t.Errorf("List: %q", got)
	}
	res, err := r.svc.Check(context.Background(), Request{WorkflowsDir: r.dir, Workflow: "talk"})
	if err != nil {
		t.Fatalf("Check: %v (%s)", err, statuses(res))
	}
	if want := "OK venv , OK program , OK program , OK file speech"; statuses(res) != want {
		t.Errorf("items %q, want %q", statuses(res), want)
	}
}

func TestSpeechWorkflowIsNotInstalledWhenAPartIsMissing(t *testing.T) {
	r := newSpeechRig(t)
	delete(r.files.files, r.model)
	if got := r.state(t); got != "partially_installed" {
		t.Errorf("no model file: %q", got)
	}
	res, err := r.svc.Check(context.Background(), Request{WorkflowsDir: r.dir, Workflow: "talk", Quick: true})
	if err == nil || statuses(res) != "OK venv , OK program , OK program , MISSING file speech" {
		t.Errorf("no model file: err %v, items %q", err, statuses(res))
	}

	r = newSpeechRig(t)
	delete(r.files.files, r.venvPy)
	delete(r.progs.paths, "ffmpeg")
	delete(r.files.files, r.model)
	delete(r.progs.paths, "ffprobe")
	if got := r.state(t); got != "not_installed" {
		t.Errorf("no part: %q", got)
	}

	r = newSpeechRig(t)
	delete(r.progs.paths, "ffmpeg")
	if got := r.state(t); got != "partially_installed" {
		t.Errorf("no ffmpeg: %q", got)
	}
}

func TestSpeechWorkflowChecksTheSizeOfTheModelFile(t *testing.T) {
	r := newSpeechRig(t)
	r.files.files[r.model] = []byte("short")
	if got := r.state(t); got != "partially_installed" {
		t.Errorf("wrong size: %q", got)
	}
}

func TestSpeechWorkflowIgnoresTheModelFoldersOfComfyUI(t *testing.T) {
	r := newSpeechRig(t)
	delete(r.files.files, r.model)
	r.files.files[filepath.Join(r.files.root, "tts", "model.bin")] = speechData // a file in the ComfyUI folder
	if got := r.state(t); got != "partially_installed" {
		t.Errorf("a file in the folder of ComfyUI counts: %q", got)
	}
}

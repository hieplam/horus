package speechsetup

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

func TestHereIsInTheUserFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HORUS_HOME", home)
	t.Setenv("HORUS_SPEECH_DIR", "")
	t.Setenv("HORUS_ROOT", t.TempDir()) // a repo root without .env
	settings.Reset()
	t.Cleanup(settings.Reset)
	dir := filepath.Join(home, "speech")
	want := Place{
		Dir:    dir,
		Venv:   filepath.Join(dir, "venv"),
		Python: platform.VenvBin(filepath.Join(dir, "venv"), "python"),
		Models: filepath.Join(dir, "models"),
		Script: filepath.Join(dir, "worker.py"),
	}
	if got := Here(); got != want {
		t.Fatalf("Here() = %+v, want %+v", got, want)
	}
	other := t.TempDir()
	t.Setenv("HORUS_SPEECH_DIR", other)
	if got := Here(); got != At(other) {
		t.Fatalf("with HORUS_SPEECH_DIR: %+v, want %+v", got, At(other))
	}
}

// speechWorkflow writes a speech workflow with a models.json of two files.
func speechWorkflow(t *testing.T) *catalog.Workflow {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "talk")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	models := `{"repo": "x/y", "files": [
	  {"role": "speech", "repo_path": "onnx_update/config.json", "folder": "vieneu/onnx_update", "bytes": 4, "sha256": "0"},
	  {"role": "recognition", "repo_path": "model.bin", "folder": "whisper-small", "bytes": 7, "sha256": "0"}]}`
	if err := os.WriteFile(filepath.Join(dir, "models.json"), []byte(models), 0o644); err != nil {
		t.Fatal(err)
	}
	return &catalog.Workflow{Name: "talk", Dir: dir, Manifest: catalog.NewObject()}
}

func TestPartsInTheOrderOfTheSetupPlan(t *testing.T) {
	p := At(filepath.Join("s", "speech"))
	parts, err := Parts(speechWorkflow(t), p)
	if err != nil {
		t.Fatal(err)
	}
	want := []Part{
		{Kind: KindVenv, Name: "speech venv", Path: p.Python},
		{Kind: KindProgram, Name: "ffmpeg"},
		{Kind: KindProgram, Name: "ffprobe"},
		{Kind: KindModelFile, Name: "config.json", Path: filepath.Join(p.Models, "vieneu", "onnx_update", "config.json"), Bytes: 4},
		{Kind: KindModelFile, Name: "model.bin", Path: filepath.Join(p.Models, "whisper-small", "model.bin"), Bytes: 7},
	}
	if !reflect.DeepEqual(parts, want) {
		t.Fatalf("Parts:\n got %+v\nwant %+v", parts, want)
	}
}

func TestPartsOfAWorkflowWithoutModelsJSON(t *testing.T) {
	w := &catalog.Workflow{Name: "bare", Dir: t.TempDir(), Manifest: catalog.NewObject()}
	parts, err := Parts(w, At("s"))
	if err != nil || len(parts) != 3 {
		t.Fatalf("Parts = %+v, %v: want the venv and the 2 programs", parts, err)
	}
}

func TestMissing(t *testing.T) {
	p := At("s")
	parts, err := Parts(speechWorkflow(t), p)
	if err != nil {
		t.Fatal(err)
	}
	names := func(ps []Part) []string {
		out := []string{}
		for _, x := range ps {
			out = append(out, x.Name)
		}
		return out
	}
	sizes := map[string]int64{p.Python: 100, parts[3].Path: 4, parts[4].Path: 7}
	size := func(path string) int64 {
		if n, ok := sizes[path]; ok {
			return n
		}
		return -1
	}
	all := func(string) bool { return true }
	if got := Missing(parts, size, all); len(got) != 0 {
		t.Fatalf("everything is there, but Missing = %v", names(got))
	}
	none := func(string) bool { return false }
	if got := names(Missing(parts, size, none)); !reflect.DeepEqual(got, []string{"ffmpeg", "ffprobe"}) {
		t.Fatalf("no programs: Missing = %v", got)
	}
	sizes[parts[4].Path] = 6 // a model file with another size
	delete(sizes, p.Python)
	if got := names(Missing(parts, size, all)); !reflect.DeepEqual(got, []string{"speech venv", "model.bin"}) {
		t.Fatalf("no venv, a short file: Missing = %v", got)
	}
}

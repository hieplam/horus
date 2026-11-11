package modelfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/settings"
)

func TestYAMLFolders(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "extra_model_paths.yaml")
	abs := filepath.Join(dir, "abs")
	text := "# comment\ncomfyui:\n    base_path: '" + dir + "'\n    is_default: true\n    vae: models/vae  # one\n" +
		"    loras: |\n        models/loras\n        \"" + abs + "\"\n\n" +
		"other:\n    vae: relative-without-base\n"
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	got := yamlFolders(p)
	want := map[string][]string{
		"vae":   {fullPath(filepath.Join(dir, "models/vae")), fullPath("relative-without-base")},
		"loras": {fullPath(filepath.Join(dir, "models/loras")), fullPath(abs)},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

// store makes a Store on a ComfyUI folder in a temporary folder, with no
// setting of a real machine.
func store(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HORUS_ROOT", dir)
	for _, k := range []string{"HORUS_MODELS_DIR", "HORUS_TEXT_ENCODER_DIR", "HORUS_COMFYUI_DIR"} {
		t.Setenv(k, "")
	}
	settings.Reset()
	t.Cleanup(settings.Reset)
	comfy := filepath.Join(dir, "comfy")
	if err := os.MkdirAll(filepath.Join(comfy, "ComfyUI"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Store{ComfyUIDir: comfy, Now: func() time.Time { return time.Date(2026, 11, 10, 9, 8, 7, 0, time.UTC) }}, dir
}

// TestDirsInTheOrderOfTheSpec: the text encoder folder first (for
// text_encoders only), the models folder, ComfyUI's own folder, then the
// folders of extra_model_paths.yaml, each one time.
func TestDirsInTheOrderOfTheSpec(t *testing.T) {
	s, dir := store(t)
	own := filepath.Join(s.ComfyUIDir, "ComfyUI", "models")
	s.ModelsDir, s.TextEncoderDir = filepath.Join(dir, "models"), filepath.Join(dir, "te")
	yaml := "a:\n    base_path: " + dir + "\n    vae: extra/vae\n    text_encoders: te\n"
	if err := os.WriteFile(filepath.Join(s.ComfyUIDir, "ComfyUI", "extra_model_paths.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "models", "vae"), filepath.Join(own, "vae"), filepath.Join(dir, "extra", "vae")}
	if got := s.Dirs("vae"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("vae:\n got %v\nwant %v", got, want)
	}
	want = []string{filepath.Join(dir, "te"), filepath.Join(dir, "models", "text_encoders"), filepath.Join(own, "text_encoders")}
	if got := s.Dirs("text_encoders"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("text_encoders (the yaml folder is the text encoder folder, one time):\n got %v\nwant %v", got, want)
	}
	if s.Own("vae") != filepath.Join(own, "vae") || fmt.Sprint(s.Mapped("vae")) != fmt.Sprint([]string{filepath.Join(dir, "extra", "vae")}) {
		t.Errorf("own %s, mapped %v", s.Own("vae"), s.Mapped("vae"))
	}
	if _, ok := s.ComfyUI(); ok {
		t.Error("ComfyUI without main.py is not installed")
	}
	os.WriteFile(filepath.Join(s.ComfyUIDir, "ComfyUI", "main.py"), []byte("#"), 0o644)
	if d, ok := s.ComfyUI(); !ok || d != s.ComfyUIDir {
		t.Errorf("ComfyUI %s %v", d, ok)
	}
}

// TestMapBacksUpTheFileOneTime: one backup before the change, and one block
// for each pair.
func TestMapBacksUpTheFileOneTime(t *testing.T) {
	s, dir := store(t)
	yaml := filepath.Join(s.ComfyUIDir, "ComfyUI", "extra_model_paths.yaml")
	old := "\xef\xbb\xbfcomfyui:\n    base_path: somewhere\n"
	os.WriteFile(yaml, []byte(old), 0o644)
	if err := s.Map([][2]string{{"vae", filepath.Join(dir, "m", "vae")}, {"loras", filepath.Join(dir, "m", "loras")}}); err != nil {
		t.Fatal(err)
	}
	base := strings.ReplaceAll(filepath.Join(dir, "m"), `\`, "/")
	want := "comfyui:\n    base_path: somewhere\n\nhorus_vae:\n    base_path: " + base + "\n    vae: vae\n" +
		"\nhorus_loras:\n    base_path: " + base + "\n    loras: loras\n"
	if b, _ := os.ReadFile(yaml); string(b) != want {
		t.Errorf("file:\n%s\nwant:\n%s", b, want)
	}
	backups, _ := filepath.Glob(yaml + ".bak-*")
	if len(backups) != 1 || filepath.Base(backups[0]) != "extra_model_paths.yaml.bak-20261110-090807" {
		t.Fatalf("backups %v", backups)
	}
	if b, _ := os.ReadFile(backups[0]); string(b) != old {
		t.Errorf("the backup is not the old file: %q", b)
	}
	// No file yet: no backup.
	os.Remove(yaml)
	os.Remove(backups[0])
	if err := s.Map([][2]string{{"vae", filepath.Join(dir, "m", "vae")}}); err != nil {
		t.Fatal(err)
	}
	if b, _ := filepath.Glob(yaml + ".bak-*"); len(b) != 0 {
		t.Errorf("a backup of no file: %v", b)
	}
}

func TestSizeHashAndDelete(t *testing.T) {
	s, dir := store(t)
	p := filepath.Join(dir, "f.bin")
	if s.Size(p) != -1 || s.Size(dir) != -1 {
		t.Error("no file and a folder must be -1")
	}
	os.WriteFile(p, []byte("abc"), 0o444)
	if h, err := s.SHA256(p); err != nil || h != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" || s.Size(p) != 3 {
		t.Errorf("size %d, sha %s %v", s.Size(p), h, err)
	}
	if err := s.Delete(p); err != nil {
		t.Fatalf("delete of a read-only file: %v", err)
	}
	if s.Size(p) != -1 {
		t.Error("the file is still there")
	}
}

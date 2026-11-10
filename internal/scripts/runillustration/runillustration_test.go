package runillustration

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/comfy/comfytest"
	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/settings"
)

const subject = "On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt."

// setup copies the workflow it-illustration into a temporary workflows
// folder, starts a fake ComfyUI and makes the wait intervals short.
func setup(t *testing.T) (fake *comfytest.Fake, tmp string) {
	t.Helper()
	tmp = t.TempDir()
	dst := filepath.Join(tmp, "workflows", "it-illustration")
	os.MkdirAll(dst, 0o755)
	for _, name := range []string{"manifest.json", "workflow.json", "models.json"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "workflows", "it-illustration", name))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dst, name), data, 0o644)
	}
	t.Setenv("HORUS_ROOT", tmp)
	t.Setenv("HORUS_WORKFLOWS_DIR", filepath.Join(tmp, "workflows"))
	t.Setenv("HORUS_OUTPUT_DIR", filepath.Join(tmp, "out"))
	t.Setenv("HORUS_AUTOSTART", "0")
	settings.Reset()
	t.Cleanup(settings.Reset)
	oldPoll, oldReport, oldAbsent := generate.PollInterval, generate.ReportInterval, generate.AbsentLimit
	generate.PollInterval, generate.ReportInterval, generate.AbsentLimit = 5*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond
	t.Cleanup(func() {
		generate.PollInterval, generate.ReportInterval, generate.AbsentLimit = oldPoll, oldReport, oldAbsent
	})
	fake = comfytest.New(t, filepath.Join("..", "..", "..", "tests", "fixtures", "object_info.json"))
	return fake, tmp
}

func caption(t *testing.T, f *comfytest.Fake) string {
	t.Helper()
	for _, n := range f.Job("job-1").Graph {
		node := n.(map[string]any)
		if node["class_type"] == "CLIPTextEncode" {
			return node["inputs"].(map[string]any)["text"].(string)
		}
	}
	t.Fatal("no CLIPTextEncode node")
	return ""
}

func TestRunWritesAPNGAndPrintsOneJSONLine(t *testing.T) {
	f, tmp := setup(t)
	out := filepath.Join(tmp, "images", "queue.png")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-url", f.URL(), "-subject", subject, "-seed", "7", "-out", out, "-width", "1024", "-height", "768"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil || len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("no PNG at -out: %v", err)
	}
	if strings.Count(stdout.String(), "\n") != 1 {
		t.Fatalf("not one line: %q", stdout.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["out"] != out || got["seed"] != 7.0 || got["prompt_id"] != "job-1" ||
		got["width"] != 1024.0 || got["height"] != 768.0 || got["execution_seconds"] != 3.5 {
		t.Fatalf("result: %v", got)
	}
	if c := caption(t, f); c != subject {
		t.Fatalf("the caption is not the subject as written: %q", c)
	}
}

func TestRunReplacesAnExistingFileAndReadsTheSubjectFile(t *testing.T) {
	f, tmp := setup(t)
	out := filepath.Join(tmp, "queue.png")
	os.WriteFile(out, []byte("old"), 0o644)
	sf := filepath.Join(tmp, "subject.txt")
	os.WriteFile(sf, []byte("  "+subject+"\n"), 0o644)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-url", f.URL(), "-subject-file", sf, "-out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if data, _ := os.ReadFile(out); len(data) < 8 || string(data[:4]) != "\x89PNG" {
		t.Fatalf("the old file is still there: %q", data)
	}
	if !strings.Contains(stdout.String(), `"seed":424242`) {
		t.Fatalf("the default seed is not 424242: %s", stdout.String())
	}
	if c := caption(t, f); c != subject {
		t.Fatalf("caption: %q", c)
	}
}

func TestStyleFileWrapsTheSubject(t *testing.T) {
	f, tmp := setup(t)
	sf := filepath.Join(tmp, "style.json")
	os.WriteFile(sf, []byte(`{"prefix":"Flat vector.","suffix":"Soft light."}`), 0o644)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-url", f.URL(), "-subject", subject, "-style-file", sf, "-out", filepath.Join(tmp, "a.png")}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if c := caption(t, f); c != "Flat vector. "+subject+" Soft light." {
		t.Fatalf("caption: %q", c)
	}
}

func TestFailureIsOneJSONLineOnStderrAndExit1(t *testing.T) {
	f, tmp := setup(t)
	f.SetMode(comfytest.OOM)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-url", f.URL(), "-subject", subject, "-out", filepath.Join(tmp, "x.png")}, &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 {
		t.Fatalf("exit %d, stdout %q", code, stdout.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil || got["ok"] != false || got["error"] == "" {
		t.Fatalf("stderr: %q (%v)", stderr.String(), err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "x.png")); err == nil {
		t.Fatal("a PNG exists after a failed job")
	}

	// ComfyUI is down and autostart is off: a failure, no image.
	stderr.Reset()
	if code := Run([]string{"-url", "http://127.0.0.1:1", "-subject", subject, "-out", filepath.Join(tmp, "y.png")}, &stdout, &stderr); code != 1 ||
		!strings.Contains(stderr.String(), `"ok":false`) {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
}

func TestFlagErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	for _, args := range [][]string{nil, {"-subject", "a", "-subject-file", "b"}, {"-nope"}} {
		if code := Run(args, &stdout, &stderr); code != 2 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	if code := Run([]string{"-subject-file", filepath.Join(t.TempDir(), "absent.txt")}, &stdout, &stderr); code != 1 {
		t.Errorf("absent subject file: exit %d", code)
	}
}

// samplerSteps reads the step count of the KSampler node that the fake ComfyUI received.
func samplerSteps(t *testing.T, f *comfytest.Fake) float64 {
	t.Helper()
	for _, n := range f.Job("job-1").Graph {
		node := n.(map[string]any)
		if node["class_type"] == "KSampler" {
			return node["inputs"].(map[string]any)["steps"].(float64)
		}
	}
	t.Fatal("no KSampler node")
	return 0
}

func runJSON(t *testing.T, args ...string) map[string]any {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := Run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSteps(t *testing.T) {
	f, tmp := setup(t)
	got := runJSON(t, "-url", f.URL(), "-subject", subject, "-steps", "4", "-out", filepath.Join(tmp, "a.png"))
	if s := samplerSteps(t, f); s != 4 {
		t.Fatalf("the graph has steps %v, want 4", s)
	}
	if got["steps"] != 4.0 {
		t.Fatalf("the JSON line has steps %v, want 4", got["steps"])
	}

	// Without the flag the graph keeps the value of the workflow.
	f2, tmp2 := setup(t)
	got2 := runJSON(t, "-url", f2.URL(), "-subject", subject, "-out", filepath.Join(tmp2, "b.png"))
	own := samplerSteps(t, f2)
	if own == 4 || got2["steps"] != own {
		t.Fatalf("graph steps %v, JSON steps %v", own, got2["steps"])
	}
}

func TestWorkflowPath(t *testing.T) {
	f, tmp := setup(t)
	// A workflow folder outside the workflows folder of the process.
	other := filepath.Join(tmp, "elsewhere", "it-illustration")
	os.MkdirAll(other, 0o755)
	for _, name := range []string{"manifest.json", "workflow.json", "models.json"} {
		data, err := os.ReadFile(filepath.Join(tmp, "workflows", "it-illustration", name))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(other, name), data, 0o644)
	}
	os.RemoveAll(filepath.Join(tmp, "workflows", "it-illustration"))
	t.Setenv("HORUS_WORKFLOWS_DIR", filepath.Join(tmp, "workflows"))
	got := runJSON(t, "-url", f.URL(), "-subject", subject, "-workflow", filepath.Join(other, "workflow.json"),
		"-out", filepath.Join(tmp, "c.png"))
	if got["ok"] != true || got["prompt_id"] != "job-1" {
		t.Fatalf("result: %v", got)
	}
	if c := caption(t, f); c != subject {
		t.Fatalf("caption: %q", c)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-url", f.URL(), "-subject", subject, "-workflow", filepath.Join(tmp, "absent", "workflow.json")}, &stdout, &stderr); code != 1 {
		t.Fatalf("absent path: exit %d", code)
	}
}

func TestJSONKeys(t *testing.T) {
	f, tmp := setup(t)
	got := runJSON(t, "-url", f.URL(), "-subject", subject, "-out", filepath.Join(tmp, "d.png"))
	for _, k := range []string{"ok", "prompt_id", "out", "seed", "steps", "width", "height", "execution_seconds", "comfy_output"} {
		if _, ok := got[k]; !ok {
			t.Errorf("the key %q is missing: %v", k, got)
		}
	}
	if name, _ := got["comfy_output"].(string); !strings.HasSuffix(name, ".png") {
		t.Errorf("comfy_output %q is not the file that ComfyUI reported", name)
	}
}

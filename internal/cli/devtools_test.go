package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/devtools"
)

func TestNewWorkflowRun(t *testing.T) {
	wf := filepath.Join(t.TempDir(), "workflows")
	os.MkdirAll(filepath.Join(wf, "_template"), 0o755)
	os.WriteFile(filepath.Join(wf, "_template", "README.md"), []byte("# {{NAME}}\n\nModel: {{MODEL_FAMILY}}.\n"), 0o644)
	var out, errOut bytes.Buffer
	args := []string{"-workflows-dir", wf, "-name", "new-one", "-description", "A test"}
	if code := NewWorkflow(args, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "Created "+filepath.Join(wf, "new-one", "README.md")+"\n") ||
		!strings.Contains(out.String(), "Next: add workflow.json") {
		t.Fatalf("output: %q", out.String())
	}
	data, _ := os.ReadFile(filepath.Join(wf, "new-one", "README.md"))
	if !strings.Contains(string(data), "Model: TBD.") {
		t.Fatalf("the default model family is not TBD: %q", data)
	}
	if code := NewWorkflow(args, &out, &errOut); code != 1 {
		t.Fatalf("a second run: exit %d", code)
	}
	if code := NewWorkflow([]string{"-workflows-dir", wf, "-name", "Bad_Name", "-description", "d"}, &out, &errOut); code != 1 {
		t.Fatalf("bad name: exit %d", code)
	}
	if code := NewWorkflow([]string{"-workflows-dir", wf, "-name", "x"}, &out, &errOut); code != 2 {
		t.Fatalf("no description: exit %d", code)
	}
}

// install is a fake of the ComfyUI install: installed when main.py is in
// <dir>/ComfyUI.
type install string

func (i install) Installed() (string, bool) {
	_, err := os.Stat(filepath.Join(string(i), "ComfyUI", "main.py"))
	return string(i), err == nil
}

func TestSyncWorkflowsLinesAndExitCodes(t *testing.T) {
	tmp := t.TempDir()
	comfy, wf := filepath.Join(tmp, "comfy"), filepath.Join(tmp, "workflows")
	os.MkdirAll(filepath.Join(comfy, "ComfyUI"), 0o755)
	os.WriteFile(filepath.Join(comfy, "ComfyUI", "main.py"), []byte("x"), 0o644)
	for name, body := range map[string]string{"alpha": `{"a":1}`, "beta": `{"b":2}`} {
		os.MkdirAll(filepath.Join(wf, name), 0o755)
		os.WriteFile(filepath.Join(wf, name, "workflow.json"), []byte(body), 0o644)
	}
	pane := filepath.Join(comfy, "ComfyUI", "user", "default", "workflows")
	var asked []string
	at := func(dir string) devtools.Install { asked = append(asked, dir); return install(dir) }
	var out, errOut bytes.Buffer
	args := []string{"-comfyui-dir", comfy, "-workflows-dir", wf}
	if code := SyncWorkflows(args, &out, &errOut, at); code != 0 || out.String() != "copied   horus-alpha.json\ncopied   horus-beta.json\n" {
		t.Fatalf("exit %d: %q %s", code, out.String(), errOut.String())
	}
	if len(asked) != 1 || asked[0] != comfy {
		t.Fatalf("install of %q", asked)
	}
	os.WriteFile(filepath.Join(pane, "horus-beta.json"), []byte("changed"), 0o644)
	out.Reset()
	want := "ok       horus-alpha.json\nCONFLICT horus-beta.json differs from " + filepath.Join("workflows", "beta", "workflow.json") +
		". Compare both, then run again with -force to use the repo copy.\n"
	if code := SyncWorkflows(args, &out, &errOut, at); code != 1 || out.String() != want {
		t.Fatalf("conflict: exit %d: %q", code, out.String())
	}
	out.Reset()
	if code := SyncWorkflows(append(args, "-force"), &out, &errOut, at); code != 0 ||
		!strings.Contains(out.String(), "replaced horus-beta.json (old copy kept as .bak)\n") {
		t.Fatalf("-force: exit %d: %q", code, out.String())
	}
	errOut.Reset()
	if code := SyncWorkflows([]string{"-comfyui-dir", filepath.Join(comfy, "absent"), "-workflows-dir", wf}, &out, &errOut, at); code != 1 ||
		!strings.HasPrefix(errOut.String(), "horus sync-workflows: ComfyUI was not found") {
		t.Fatalf("no ComfyUI: exit %d: %s", code, errOut.String())
	}
	empty := filepath.Join(tmp, "empty")
	os.MkdirAll(empty, 0o755)
	out.Reset()
	if code := SyncWorkflows([]string{"-comfyui-dir", comfy, "-workflows-dir", empty}, &out, &errOut, at); code != 0 ||
		out.String() != "No workflows/*/workflow.json files found. Nothing to sync.\n" {
		t.Fatalf("no workflows: exit %d: %q", code, out.String())
	}
	if code := SyncWorkflows([]string{"-nope"}, &out, &errOut, at); code != 2 {
		t.Fatalf("bad flag: exit %d", code)
	}
}

package syncworkflows

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type rig struct {
	comfy, wf, pane string
	out             *bytes.Buffer
}

// newRig has a fake ComfyUI folder and a workflows folder with two workflows
// and one folder without workflow.json (the template).
func newRig(t *testing.T) *rig {
	t.Helper()
	tmp := t.TempDir()
	r := &rig{comfy: filepath.Join(tmp, "comfy"), wf: filepath.Join(tmp, "workflows"), out: &bytes.Buffer{}}
	os.MkdirAll(filepath.Join(r.comfy, "ComfyUI"), 0o755)
	os.WriteFile(filepath.Join(r.comfy, "ComfyUI", "main.py"), []byte("x"), 0o644)
	for name, body := range map[string]string{"alpha": `{"a":1}`, "beta": `{"b":2}`} {
		os.MkdirAll(filepath.Join(r.wf, name), 0o755)
		os.WriteFile(filepath.Join(r.wf, name, "workflow.json"), []byte(body), 0o644)
	}
	os.MkdirAll(filepath.Join(r.wf, "_template"), 0o755)
	os.WriteFile(filepath.Join(r.wf, "_template", "README.md"), []byte("t"), 0o644)
	r.pane = filepath.Join(r.comfy, "ComfyUI", "user", "default", "workflows")
	return r
}

func (r *rig) sync(force bool) (int, error) {
	r.out.Reset()
	return Sync(Options{ComfyUIDir: r.comfy, WorkflowsDir: r.wf, Force: force, Stdout: r.out})
}

func read(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSyncCopiesEachWorkflowIntoThePane(t *testing.T) {
	r := newRig(t)
	conflicts, err := r.sync(false)
	if err != nil || conflicts != 0 {
		t.Fatalf("conflicts %d, err %v", conflicts, err)
	}
	if got := read(t, filepath.Join(r.pane, "horus-alpha.json")); got != `{"a":1}` {
		t.Fatalf("alpha copy: %q", got)
	}
	if got := read(t, filepath.Join(r.pane, "horus-beta.json")); got != `{"b":2}` {
		t.Fatalf("beta copy: %q", got)
	}
	if got := r.out.String(); got != "copied   horus-alpha.json\ncopied   horus-beta.json\n" {
		t.Fatalf("output: %q", got)
	}
	if _, err := os.Stat(filepath.Join(r.pane, "horus-_template.json")); err == nil {
		t.Fatal("the template folder was copied")
	}
	// A second run changes nothing.
	if conflicts, err := r.sync(false); err != nil || conflicts != 0 || r.out.String() != "ok       horus-alpha.json\nok       horus-beta.json\n" {
		t.Fatalf("second run: %d %v %q", conflicts, err, r.out)
	}
}

func TestSyncReportsAConflictAndLeavesTheCopy(t *testing.T) {
	r := newRig(t)
	r.sync(false)
	edited := filepath.Join(r.pane, "horus-alpha.json")
	os.WriteFile(edited, []byte(`{"saved":"in the page"}`), 0o644)
	conflicts, err := r.sync(false)
	if err != nil || conflicts != 1 {
		t.Fatalf("conflicts %d, err %v", conflicts, err)
	}
	if !strings.Contains(r.out.String(), "CONFLICT horus-alpha.json differs from ") || !strings.Contains(r.out.String(), "ok       horus-beta.json") {
		t.Fatalf("output: %q", r.out)
	}
	if got := read(t, edited); got != `{"saved":"in the page"}` {
		t.Fatalf("the pane copy changed: %q", got)
	}
	if _, err := os.Stat(edited + ".bak"); err == nil {
		t.Fatal("a .bak file exists without -force")
	}
}

func TestSyncForceReplacesAndKeepsABackup(t *testing.T) {
	r := newRig(t)
	r.sync(false)
	edited := filepath.Join(r.pane, "horus-alpha.json")
	os.WriteFile(edited, []byte(`{"saved":"in the page"}`), 0o644)
	conflicts, err := r.sync(true)
	if err != nil || conflicts != 0 {
		t.Fatalf("conflicts %d, err %v", conflicts, err)
	}
	if got := read(t, edited); got != `{"a":1}` {
		t.Fatalf("the pane copy is not the repo copy: %q", got)
	}
	if got := read(t, edited+".bak"); got != `{"saved":"in the page"}` {
		t.Fatalf("the .bak is not the old copy: %q", got)
	}
	if !strings.Contains(r.out.String(), "replaced horus-alpha.json (old copy kept as .bak)\n") {
		t.Fatalf("output: %q", r.out)
	}
}

func TestSyncNeedsComfyUIAndAWorkflowsFolder(t *testing.T) {
	r := newRig(t)
	os.Remove(filepath.Join(r.comfy, "ComfyUI", "main.py"))
	if _, err := r.sync(false); err == nil || !strings.Contains(err.Error(), "ComfyUI was not found in "+r.comfy) {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(r.pane); err == nil {
		t.Fatal("the pane folder was made for a missing ComfyUI")
	}
	r = newRig(t)
	r.wf = filepath.Join(r.wf, "absent")
	if _, err := r.sync(false); err == nil || !strings.Contains(err.Error(), "no workflows folder") {
		t.Fatalf("err: %v", err)
	}
}

func TestRunExitCodes(t *testing.T) {
	r := newRig(t)
	var out, errOut bytes.Buffer
	args := []string{"-comfyui-dir", r.comfy, "-workflows-dir", r.wf}
	if code := Run(args, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	os.WriteFile(filepath.Join(r.pane, "horus-beta.json"), []byte("changed"), 0o644)
	out.Reset()
	if code := Run(args, &out, &errOut); code != 1 || !strings.Contains(out.String(), "CONFLICT") {
		t.Fatalf("conflict: exit %d: %s", code, out.String())
	}
	if code := Run(append(args, "-force"), &out, &errOut); code != 0 {
		t.Fatalf("-force: exit %d", code)
	}
	errOut.Reset()
	if code := Run([]string{"-comfyui-dir", filepath.Join(r.comfy, "absent"), "-workflows-dir", r.wf}, &out, &errOut); code != 1 ||
		!strings.HasPrefix(errOut.String(), "horus sync-workflows: ComfyUI was not found") {
		t.Fatalf("no ComfyUI: exit %d: %s", code, errOut.String())
	}
}

package devtools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const template = "# {{NAME}}\n\n{{DESCRIPTION}}\n\nModel: {{MODEL_FAMILY}}. Again {{NAME}}.\n"

// newDir is a temporary copy of the workflows folder: the real template and
// one existing workflow folder.
func newDir(t *testing.T) string {
	t.Helper()
	wf := filepath.Join(t.TempDir(), "workflows")
	os.MkdirAll(filepath.Join(wf, "_template"), 0o755)
	os.MkdirAll(filepath.Join(wf, "existing"), 0o755)
	os.WriteFile(filepath.Join(wf, "_template", "README.md"), []byte(template), 0o644)
	return wf
}

func TestCreateFillsTheTemplate(t *testing.T) {
	wf := newDir(t)
	readme, err := NewWorkflow(wf, "postcard-watercolor", "Soft watercolor landscape postcards", "FLUX 3 Image Klein")
	if err != nil {
		t.Fatal(err)
	}
	if readme != filepath.Join(wf, "postcard-watercolor", "README.md") {
		t.Fatalf("path: %s", readme)
	}
	data, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	want := "# postcard-watercolor\n\nSoft watercolor landscape postcards\n\nModel: FLUX 3 Image Klein. Again postcard-watercolor.\n"
	if string(data) != want {
		t.Fatalf("README:\n%q\nwant:\n%q", data, want)
	}
}

func TestCreateRefusesANameThatIsNotKebabCase(t *testing.T) {
	for _, name := range []string{"Postcard", "post_card", "-post", "post-", "post--card", "post card", "", "a/b", "..", "pöst"} {
		wf := newDir(t)
		if _, err := NewWorkflow(wf, name, "d", "TBD"); err == nil || !strings.Contains(err.Error(), "Name must be kebab-case") {
			t.Errorf("%q: err %v", name, err)
		}
		entries, _ := os.ReadDir(wf)
		if len(entries) != 2 {
			t.Errorf("%q: the folder changed: %v", name, entries)
		}
	}
	for _, name := range []string{"a", "post-card", "v2-style", "9"} {
		if _, err := NewWorkflow(newDir(t), name, "d", "TBD"); err != nil {
			t.Errorf("%q: %v", name, err)
		}
	}
}

func TestCreateNeverOverwrites(t *testing.T) {
	wf := newDir(t)
	if _, err := NewWorkflow(wf, "existing", "d", "TBD"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wf, "existing", "README.md")); err == nil {
		t.Fatal("wrote into an existing folder")
	}
	os.Remove(filepath.Join(wf, "_template", "README.md"))
	if _, err := NewWorkflow(wf, "fresh", "d", "TBD"); err == nil || !strings.Contains(err.Error(), "Template not found") {
		t.Fatalf("err: %v", err)
	}
}

// install is a fake of the ComfyUI install.
type install struct {
	dir string
	ok  bool
}

func (i install) Installed() (string, bool) { return i.dir, i.ok }

type rig struct {
	comfy, wf, pane string
	ok              bool
}

// newRig has a ComfyUI folder and a workflows folder with two workflows and
// one folder without workflow.json (the template).
func newRig(t *testing.T) *rig {
	t.Helper()
	tmp := t.TempDir()
	r := &rig{comfy: filepath.Join(tmp, "comfy"), wf: filepath.Join(tmp, "workflows"), ok: true}
	os.MkdirAll(filepath.Join(r.comfy, "ComfyUI"), 0o755)
	for name, body := range map[string]string{"alpha": `{"a":1}`, "beta": `{"b":2}`} {
		os.MkdirAll(filepath.Join(r.wf, name), 0o755)
		os.WriteFile(filepath.Join(r.wf, name, "workflow.json"), []byte(body), 0o644)
	}
	os.MkdirAll(filepath.Join(r.wf, "_template"), 0o755)
	os.WriteFile(filepath.Join(r.wf, "_template", "README.md"), []byte("t"), 0o644)
	r.pane = filepath.Join(r.comfy, "ComfyUI", "user", "default", "workflows")
	return r
}

func (r *rig) sync(force bool) ([]Copy, error) {
	return Sync(install{r.comfy, r.ok}, r.wf, force)
}

// states is "<file> <state>" for each copy.
func states(cs []Copy) string {
	var s []string
	for _, c := range cs {
		s = append(s, c.File+" "+c.State)
	}
	return strings.Join(s, ", ")
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
	cs, err := r.sync(false)
	if err != nil || states(cs) != "horus-alpha.json copied, horus-beta.json copied" {
		t.Fatalf("copies %q, err %v", states(cs), err)
	}
	if got := read(t, filepath.Join(r.pane, "horus-alpha.json")); got != `{"a":1}` {
		t.Fatalf("alpha copy: %q", got)
	}
	if got := read(t, filepath.Join(r.pane, "horus-beta.json")); got != `{"b":2}` {
		t.Fatalf("beta copy: %q", got)
	}
	if cs[0].Source != filepath.Join("workflows", "alpha", "workflow.json") {
		t.Fatalf("source: %q", cs[0].Source)
	}
	if _, err := os.Stat(filepath.Join(r.pane, "horus-_template.json")); err == nil {
		t.Fatal("the template folder was copied")
	}
	// A second run changes nothing.
	if cs, err := r.sync(false); err != nil || states(cs) != "horus-alpha.json ok, horus-beta.json ok" {
		t.Fatalf("second run: %q %v", states(cs), err)
	}
}

func TestSyncReportsAConflictAndLeavesTheCopy(t *testing.T) {
	r := newRig(t)
	r.sync(false)
	edited := filepath.Join(r.pane, "horus-alpha.json")
	os.WriteFile(edited, []byte(`{"saved":"in the page"}`), 0o644)
	cs, err := r.sync(false)
	if err != nil || states(cs) != "horus-alpha.json conflict, horus-beta.json ok" {
		t.Fatalf("copies %q, err %v", states(cs), err)
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
	cs, err := r.sync(true)
	if err != nil || states(cs) != "horus-alpha.json replaced, horus-beta.json ok" {
		t.Fatalf("copies %q, err %v", states(cs), err)
	}
	if got := read(t, edited); got != `{"a":1}` {
		t.Fatalf("the pane copy is not the repo copy: %q", got)
	}
	if got := read(t, edited+".bak"); got != `{"saved":"in the page"}` {
		t.Fatalf("the .bak is not the old copy: %q", got)
	}
}

func TestSyncNeedsComfyUIAndAWorkflowsFolder(t *testing.T) {
	r := newRig(t)
	r.ok = false
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
	// No workflow.json at all: no copy and no error.
	r = newRig(t)
	empty := filepath.Join(t.TempDir(), "workflows")
	os.MkdirAll(empty, 0o755)
	if cs, err := Sync(install{r.comfy, true}, empty, false); err != nil || len(cs) != 0 {
		t.Fatalf("empty folder: %v %v", fmt.Sprint(cs), err)
	}
}

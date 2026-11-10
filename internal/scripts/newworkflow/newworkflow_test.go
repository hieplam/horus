package newworkflow

import (
	"bytes"
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
	readme, err := Create(wf, "postcard-watercolor", "Soft watercolor landscape postcards", "FLUX 3 Image Klein")
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
		if _, err := Create(wf, name, "d", "TBD"); err == nil || !strings.Contains(err.Error(), "Name must be kebab-case") {
			t.Errorf("%q: err %v", name, err)
		}
		entries, _ := os.ReadDir(wf)
		if len(entries) != 2 {
			t.Errorf("%q: the folder changed: %v", name, entries)
		}
	}
	for _, name := range []string{"a", "post-card", "v2-style", "9"} {
		if _, err := Create(newDir(t), name, "d", "TBD"); err != nil {
			t.Errorf("%q: %v", name, err)
		}
	}
}

func TestCreateNeverOverwrites(t *testing.T) {
	wf := newDir(t)
	if _, err := Create(wf, "existing", "d", "TBD"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wf, "existing", "README.md")); err == nil {
		t.Fatal("wrote into an existing folder")
	}
	os.Remove(filepath.Join(wf, "_template", "README.md"))
	if _, err := Create(wf, "fresh", "d", "TBD"); err == nil || !strings.Contains(err.Error(), "Template not found") {
		t.Fatalf("err: %v", err)
	}
}

func TestRun(t *testing.T) {
	wf := newDir(t)
	var out, errOut bytes.Buffer
	args := []string{"-workflows-dir", wf, "-name", "new-one", "-description", "A test"}
	if code := Run(args, &out, &errOut); code != 0 {
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
	if code := Run(args, &out, &errOut); code != 1 {
		t.Fatalf("a second run: exit %d", code)
	}
	if code := Run([]string{"-workflows-dir", wf, "-name", "Bad_Name", "-description", "d"}, &out, &errOut); code != 1 {
		t.Fatalf("bad name: exit %d", code)
	}
	if code := Run([]string{"-workflows-dir", wf, "-name", "x"}, &out, &errOut); code != 2 {
		t.Fatalf("no description: exit %d", code)
	}
}

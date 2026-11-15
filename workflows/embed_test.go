package workflows

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

// TestEveryWorkflowFolderIsEmbedded: each folder of workflows/ with a
// manifest.json is in FS with the same files, and _template is not.
func TestEveryWorkflowFolderIsEmbedded(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		if _, err := os.Stat(e.Name() + "/manifest.json"); err != nil {
			continue
		}
		n++
		err := fs.WalkDir(os.DirFS("."), e.Name(), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			want, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			got, err := fs.ReadFile(FS, p)
			if err != nil {
				t.Errorf("%s is not in the program: add the folder %s to the go:embed line", p, e.Name())
				return nil
			}
			if string(got) != string(want) {
				t.Errorf("%s in the program differs from the file", p)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if n == 0 {
		t.Fatal("no workflow folder with a manifest.json")
	}
	if _, err := fs.Stat(FS, "_template"); err == nil {
		t.Error("_template must not be in the program")
	}
	if _, err := fs.ReadFile(FS, "it-illustration/manifest.json"); err != nil {
		t.Errorf("it-illustration/manifest.json: %v", err)
	}
}

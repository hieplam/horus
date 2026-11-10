// Tests of the port of horus/catalog.py.
package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
)

func useWorkflows(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HORUS_WORKFLOWS_DIR", dir)
	settings.Reset()
	t.Cleanup(settings.Reset)
}

// compactText writes Go JSON in the form of the golden file: the workflow
// folder is <dir> and every path uses "/".
func compactText(t *testing.T, v any, dir string) string {
	t.Helper()
	raw, err := marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	esc, _ := json.Marshal(dir)
	s := strings.ReplaceAll(string(raw), string(esc[1:len(esc)-1]), "<dir>")
	return strings.ReplaceAll(s, `\\`, "/")
}

func compact(t *testing.T, s string) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, []byte(s)); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// TestShippedWorkflowMatchesPython compares SummaryOf and DetailOf of
// workflows/it-illustration with catalog.summary and catalog.detail of the
// Python code, as text, so that the key order is checked too.
func TestShippedWorkflowMatchesPython(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", "..", "workflows"))
	useWorkflows(t, root)
	raw, err := os.ReadFile("testdata/it-illustration.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct{ Summary, Detail string }
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	w, err := Get("it-illustration")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := SummaryOf(w)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := compactText(t, sum, w.Dir), compact(t, golden.Summary); got != want {
		t.Errorf("summary differs from Python\n got %s\nwant %s", got, want)
	}
	det, err := DetailOf(w)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := compactText(t, det, w.Dir), compact(t, golden.Detail); got != want {
		t.Errorf("detail differs from Python\n got %s\nwant %s", got, want)
	}
	if strings.Contains(compactText(t, det, w.Dir), "workflow.json") {
		t.Error("detail shows the graph file")
	}
}

func writeManifest(t *testing.T, root, name string, edit func(m map[string]any)) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "workflows", "it-illustration", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(src, &m); err != nil {
		t.Fatal(err)
	}
	m["name"] = name
	if edit != nil {
		edit(m)
	}
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestManifestChecks(t *testing.T) {
	cases := []struct {
		name string
		edit func(m map[string]any)
		want string
	}{
		{"theme", func(m map[string]any) { m["theme"] = "navy" }, "has theme: a workflow must not add a style; the caller owns it"},
		{"caption", func(m map[string]any) { m["caption"] = map[string]any{} }, "has caption: a workflow must not add a style"},
		{"lacks", func(m map[string]any) { delete(m, "summary"); delete(m, "graph") }, "lacks: summary, graph"},
		{"wrongname", func(m map[string]any) { m["name"] = "other" }, "has name 'other'"},
		{"inputs", func(m map[string]any) {
			g := m["graph"].(map[string]any)
			in := g["inputs"].(map[string]any)
			delete(in, "seed")
			delete(in, "filename_prefix")
		}, "graph.inputs lacks seed, filename_prefix"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			useWorkflows(t, root)
			writeManifest(t, root, "w-"+c.name, c.edit)
			_, err := All()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want %q", err, c.want)
			}
		})
	}
}

func TestFoldersThatAreNotOffered(t *testing.T) {
	root := t.TempDir()
	useWorkflows(t, root)
	writeManifest(t, root, "b-good", nil)
	writeManifest(t, root, "a-good", nil)
	writeManifest(t, root, "_template", nil)
	if err := os.MkdirAll(filepath.Join(root, "no-manifest"), 0o755); err != nil {
		t.Fatal(err)
	}
	all, err := All()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, w := range all {
		names = append(names, w.Name)
	}
	if strings.Join(names, ",") != "a-good,b-good" {
		t.Fatalf("offered %v", names)
	}
}

func TestGetErrors(t *testing.T) {
	root := t.TempDir()
	useWorkflows(t, root)
	writeManifest(t, root, "zeta", nil)
	writeManifest(t, root, "alpha", nil)

	var he *herr.Error
	_, err := Get("bad name!")
	if !errors.As(err, &he) || he.Code != "INVALID_ARGUMENT" || he.Details["field"] != "workflow" || he.Message != "invalid workflow name 'bad name!'" {
		t.Fatalf("bad name: %v", err)
	}
	_, err = Get("nope")
	if !errors.As(err, &he) || he.Code != "WORKFLOW_NOT_FOUND" || he.Message != "no workflow named 'nope'" {
		t.Fatalf("unknown: %v", err)
	}
	if known, _ := he.Details["known"].([]string); strings.Join(known, ",") != "alpha,zeta" {
		t.Fatalf("known = %v", he.Details["known"])
	}
	if w, err := Get("alpha"); err != nil || w.Name != "alpha" {
		t.Fatalf("Get(alpha) = %v, %v", w, err)
	}
}

func TestLicenseWithoutModelsFile(t *testing.T) {
	root := t.TempDir()
	useWorkflows(t, root)
	writeManifest(t, root, "solo", nil)
	w, err := Get("solo")
	if err != nil {
		t.Fatal(err)
	}
	lic, err := w.License()
	if err != nil || lic.Name != "not stated" || lic.URL != nil || lic.DownloadBytes != 0 {
		t.Fatalf("license = %+v, %v", lic, err)
	}
}

func TestPyRepr(t *testing.T) {
	for in, want := range map[string]string{"nope": "'nope'", "it's": `"it's"`, `a'b"c`: `'a\'b"c'`, "x\n": `'x\n'`} {
		if got := pyRepr(in); got != want {
			t.Errorf("pyRepr(%q) = %s, want %s", in, got, want)
		}
	}
}

// Tests of the kind of a workflow in list_workflows and describe_workflow.
package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// noFilesSetup is a setup service for a speech workflow that lists no model files.
type noFilesSetup struct{ stateSetup }

func (noFilesSetup) List(context.Context, string) ([]provision.Row, error) { return nil, nil }
func (noFilesSetup) Check(_ context.Context, req provision.Request) (*provision.Result, error) {
	return &provision.Result{Action: "check", Workflow: req.Workflow, Checked: true}, nil
}

// useKindWorkflows makes a workflows folder with the image workflow
// it-illustration (its manifest) and the stand-in speech workflow of testdata.
func useKindWorkflows(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	for name, src := range map[string]string{
		"it-illustration": filepath.Join("..", "..", "workflows", "it-illustration", "manifest.json"),
		"stand-in-speech": filepath.Join("..", "catalog", "testdata", "stand-in-speech", "manifest.json"),
	} {
		raw, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name, "manifest.json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HORUS_WORKFLOWS_DIR", root)
	settings.Reset()
	t.Cleanup(settings.Reset)
}

func TestKindIsInListWorkflows(t *testing.T) {
	useKindWorkflows(t)
	cs := connect(t, New(&fakeImages{}, noFilesSetup{}), nil)
	res, _ := envelopeOf(t, callTool(t, cs, "list_workflows", map[string]any{}))["result"].(map[string]any)
	kinds := map[string]any{}
	outputs := map[string]any{}
	rows, _ := res["workflows"].([]any)
	for _, r := range rows {
		m := r.(map[string]any)
		kinds[m["name"].(string)], outputs[m["name"].(string)] = m["kind"], m["output"]
	}
	if len(kinds) != 2 || kinds["it-illustration"] != "image" || kinds["stand-in-speech"] != "speech" {
		t.Fatalf("kinds %v", kinds)
	}
	if outputs["it-illustration"] == nil || outputs["stand-in-speech"] != nil {
		t.Fatalf("outputs %v: an image workflow has an output size, a speech workflow has none", outputs)
	}
}

func TestKindSpeechDescribeShowsTheSpeechFields(t *testing.T) {
	useKindWorkflows(t)
	cs := connect(t, New(&fakeImages{}, noFilesSetup{}), nil)
	env := envelopeOf(t, callTool(t, cs, "describe_workflow", map[string]any{"workflow": "stand-in-speech"}))
	r, _ := env["result"].(map[string]any)
	if env["ok"] != true || r == nil {
		t.Fatalf("describe_workflow: %v", env)
	}
	if r["kind"] != "speech" || r["default_voice"] != "alpha" {
		t.Errorf("kind %v, default_voice %v", r["kind"], r["default_voice"])
	}
	for k, n := range map[string]int{"voices": 2, "tags": 2, "formats": 2} {
		if l, _ := r[k].([]any); len(l) != n {
			t.Errorf("%s: %v, want %d items", k, r[k], n)
		}
	}
	if txt, _ := r["text"].(map[string]any); txt["max_chars"] != float64(500) {
		t.Errorf("text: %v", r["text"])
	}
	if r["output"] != nil || r["subject"] != nil {
		t.Errorf("a speech workflow has no output size and no subject: %v, %v", r["output"], r["subject"])
	}
}

func TestKindImageDescribeHasNoSpeechFields(t *testing.T) {
	useKindWorkflows(t)
	cs := connect(t, New(&fakeImages{}, noFilesSetup{}), nil)
	r, _ := envelopeOf(t, callTool(t, cs, "describe_workflow", map[string]any{"workflow": "it-illustration"}))["result"].(map[string]any)
	if r["kind"] != "image" {
		t.Errorf("kind %v", r["kind"])
	}
	for _, k := range []string{"text", "voices", "default_voice", "tags", "formats"} {
		if _, ok := r[k]; ok {
			t.Errorf("an image workflow shows %s", k)
		}
	}
}

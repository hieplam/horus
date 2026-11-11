package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/provision"
)

func TestGB2AsTheScriptPrintsIt(t *testing.T) {
	// The values that workflow.ps1 (on .NET Framework) printed for "{0:N2}".
	for b, want := range map[int64]string{0: "0.00", 172: "0.00", 5_827_874_779: "5.83", 1_234_567_890_123: "1,234.57",
		999_995_000_000: "1,000.00", 1_005_000_000: "1.01", 2_675_000_000: "2.68", 4_000_000: "0.00", 5_000_000: "0.01"} {
		if got := gb2(b); got != want {
			t.Errorf("gb2(%d) = %q, want %q", b, got, want)
		}
	}
}

// fakeSetup returns res and err for each action, and keeps the request.
type fakeSetup struct {
	req provision.Request
	res *provision.Result
	err error
}

func (f *fakeSetup) List(context.Context, string) ([]provision.Row, error) { return nil, f.err }
func (f *fakeSetup) Check(_ context.Context, r provision.Request) (*provision.Result, error) {
	f.req = r
	return f.res, f.err
}
func (f *fakeSetup) Install(_ context.Context, r provision.Request, w provision.Watch) (*provision.Result, error) {
	f.req = r
	w.Started("lic", 1)
	w.Downloading("vae", "/m/vae/v.safetensors")
	io.WriteString(w.Out, "Downloading v.safetensors\n")
	w.Failed(errors.New("no"))
	return f.res, f.err
}
func (f *fakeSetup) Remove(_ context.Context, r provision.Request) (*provision.Result, error) {
	f.req = r
	return f.res, f.err
}

// TestWorkflowPassesTheFlagsAndPrintsTheResult: the flags become the
// request, and the typed result becomes the lines and the HORUS_JSON line.
func TestWorkflowPassesTheFlagsAndPrintsTheResult(t *testing.T) {
	f := &fakeSetup{res: &provision.Result{Action: "install", Workflow: "demo", Message: "1 of 1 file(s) are not ready.",
		Items:    []provision.Item{{Status: "FAILED", Kind: "download", Role: "vae", File: "v.safetensors", Path: "/m/vae/v.safetensors", Bytes: 5, Detail: "FAIL: no"}},
		Mappings: []provision.Mapping{{Folder: "vae", Dir: "/m/vae", Added: true}}}, err: errors.New("1 of 1 file(s) are not ready.")}
	var place SetupPlace
	var out bytes.Buffer
	code := Workflow(context.Background(), []string{"install", "demo", "-only", "vae", "-json", "-workflows-dir", "/w", "-models-dir", "/m", "-server-url", "http://h:1"},
		&out, io.Discard, func(p SetupPlace) Setup { place = p; return f })
	if code != 1 || f.req.Workflow != "demo" || f.req.WorkflowsDir != "/w" || strings.Join(f.req.Only, ",") != "vae" ||
		place.ModelsDir != "/m" || place.ServerURL != "http://h:1" {
		t.Fatalf("exit %d, request %+v, place %+v", code, f.req, place)
	}
	want := "License: lic\nInstalling 1 model file(s) for workflow 'demo'.\n-- vae -> /m/vae/v.safetensors\nDownloading v.safetensors\nFAIL: no\n" +
		"extra_model_paths.yaml: added vae -> /m/vae\nRestart ComfyUI so that it reads the new paths (horus start-comfyui).\n" +
		"FAIL: 1 of 1 file(s) are not ready.\n" +
		`HORUS_JSON {"ok":false,"action":"install","workflow":"demo","message":"1 of 1 file(s) are not ready.","items":[{"status":"FAILED","kind":"download","role":"vae","file":"v.safetensors","path":"/m/vae/v.safetensors","bytes":5,"detail":"FAIL: no"}]}` + "\n"
	if got := strings.ReplaceAll(out.String(), `\`, "/"); got != want {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}

	// A flag that does not parse calls no service.
	called := false
	if code := Workflow(context.Background(), []string{"check", "-nope"}, io.Discard, io.Discard, func(SetupPlace) Setup { called = true; return f }); code != 2 || called {
		t.Errorf("exit %d, service called %v", code, called)
	}
}

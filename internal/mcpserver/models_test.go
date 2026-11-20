// Tests of the model tools in the stack (ADR 0041): the plan of
// install_workflow holds only model files in the models folder of the stack,
// the tools show the host paths, and the tools refuse a speech workflow and a
// stack without a models folder. The setup service is the real
// provision.Service on the models folder of the test.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// hostModels is the host folder of the models folder in these tests. The
// container folder is the models folder of the sandbox.
const hostModels = `D:\horus\models`

// standIns writes the stand-in body of each model file of it-illustration in
// the sandbox into the models folder, and gives the path of each one by its
// file name.
func standIns(t *testing.T, models string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.Getenv("HORUS_WORKFLOWS_DIR"), "it-illustration", "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Files []struct {
			RepoPath string `json:"repo_path"`
			Folder   string `json:"folder"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for _, f := range list.Files {
		name := f.RepoPath[strings.LastIndex(f.RepoPath, "/")+1:]
		p := filepath.Join(models, f.Folder, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("stand-in for "+f.RepoPath), 0o644); err != nil {
			t.Fatal(err)
		}
		paths[name] = p
	}
	return paths
}

// countingDownloads counts the downloads, and downloads nothing.
type countingDownloads struct{ n int }

func (d *countingDownloads) Download(context.Context, provision.Fetch, io.Writer) (bool, error) {
	d.n++
	return false, errors.New("the test downloads nothing")
}

// TestInstallWorkflowInTheStackPlansOnlyTheMissingModelFile: with each file
// in the models folder, list_workflows shows installed and install_workflow
// answers already_installed with the host paths. With one file removed, the
// workflow is partially_installed, and the first call answers
// LICENSE_CONFIRMATION_REQUIRED with exactly one step: that model file, in
// the host folder. It downloads nothing.
func TestInstallWorkflowInTheStackPlansOnlyTheMissingModelFile(t *testing.T) {
	models := setupSandbox(t)
	paths := standIns(t, models)
	dl := &countingDownloads{}
	svc := provision.New(diskFiles{models}, dl, downComfy{}, diskFiles{models}, "")
	cs := connect(t, NewWith(&fakeImages{}, svc, nil, Options{Models: ModelsOf(models, hostModels, "windows")}), nil)

	state := func() string {
		res, _ := envelopeOf(t, callTool(t, cs, "list_workflows", map[string]any{}))["result"].(map[string]any)
		rows, _ := res["workflows"].([]any)
		for _, r := range rows {
			if m := r.(map[string]any); m["name"] == "it-illustration" {
				return m["install_state"].(string)
			}
		}
		return "no row"
	}
	if s := state(); s != "installed" {
		t.Fatalf("install_state %s, want installed", s)
	}
	env := envelopeOf(t, callTool(t, cs, "install_workflow", map[string]any{"workflow": "it-illustration"}))
	r, _ := env["result"].(map[string]any)
	files, _ := r["files"].([]any)
	if env["ok"] != true || r["already_installed"] != true || len(files) != 3 {
		t.Fatalf("installed: %v", env)
	}
	want := hostModels + `\vae\flux2-vae.safetensors`
	var got []string
	for _, f := range files {
		got = append(got, f.(map[string]any)["path"].(string))
	}
	if !slices.Contains(got, want) {
		t.Errorf("the paths %v have no host path %s", got, want)
	}

	if err := os.Remove(paths["flux2-vae.safetensors"]); err != nil {
		t.Fatal(err)
	}
	if s := state(); s != "partially_installed" {
		t.Errorf("one file removed: install_state %s, want partially_installed", s)
	}
	code, d, _ := errorOf(t, envelopeOf(t, callTool(t, cs, "install_workflow", map[string]any{"workflow": "it-illustration"})))
	steps, _ := d["steps"].([]any)
	if code != "LICENSE_CONFIRMATION_REQUIRED" || len(steps) != 1 {
		t.Fatalf("one file removed: %s with steps %v, want LICENSE_CONFIRMATION_REQUIRED with 1 step", code, d["steps"])
	}
	st := steps[0].(map[string]any)
	if st["kind"] != "file" || st["name"] != "flux2-vae.safetensors" || st["dir"] != hostModels+`\vae` || st["command"] != nil {
		t.Errorf("the step %v, want the file flux2-vae.safetensors in %s", st, hostModels+`\vae`)
	}
	if dl.n != 0 {
		t.Errorf("the tool downloaded %d file(s) before the consent", dl.n)
	}
}

// TestInstallWorkflowInTheStackRefusesASetupStep: a setup service whose plan
// installs ComfyUI and a system package is a bug in the stack. The tool
// answers INTERNAL with the steps, also with both consents, and runs nothing.
func TestInstallWorkflowInTheStackRefusesASetupStep(t *testing.T) {
	m, svc := newEmptyMachine(t, true)
	before := tree(t, m.root)
	cs := connect(t, NewWith(&fakeImages{}, svc, nil, Options{Models: ModelsOf(m.models, hostModels, "windows")}), nil)
	code, d, _ := errorOf(t, envelopeOf(t, callTool(t, cs, "install_workflow",
		map[string]any{"workflow": "it-illustration", "accept_setup": true, "accept_license": true})))
	if code != "INTERNAL" || d["steps"] == nil {
		t.Fatalf("got %s %v, want INTERNAL with the steps", code, d)
	}
	if l := m.log.all(); len(l) != 0 {
		t.Errorf("the tool ran something: %v", l)
	}
	if after := tree(t, m.root); !slices.Equal(before, after) {
		t.Errorf("the files changed")
	}
}

// TestModelToolsInTheStackWithoutAModelsFolder: in the stack without
// HORUS_MODELS_DIR (also a stack whose options have no Models),
// install_workflow and uninstall_workflow answer BACKEND_NOT_INSTALLED,
// action by human, with a next_step that names the setting. Nothing changes.
func TestModelToolsInTheStackWithoutAModelsFolder(t *testing.T) {
	models := setupSandbox(t)
	standIns(t, models)
	before := tree(t, models)
	svc := provision.New(diskFiles{models}, &countingDownloads{}, downComfy{}, diskFiles{models}, "")
	for name, o := range map[string]Options{"Models without a folder": {Models: ModelsOf("", "", "")}, "a Queue without Models": {Queue: &Queue{}}} {
		cs := connect(t, NewWith(&fakeImages{}, svc, nil, o), nil)
		for _, call := range []map[string]any{
			{"tool": "install_workflow", "args": map[string]any{"workflow": "it-illustration", "accept_license": true}},
			{"tool": "uninstall_workflow", "args": map[string]any{"workflow": "it-illustration", "confirm": true}},
		} {
			code, d, e := errorOf(t, envelopeOf(t, callTool(t, cs, call["tool"].(string), call["args"])))
			ns, _ := e["next_step"].(string)
			if code != "BACKEND_NOT_INSTALLED" || e["action_by"] != "human" || d["setting"] != "HORUS_MODELS_DIR" ||
				!strings.Contains(ns, "HORUS_MODELS_DIR") || !strings.Contains(ns, "horus stack-mounts") {
				t.Errorf("%s, %s: %v", name, call["tool"], e)
			}
		}
	}
	if after := tree(t, models); !slices.Equal(before, after) {
		t.Errorf("the files changed")
	}
}

// TestModelToolsInTheStackRefuseASpeechWorkflow: a speech workflow does not
// run in the stack, so install_workflow and uninstall_workflow answer
// BACKEND_UNAVAILABLE, not retryable, with a next_step to the horus program
// of the host.
func TestModelToolsInTheStackRefuseASpeechWorkflow(t *testing.T) {
	models := setupSandbox(t)
	flows := os.Getenv("HORUS_WORKFLOWS_DIR")
	manifest, err := os.ReadFile(filepath.Join("..", "catalog", "testdata", "stand-in-speech", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(flows, "stand-in-speech"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flows, "stand-in-speech", "manifest.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	settings.Reset()
	svc := provision.New(diskFiles{models}, &countingDownloads{}, downComfy{}, diskFiles{models}, "")
	cs := connect(t, NewWith(&fakeImages{}, svc, nil, Options{Models: ModelsOf(models, hostModels, "windows")}), nil)
	for _, tool := range []string{"install_workflow", "uninstall_workflow"} {
		code, _, e := errorOf(t, envelopeOf(t, callTool(t, cs, tool, map[string]any{"workflow": "stand-in-speech"})))
		ns, _ := e["next_step"].(string)
		if code != "BACKEND_UNAVAILABLE" || e["retryable"] != false || e["action_by"] != "human" || !strings.Contains(ns, "horus register") {
			t.Errorf("%s: %v", tool, e)
		}
	}
}

// TestUninstallWorkflowInTheStack: the plan and the delete of uninstall_workflow
// in the stack name the host paths, and the confirmed call deletes the files
// of the models folder.
func TestUninstallWorkflowInTheStack(t *testing.T) {
	models := setupSandbox(t)
	paths := standIns(t, models)
	images := &fakeImages{}
	svc := provision.New(diskFiles{models}, &countingDownloads{}, downComfy{}, diskFiles{models}, "")
	cs := connect(t, NewWith(images, svc, nil, Options{Models: ModelsOf(models, hostModels, "windows")}), nil)
	code, d, _ := errorOf(t, envelopeOf(t, callTool(t, cs, "uninstall_workflow", map[string]any{"workflow": "it-illustration"})))
	files, _ := d["files"].([]any)
	if code != "CONFIRMATION_REQUIRED" || len(files) != 3 {
		t.Fatalf("plan: %s %v", code, d)
	}
	for _, f := range files {
		if p, _ := f.(map[string]any)["path"].(string); !strings.HasPrefix(p, hostModels+`\`) {
			t.Errorf("the plan names %s, want a host path in %s", p, hostModels)
		}
	}
	env := envelopeOf(t, callTool(t, cs, "uninstall_workflow", map[string]any{"workflow": "it-illustration", "confirm": true}))
	if r, _ := env["result"].(map[string]any); env["ok"] != true || stateList(r["files"]) != "deleted,deleted,deleted" || images.frees() != 1 {
		t.Fatalf("remove: %v, %d frees", env, images.frees())
	}
	for _, p := range paths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s is still there", p)
		}
	}
}

// TestModelsHostPath: a container path in the models folder becomes the host
// path, in the form of the host. Another path, and every path on the direct
// path, stays as it is.
func TestModelsHostPath(t *testing.T) {
	m := ModelsOf("/models", `C:\Users\you\ComfyUI-models`, "windows")
	for in, want := range map[string]string{
		"/models/vae/flux2-vae.safetensors": `C:\Users\you\ComfyUI-models\vae\flux2-vae.safetensors`,
		"/models":                           `C:\Users\you\ComfyUI-models`,
		"/elsewhere/a.safetensors":          "/elsewhere/a.safetensors",
		"":                                  "",
	} {
		if got := m.hostPath(in); got != want {
			t.Errorf("hostPath(%q) = %q, want %q", in, got, want)
		}
	}
	var direct *Models
	if got := direct.hostPath("/models/a"); got != "/models/a" {
		t.Errorf("the direct path changed %q", got)
	}
	if got := ModelsOf("/models", "", "").hostPath("/models/a"); got != "/models/a" {
		t.Errorf("without a host folder: %q", got)
	}
}

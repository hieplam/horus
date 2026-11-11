// Tests of the setup path through the wiring of cmd/horus: the handler of
// horus workflow (internal/cli) and the MCP tools (internal/mcpserver), on
// the setup service with the real model files adapter and the real
// downloader. They reach the parts that tests/workflow_parity.py cannot reach
// without a download: DOWNLOADED, FAILED with its text, the mapping in
// extra_model_paths.yaml, DELETE_FAILED, and the error codes of a failed
// download. Every download goes to a local fake HTTPS server, and every file
// is in a temporary folder; no test reaches the network.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/cli"
	"github.com/hieplam/horus/internal/download"
	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/mcpserver"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

var (
	vaeData = []byte("stand-in vae file for the Go test")
	teData  = []byte("stand-in text encoder file for the Go test, longer")
)

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// sandbox is one workflow "demo" with two model files, a ComfyUI folder and
// a models folder outside it.
type sandbox struct {
	root, workflows, comfy, models string
	mu                             sync.Mutex
	status                         int // HTTP status of the fake download server; 0 = 200
	downloads                      int
	srv                            *httptest.Server
	comfyUI                        *httptest.Server
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HORUS_ROOT", root) // no .env of a real checkout is read
	for _, k := range []string{"HORUS_MODELS_DIR", "HORUS_TEXT_ENCODER_DIR", "HORUS_COMFYUI_DIR", "HORUS_COMFYUI_URL"} {
		t.Setenv(k, "")
	}
	settings.Reset()
	t.Cleanup(settings.Reset)
	s := &sandbox{root: root, workflows: filepath.Join(root, "workflows"), comfy: filepath.Join(root, "comfy"),
		models: filepath.Join(root, "models")}
	wf := filepath.Join(s.workflows, "demo")
	must(t, os.MkdirAll(wf, 0o755))
	must(t, os.MkdirAll(filepath.Join(s.comfy, "ComfyUI"), 0o755))
	must(t, os.WriteFile(filepath.Join(s.comfy, "ComfyUI", "main.py"), []byte("# fake\n"), 0o644))
	models := map[string]any{"repo": "org/repo", "revision": "abc", "license": "test license", "files": []any{
		map[string]any{"role": "vae", "repo_path": "vae/demo-vae.safetensors", "folder": "vae", "bytes": len(vaeData), "sha256": sum(vaeData)},
		map[string]any{"role": "text_encoder", "repo_path": "te/demo-te.safetensors", "folder": "text_encoders", "bytes": len(teData), "sha256": sum(teData)},
	}}
	writeJSON(t, filepath.Join(wf, "models.json"), models)
	writeJSON(t, filepath.Join(wf, "workflow.json"), map[string]any{"nodes": []any{
		map[string]any{"type": "VAELoader", "widgets_values": []any{"demo-vae.safetensors"}},
		map[string]any{"type": "CLIPLoader", "widgets_values": []any{"demo-te.safetensors"}},
	}})
	must(t, os.WriteFile(filepath.Join(wf, "README.md"), []byte("# demo\n\nA demo workflow.\n"), 0o644))

	s.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.downloads++
		status := s.status
		s.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/demo-vae.safetensors"):
			w.Write(vaeData)
		case strings.HasSuffix(r.URL.Path, "/demo-te.safetensors"):
			w.Write(teData)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(s.srv.Close)
	info := map[string]any{"VAELoader": map[string]any{"input": map[string]any{"required": map[string]any{
		"vae_name": []any{[]any{"demo-vae.safetensors"}}}}},
		"CLIPLoader": map[string]any{"input": map[string]any{"required": map[string]any{
			"clip_name": []any{[]any{"demo-te.safetensors"}}}}}}
	s.comfyUI = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(info)
	}))
	t.Cleanup(s.comfyUI.Close)
	return s
}

func (s *sandbox) requests() int { s.mu.Lock(); defer s.mu.Unlock(); return s.downloads }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	must(t, err)
	must(t, os.WriteFile(path, b, 0o644))
}

// seams sends every download to the fake server, with no waits and the given
// free space, and pins the time of a backup.
func (s *sandbox) seams(free uint64) seams {
	addr := s.srv.Listener.Addr().String()
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return seams{
		download: download.Options{Transport: tr, Root: s.root, Sleep: func(time.Duration) {},
			FreeSpace: func(string) (uint64, error) { return free, nil }},
		now: func() time.Time { return time.Date(2026, 11, 10, 9, 8, 7, 0, time.UTC) },
	}
}

// run runs horus workflow and returns the exit code, the lines and the result.
func (s *sandbox) run(t *testing.T, sm seams, args ...string) (int, []string, *provision.Result) {
	t.Helper()
	full := append(args, "-json", "-workflows-dir", s.workflows, "-comfyui-dir", s.comfy,
		"-models-dir", s.models, "-server-url", s.comfyUI.URL)
	var out bytes.Buffer
	code := cli.Workflow(context.Background(), full, &out, io.Discard, func(p cli.SetupPlace) cli.Setup { return newSetup(p, sm) })
	var lines []string
	var res *provision.Result
	for _, l := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if strings.HasPrefix(l, cli.Mark) {
			res = &provision.Result{}
			must(t, json.Unmarshal([]byte(l[len(cli.Mark):]), res))
			continue
		}
		lines = append(lines, l)
	}
	return code, lines, res
}

func statuses(res *provision.Result) string {
	var s []string
	for _, it := range res.Items {
		s = append(s, it.Status+" "+it.Kind+" "+it.Role)
	}
	return strings.Join(s, ", ")
}

func hasLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

// full is the full path of p, as the model files adapter prints it.
func full(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(p)
	must(t, err)
	return strings.TrimRight(filepath.Clean(a), `\`)
}

func TestInstallDownloadsMapsTheFolderAndChecks(t *testing.T) {
	s := newSandbox(t)
	yaml := filepath.Join(s.comfy, "ComfyUI", "extra_model_paths.yaml")
	old := "comfyui:\n    base_path: somewhere\n    checkpoints: models/checkpoints\n"
	must(t, os.WriteFile(yaml, []byte(old), 0o644))

	code, lines, res := s.run(t, s.seams(1<<50), "install", "demo")
	if code != 0 || res == nil || !res.OK {
		t.Fatalf("exit %d, result %+v\n%s", code, res, strings.Join(lines, "\n"))
	}
	want := "DOWNLOADED download vae, DOWNLOADED download text_encoder, OK file vae, OK file text_encoder, " +
		"OK graph , OK server , OK server vae, OK server text_encoder"
	if got := statuses(res); got != want {
		t.Errorf("items:\n got %s\nwant %s", got, want)
	}
	if res.Items[0].Detail != "downloaded and SHA-256 verified" || res.Message != "workflow 'demo' is installed and ready" {
		t.Errorf("detail %q, message %q", res.Items[0].Detail, res.Message)
	}
	vae := filepath.Join(full(t, s.models), "vae", "demo-vae.safetensors")
	if b, err := os.ReadFile(vae); err != nil || !bytes.Equal(b, vaeData) {
		t.Errorf("the VAE file on disk: %q, %v", b, err)
	}
	if !hasLine(lines, "-- vae -> "+vae) {
		t.Errorf("no line '-- vae -> %s':\n%s", vae, strings.Join(lines, "\n"))
	}
	// Both folders are outside ComfyUI/models, so both are added after a backup.
	base := strings.ReplaceAll(full(t, s.models), `\`, "/")
	wantYAML := strings.TrimRight(old, "\n") + "\n\nhorus_vae:\n    base_path: " + base + "\n    vae: vae\n" +
		"\nhorus_text_encoders:\n    base_path: " + base + "\n    text_encoders: text_encoders\n"
	if b, _ := os.ReadFile(yaml); string(b) != wantYAML {
		t.Errorf("extra_model_paths.yaml:\n%s\nwant:\n%s", b, wantYAML)
	}
	if b, err := os.ReadFile(yaml + ".bak-20261110-090807"); err != nil || string(b) != old {
		t.Errorf("backup: %q, %v", b, err)
	}
	if !hasLine(lines, "extra_model_paths.yaml: added vae -> "+base+"/vae") ||
		!hasLine(lines, "Restart ComfyUI so that it reads the new paths (horus start-comfyui).") {
		t.Errorf("lines:\n%s", strings.Join(lines, "\n"))
	}

	// A second install finds both files: PRESENT, no request, no change to the yaml file.
	n := s.requests()
	code, _, res = s.run(t, s.seams(1<<50), "install", "demo")
	if code != 0 || !strings.HasPrefix(statuses(res), "PRESENT download vae, PRESENT download text_encoder") || s.requests() != n {
		t.Errorf("second install: exit %d, %s, %d requests", code, statuses(res), s.requests()-n)
	}
	if b, _ := os.ReadFile(yaml); string(b) != wantYAML {
		t.Errorf("the second install changed extra_model_paths.yaml:\n%s", b)
	}
}

func TestInstallKeepsAFolderThatTheYAMLAlreadyMaps(t *testing.T) {
	s := newSandbox(t)
	yaml := filepath.Join(s.comfy, "ComfyUI", "extra_model_paths.yaml")
	text := "mine:\n    base_path: " + strings.ReplaceAll(s.models, `\`, "/") + "  # a comment\n    vae: |\n        vae\n        \"other\"\n    text_encoders: text_encoders\n"
	must(t, os.WriteFile(yaml, []byte(text), 0o644))
	code, lines, _ := s.run(t, s.seams(1<<50), "install", "demo")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, strings.Join(lines, "\n"))
	}
	if !hasLine(lines, "extra_model_paths.yaml already maps vae -> "+filepath.Join(full(t, s.models), "vae")) {
		t.Errorf("no 'already maps' line:\n%s", strings.Join(lines, "\n"))
	}
	if b, _ := os.ReadFile(yaml); string(b) != text {
		t.Errorf("extra_model_paths.yaml changed:\n%s", b)
	}
	if m, _ := filepath.Glob(yaml + ".bak-*"); len(m) != 0 {
		t.Errorf("a backup without a change: %v", m)
	}
}

// failureCases are the outcomes of a download that the MCP tool maps to an
// error code. want is in the detail of the FAILED item.
var failureCases = []struct {
	name   string
	status int
	free   uint64
	setup  func(t *testing.T, s *sandbox)
	want   []string
	kind   provision.DownloadKind
	code   string
}{
	{"401", 401, 1 << 50, nil, []string{"Attempt 1 of 4 did not finish (the server returned error: 401", "The server answered with an error.", "FAIL: Download did not finish after 4 attempts."},
		provision.DownloadUnauthorized, "HF_TOKEN_INVALID"},
	{"403", 403, 1 << 50, nil, []string{"error: 403", "FAIL: "}, provision.DownloadForbidden, "LICENSE_NOT_ACCEPTED"},
	{"disk full", 0, 10, nil, []string{"FAIL: Not enough free space on "}, provision.DownloadDiskFull, "DISK_FULL"},
	{"conflict", 0, 1 << 50, func(t *testing.T, s *sandbox) {
		p := filepath.Join(s.models, "vae", "demo-vae.safetensors")
		must(t, os.MkdirAll(filepath.Dir(p), 0o755))
		must(t, os.WriteFile(p, bytes.Repeat([]byte("x"), len(vaeData)), 0o644))
	}, []string{"FAIL: Refusing to overwrite "}, provision.DownloadConflict, "FILE_CONFLICT"},
}

func TestDownloadFailuresKeepTheirTextAndTheirKind(t *testing.T) {
	for _, c := range failureCases {
		t.Run(c.name, func(t *testing.T) {
			s := newSandbox(t)
			s.status = c.status
			if c.setup != nil {
				c.setup(t, s)
			}
			code, lines, res := s.run(t, s.seams(c.free), "install", "demo", "-only", "vae")
			if code != 1 || res == nil || res.OK {
				t.Fatalf("exit %d, result %+v", code, res)
			}
			if res.Message != "1 of 1 file(s) are not ready." || res.Items[0].Status != "FAILED" || res.Items[0].Kind != "download" {
				t.Fatalf("result %+v", res)
			}
			for _, w := range c.want {
				if !strings.Contains(res.Items[0].Detail, w) {
					t.Errorf("detail %q has no %q", res.Items[0].Detail, w)
				}
			}
			if !strings.Contains(res.Items[0].Detail, " | ") && c.status != 0 {
				t.Errorf("detail lines are not joined with ' | ': %q", res.Items[0].Detail)
			}
			if lines[len(lines)-1] != "FAIL: 1 of 1 file(s) are not ready." {
				t.Errorf("last line %q", lines[len(lines)-1])
			}

			// The service gets the kind of the failure as a typed value.
			res, err := newSetup(cli.SetupPlace{ComfyUIDir: s.comfy, ModelsDir: s.models, ServerURL: s.comfyUI.URL}, s.seams(c.free)).
				Install(context.Background(), provision.Request{WorkflowsDir: s.workflows, Workflow: "demo", Only: []string{"vae"}}, provision.Watch{})
			var de *provision.DownloadError
			if err == nil || !errors.As(res.Items[0].Err, &de) || de.Kind != c.kind {
				t.Errorf("the error of the item: %v (%v), want the kind %v", res.Items[0].Err, err, c.kind)
			}
		})
	}
}

func TestRemoveThatCannotDeleteGivesDeleteFailed(t *testing.T) {
	s := newSandbox(t)
	p := filepath.Join(s.models, "vae", "demo-vae.safetensors")
	must(t, os.MkdirAll(filepath.Dir(p), 0o755))
	must(t, os.WriteFile(p, vaeData, 0o644))
	undo, err := platform.BlockDelete(p)
	if errors.Is(err, platform.ErrCannotBlock) {
		t.Skip(err)
	}
	must(t, err)
	defer undo()
	code, lines, res := s.run(t, seams{}, "remove", "demo", "-yes")
	if code != 1 || res == nil || res.OK || res.Message != "1 file(s) were not deleted." {
		t.Fatalf("exit %d, result %+v", code, res)
	}
	if got := statuses(res); got != "ABSENT file text_encoder, DELETE_FAILED file vae" {
		t.Errorf("items %s", got)
	}
	if !strings.HasPrefix(lines[0], "ABSENT    text_encoder demo-te.safetensors") ||
		!strings.Contains(strings.Join(lines, "\n"), "(Is ComfyUI using it? Stop ComfyUI and try again.)") {
		t.Errorf("lines:\n%s", strings.Join(lines, "\n"))
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("the file is gone: %v", err)
	}
}

func TestRemoveDeletesAReadOnlyFile(t *testing.T) {
	s := newSandbox(t)
	p := filepath.Join(s.models, "vae", "demo-vae.safetensors")
	must(t, os.MkdirAll(filepath.Dir(p), 0o755))
	must(t, os.WriteFile(p, vaeData, 0o444))
	code, _, res := s.run(t, seams{}, "remove", "demo", "-yes", "-only", "vae")
	if code != 0 || statuses(res) != "DELETED file vae" {
		t.Fatalf("exit %d, %s", code, statuses(res))
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("the read-only file is still there: %v", err)
	}
}

func TestFlagsAndExitCodes(t *testing.T) {
	s := newSandbox(t)
	sm := s.seams(1 << 50)
	setup := func(p cli.SetupPlace) cli.Setup { return newSetup(p, sm) }
	cases := []struct {
		args []string
		code int
	}{
		{[]string{}, 2},
		{[]string{"nope"}, 2},
		{[]string{"list", "a", "b"}, 2},
		{[]string{"-no-such-flag", "list"}, 2},
		{[]string{"-h"}, 0},
	}
	for _, c := range cases {
		if code := cli.Workflow(context.Background(), c.args, io.Discard, io.Discard, setup); code != c.code {
			t.Errorf("%v: exit %d, want %d", c.args, code, c.code)
		}
	}
	// The flags may come before and after the name; -only takes a list and repeats.
	code, lines, res := s.run(t, sm, "-quick", "CHECK", "demo", "-only", "vae,nothing", "-only", "TEXT_ENCODER")
	if res == nil || res.Action != "check" || !hasLine(lines, "Checking workflow 'demo': 2 model file(s), size only.") {
		t.Fatalf("exit %d, result %+v\n%s", code, res, strings.Join(lines, "\n"))
	}
	code, _, res = s.run(t, sm, "check", "demo", "-only", "nothing")
	if code != 1 || res.Message != "-only nothing matches no file. Roles: vae, text_encoder" {
		t.Errorf("exit %d, message %q", code, res.Message)
	}
	code, _, res = s.run(t, sm, "check")
	if code != 1 || res.Message != "Name a workflow. Run 'horus workflow list' to see them." {
		t.Errorf("exit %d, message %q", code, res.Message)
	}
	code, _, res = s.run(t, sm, "check", "other")
	if code != 1 || res.Message != "No workflow 'other'. Known: demo" {
		t.Errorf("exit %d, message %q", code, res.Message)
	}
	code, _, res = s.run(t, sm, "check", "../demo")
	if code != 1 || res.Message != "Invalid workflow name: ../demo" {
		t.Errorf("exit %d, message %q", code, res.Message)
	}
}

// fakeImages is a fake of the image service for the model tools: the
// backend is down or up, and FreeMemory counts its calls.
type fakeImages struct {
	mu      sync.Mutex
	running bool
	nFrees  int
}

func (f *fakeImages) Generate(context.Context, generate.Request, generate.Options) (*generate.Result, error) {
	return nil, errors.New("not in this test")
}
func (f *fakeImages) Running(context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running, nil
}
func (f *fakeImages) FreeMemory(context.Context) { f.mu.Lock(); f.nFrees++; f.mu.Unlock() }
func (f *fakeImages) setRunning(v bool)          { f.mu.Lock(); f.running = v; f.mu.Unlock() }
func (f *fakeImages) frees() int                 { f.mu.Lock(); defer f.mu.Unlock(); return f.nFrees }

// toolSandbox copies workflows/it-illustration into a temporary folder,
// writes small stand-in model files and a models.json with their sizes and
// SHA-256, and points the settings at it.
type toolSandbox struct {
	dir, models string
	files       []string // the stand-in model files
}

func newToolSandbox(t *testing.T) *toolSandbox {
	t.Helper()
	dir := t.TempDir()
	s := &toolSandbox{dir: dir, models: filepath.Join(dir, "comfy", "ComfyUI", "models")}
	src := filepath.Join("..", "..", "workflows", "it-illustration")
	dst := filepath.Join(dir, "workflows", "it-illustration")
	must(t, os.MkdirAll(dst, 0o755))
	for _, name := range []string{"manifest.json", "workflow.json"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		must(t, err)
		must(t, os.WriteFile(filepath.Join(dst, name), data, 0o644))
	}
	var m map[string]any
	data, _ := os.ReadFile(filepath.Join(src, "models.json"))
	must(t, json.Unmarshal(data, &m))
	for _, x := range m["files"].([]any) {
		f := x.(map[string]any)
		parts := strings.Split(f["repo_path"].(string), "/")
		body := []byte("stand-in for " + f["repo_path"].(string))
		p := filepath.Join(s.models, f["folder"].(string), parts[len(parts)-1])
		must(t, os.MkdirAll(filepath.Dir(p), 0o755))
		must(t, os.WriteFile(p, body, 0o644))
		f["bytes"], f["sha256"] = len(body), sum(body)
		s.files = append(s.files, p)
	}
	out, _ := json.Marshal(m)
	must(t, os.WriteFile(filepath.Join(dst, "models.json"), out, 0o644))
	must(t, os.WriteFile(filepath.Join(dir, "comfy", "ComfyUI", "main.py"), []byte("# fake\n"), 0o644))

	t.Setenv("HORUS_ROOT", dir)
	t.Setenv("HORUS_WORKFLOWS_DIR", filepath.Join(dir, "workflows"))
	t.Setenv("HORUS_COMFYUI_DIR", filepath.Join(dir, "comfy"))
	t.Setenv("HORUS_MODELS_DIR", s.models)
	t.Setenv("HORUS_TEXT_ENCODER_DIR", filepath.Join(s.models, "text_encoders"))
	t.Setenv("HORUS_COMFYUI_URL", "http://127.0.0.1:9") // nothing answers: list says backend_running false
	t.Setenv("HORUS_AUTOSTART", "0")
	settings.Reset()
	t.Cleanup(settings.Reset)
	return s
}

func (s *toolSandbox) present() int {
	n := 0
	for _, p := range s.files {
		if _, err := os.Stat(p); err == nil {
			n++
		}
	}
	return n
}

func fileStates(files any) string {
	var out []string
	for _, x := range files.([]any) {
		out = append(out, x.(map[string]any)["state"].(string))
	}
	return strings.Join(out, ",")
}

// TestModelToolsOnASandbox runs the four model tools through MCP on the
// setup service of the wiring, as horus mcp does.
func TestModelToolsOnASandbox(t *testing.T) {
	s := newToolSandbox(t)
	images := &fakeImages{} // the backend is down: list says backend_running false
	cs := connect(t, mcpserver.New(images, newSetup(cli.SetupPlace{}, seams{})), nil)

	env := envelopeOf(t, callTool(t, cs, "list_workflows", map[string]any{}))
	r, _ := env["result"].(map[string]any)
	if env["ok"] != true || r["backend_running"] != false {
		t.Fatalf("list: %v", env)
	}
	rows := r["workflows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["install_state"] != "installed" || rows[0].(map[string]any)["download_bytes"] != float64(172) {
		t.Fatalf("list rows: %v", rows)
	}
	images.setRunning(true)
	env = envelopeOf(t, callTool(t, cs, "list_workflows", map[string]any{}))
	if r, _ := env["result"].(map[string]any); env["ok"] != true || r["backend_running"] != true {
		t.Fatalf("list with the backend up: %v", env)
	}

	res := callTool(t, cs, "describe_workflow", map[string]any{"workflow": "it-illustration"})
	env = envelopeOf(t, res)
	r, _ = env["result"].(map[string]any)
	if env["ok"] != true || r["install_state"] != "installed" || fileStates(r["files"]) != "present,present,present" {
		t.Fatalf("describe: %v", env)
	}

	env = envelopeOf(t, callTool(t, cs, "install_workflow", map[string]any{"workflow": "it-illustration"}))
	r, _ = env["result"].(map[string]any)
	if env["ok"] != true || r["already_installed"] != true || fileStates(r["files"]) != "present,present,present" {
		t.Fatalf("install of an installed workflow: %v", env)
	}

	env = envelopeOf(t, callTool(t, cs, "uninstall_workflow", map[string]any{"workflow": "it-illustration"}))
	e, _ := env["error"].(map[string]any)
	d, _ := e["details"].(map[string]any)
	if e["code"] != "CONFIRMATION_REQUIRED" || fileStates(d["files"]) != "would_delete,would_delete,would_delete" ||
		d["would_free_bytes"] != float64(172) || s.present() != 3 || images.frees() != 0 {
		t.Fatalf("uninstall plan: %v, %d files on disk, %d frees", env, s.present(), images.frees())
	}

	env = envelopeOf(t, callTool(t, cs, "uninstall_workflow", map[string]any{"workflow": "it-illustration", "confirm": true}))
	r, _ = env["result"].(map[string]any)
	if env["ok"] != true || fileStates(r["files"]) != "deleted,deleted,deleted" || r["freed_bytes"] != float64(172) || s.present() != 0 {
		t.Fatalf("uninstall: %v, %d files on disk", env, s.present())
	}
	if images.frees() != 1 {
		t.Fatalf("a confirmed uninstall frees the models first: %d frees", images.frees())
	}

	env = envelopeOf(t, callTool(t, cs, "list_workflows", map[string]any{}))
	rows = env["result"].(map[string]any)["workflows"].([]any)
	if rows[0].(map[string]any)["install_state"] != "not_installed" {
		t.Fatalf("list after uninstall: %v", rows)
	}

	env = envelopeOf(t, callTool(t, cs, "install_workflow", map[string]any{"workflow": "it-illustration"}))
	e, _ = env["error"].(map[string]any)
	d, _ = e["details"].(map[string]any)
	if e["code"] != "LICENSE_CONFIRMATION_REQUIRED" || e["action_by"] != "human" || d["download_bytes"] != float64(172) ||
		e["message"] != "installing it-illustration downloads 0.00 MB under the license 'FLUX Kommunity License (gated: accept it on the repo page first)'" {
		t.Fatalf("install without accept_license: %v", env)
	}
}

// TestInstallToolGivesTheCodeOfAFailedDownload proves the typed path from
// the downloader to the error code of install_workflow: HTTP 401, HTTP 403,
// a full disk and a file conflict.
func TestInstallToolGivesTheCodeOfAFailedDownload(t *testing.T) {
	for _, c := range failureCases {
		t.Run(c.name, func(t *testing.T) {
			s := newSandbox(t)
			s.status = c.status
			if c.setup != nil {
				c.setup(t, s)
			}
			// The demo workflow, offered by the server: a copy of the manifest of it-illustration.
			data, err := os.ReadFile(filepath.Join("..", "..", "workflows", "it-illustration", "manifest.json"))
			must(t, err)
			var m map[string]any
			must(t, json.Unmarshal(data, &m))
			m["name"] = "demo"
			writeJSON(t, filepath.Join(s.workflows, "demo", "manifest.json"), m)
			t.Setenv("HORUS_WORKFLOWS_DIR", s.workflows)
			settings.Reset()
			setup := newSetup(cli.SetupPlace{ComfyUIDir: s.comfy, ModelsDir: s.models, ServerURL: s.comfyUI.URL}, s.seams(c.free))
			cs := connect(t, mcpserver.New(&fakeImages{}, setup), nil)
			env := envelopeOf(t, callTool(t, cs, "install_workflow", map[string]any{"workflow": "demo", "accept_license": true}))
			e, _ := env["error"].(map[string]any)
			d, _ := e["details"].(map[string]any)
			if e["code"] != c.code || d["file"] != "demo-vae.safetensors" {
				t.Fatalf("got %v, want the code %s for demo-vae.safetensors", env, c.code)
			}
			if c.code == "LICENSE_NOT_ACCEPTED" && d["license"] != "test license" {
				t.Errorf("the details name no license: %v", d)
			}
		})
	}
}

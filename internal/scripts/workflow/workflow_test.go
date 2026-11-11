// Tests of horus workflow for the parts that tests/workflow_parity.py cannot
// reach without a real download: DOWNLOADED, FAILED with the text that
// horus/models.py reads, the mapping in extra_model_paths.yaml, and
// DELETE_FAILED. Every download goes to a local fake HTTPS server; no test
// reaches the network.
package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/scripts/downloadmodel"
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
	status                         int // HTTP status of the fake download server; 0 = 200
	downloads                      int
	srv                            *httptest.Server
	comfyUI                        *httptest.Server
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HORUS_ROOT", root) // no .env of a real checkout is read
	for _, k := range []string{"HORUS_MODELS_DIR", "HORUS_TEXT_ENCODER_DIR", "HORUS_COMFYUI_DIR", "HORUS_COMFYUI_URL", "HF_TOKEN"} {
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
		s.downloads++
		if s.status != 0 {
			w.WriteHeader(s.status)
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

// options sends every download to the fake server, with no waits and enough free space.
func (s *sandbox) options(free uint64) Options {
	addr := s.srv.Listener.Addr().String()
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return Options{
		Download: func(ctx context.Context, o downloadmodel.Options) (downloadmodel.Result, error) {
			o.Transport, o.Root = tr, s.root
			o.Sleep = func(time.Duration) {}
			o.FreeSpace = func(string) (uint64, error) { return free, nil }
			return downloadmodel.Download(ctx, o)
		},
		Now: func() time.Time { return time.Date(2026, 11, 10, 9, 8, 7, 0, time.UTC) },
	}
}

// run runs horus workflow and returns the exit code, the lines and the result.
func (s *sandbox) run(t *testing.T, opt Options, args ...string) (int, []string, *Result) {
	t.Helper()
	full := append(args, "-json", "-workflows-dir", s.workflows, "-comfyui-dir", s.comfy,
		"-models-dir", s.models, "-server-url", s.comfyUI.URL)
	var out bytes.Buffer
	code := Main(context.Background(), full, &out, opt)
	var lines []string
	var res *Result
	for _, l := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if strings.HasPrefix(l, Mark) {
			res = &Result{}
			must(t, json.Unmarshal([]byte(l[len(Mark):]), res))
			continue
		}
		lines = append(lines, l)
	}
	return code, lines, res
}

func statuses(res *Result) string {
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

func TestInstallDownloadsMapsTheFolderAndChecks(t *testing.T) {
	s := newSandbox(t)
	yaml := filepath.Join(s.comfy, "ComfyUI", "extra_model_paths.yaml")
	old := "comfyui:\n    base_path: somewhere\n    checkpoints: models/checkpoints\n"
	must(t, os.WriteFile(yaml, []byte(old), 0o644))

	code, lines, res := s.run(t, s.options(1<<50), "install", "demo")
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
	vae := filepath.Join(fullPath(s.models), "vae", "demo-vae.safetensors")
	if b, err := os.ReadFile(vae); err != nil || !bytes.Equal(b, vaeData) {
		t.Errorf("the VAE file on disk: %q, %v", b, err)
	}
	if !hasLine(lines, "-- vae -> "+vae) {
		t.Errorf("no line '-- vae -> %s' (horus/models.py reads it):\n%s", vae, strings.Join(lines, "\n"))
	}
	// Both folders are outside ComfyUI/models, so both are added after a backup.
	base := strings.ReplaceAll(fullPath(s.models), `\`, "/")
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
	n := s.downloads
	code, _, res = s.run(t, s.options(1<<50), "install", "demo")
	if code != 0 || !strings.HasPrefix(statuses(res), "PRESENT download vae, PRESENT download text_encoder") || s.downloads != n {
		t.Errorf("second install: exit %d, %s, %d requests", code, statuses(res), s.downloads-n)
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
	code, lines, _ := s.run(t, s.options(1<<50), "install", "demo")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, strings.Join(lines, "\n"))
	}
	if !hasLine(lines, "extra_model_paths.yaml already maps vae -> "+filepath.Join(fullPath(s.models), "vae")) {
		t.Errorf("no 'already maps' line:\n%s", strings.Join(lines, "\n"))
	}
	if b, _ := os.ReadFile(yaml); string(b) != text {
		t.Errorf("extra_model_paths.yaml changed:\n%s", b)
	}
	if m, _ := filepath.Glob(yaml + ".bak-*"); len(m) != 0 {
		t.Errorf("a backup without a change: %v", m)
	}
}

func TestDownloadFailuresKeepTheTextThatModelsPyReads(t *testing.T) {
	cases := []struct {
		name   string
		status int
		free   uint64
		setup  func(s *sandbox)
		want   []string // in the detail of the first FAILED item
	}{
		{"401", 401, 1 << 50, nil, []string{"Attempt 1 of 4 did not finish (the server returned error: 401", "The server answered with an error.", "FAIL: Download did not finish after 4 attempts."}},
		{"403", 403, 1 << 50, nil, []string{"error: 403", "FAIL: "}},
		{"disk full", 0, 10, nil, []string{"FAIL: Not enough free space on "}},
		{"conflict", 0, 1 << 50, func(s *sandbox) {
			p := filepath.Join(s.models, "vae", "demo-vae.safetensors")
			must(t, os.MkdirAll(filepath.Dir(p), 0o755))
			must(t, os.WriteFile(p, bytes.Repeat([]byte("x"), len(vaeData)), 0o644))
		}, []string{"FAIL: Refusing to overwrite "}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSandbox(t)
			s.status = c.status
			if c.setup != nil {
				c.setup(s)
			}
			code, lines, res := s.run(t, s.options(c.free), "install", "demo", "-only", "vae")
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
	code, lines, res := s.run(t, Options{}, "remove", "demo", "-yes")
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
	code, _, res := s.run(t, Options{}, "remove", "demo", "-yes", "-only", "vae")
	if code != 0 || statuses(res) != "DELETED file vae" {
		t.Fatalf("exit %d, %s", code, statuses(res))
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("the read-only file is still there: %v", err)
	}
}

func TestFlagsAndExitCodes(t *testing.T) {
	s := newSandbox(t)
	opt := s.options(1 << 50)
	var out bytes.Buffer
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
		if code := Main(context.Background(), c.args, &out, opt); code != c.code {
			t.Errorf("%v: exit %d, want %d", c.args, code, c.code)
		}
	}
	// The flags may come before and after the name; -only takes a list and repeats.
	code, lines, res := s.run(t, opt, "-quick", "CHECK", "demo", "-only", "vae,nothing", "-only", "TEXT_ENCODER")
	if res == nil || res.Action != "check" || !hasLine(lines, "Checking workflow 'demo': 2 model file(s), size only.") {
		t.Fatalf("exit %d, result %+v\n%s", code, res, strings.Join(lines, "\n"))
	}
	code, lines, res = s.run(t, opt, "check", "demo", "-only", "nothing")
	if code != 1 || res.Message != "-only nothing matches no file. Roles: vae, text_encoder" {
		t.Errorf("exit %d, message %q", code, res.Message)
	}
	code, _, res = s.run(t, opt, "check")
	if code != 1 || res.Message != "Name a workflow. Run 'horus workflow list' to see them." {
		t.Errorf("exit %d, message %q", code, res.Message)
	}
	code, _, res = s.run(t, opt, "check", "other")
	if code != 1 || res.Message != "No workflow 'other'. Known: demo" {
		t.Errorf("exit %d, message %q", code, res.Message)
	}
	code, _, res = s.run(t, opt, "check", "../demo")
	if code != 1 || res.Message != "Invalid workflow name: ../demo" {
		t.Errorf("exit %d, message %q", code, res.Message)
	}
}

func TestYAMLFolders(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "extra_model_paths.yaml")
	abs := filepath.Join(dir, "abs")
	text := "# comment\ncomfyui:\n    base_path: '" + dir + "'\n    is_default: true\n    vae: models/vae  # one\n" +
		"    loras: |\n        models/loras\n        \"" + abs + "\"\n\n" +
		"other:\n    vae: relative-without-base\n"
	must(t, os.WriteFile(p, []byte(text), 0o644))
	got := yamlFolders(p)
	want := map[string][]string{
		"vae":   {fullPath(filepath.Join(dir, "models/vae")), fullPath("relative-without-base")},
		"loras": {fullPath(filepath.Join(dir, "models/loras")), fullPath(abs)},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

func TestNumbersAsTheScriptPrintsThem(t *testing.T) {
	// The values that workflow.ps1 (on .NET Framework) printed for "{0:N2}" and [math]::Round(x, 2).
	for b, want := range map[int64]string{0: "0.00", 172: "0.00", 5_827_874_779: "5.83", 1_234_567_890_123: "1,234.57",
		999_995_000_000: "1,000.00", 1_005_000_000: "1.01", 2_675_000_000: "2.68", 4_000_000: "0.00", 5_000_000: "0.01"} {
		if got := gb2(b); got != want {
			t.Errorf("gb2(%d) = %q, want %q", b, got, want)
		}
	}
	for b, want := range map[int64]float64{0: 0, 5_827_874_779: 5.83, 1_234_567_890_123: 1234.57, 999_995_000_000: 1000, 1_005_000_000: 1, 2_675_000_000: 2.68} {
		if got := roundGB(b); got != want {
			t.Errorf("roundGB(%d) = %v, want %v", b, got, want)
		}
	}
}

// Tests of the tools list_workflows, describe_workflow, install_workflow and
// uninstall_workflow through MCP. The test binary is also the `horus workflow`
// child, so every call starts a real child process. All files are in a
// temporary sandbox. Part of the port of horus/server.py.
package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/models"
	"github.com/hieplam/horus/internal/scripts/workflow"
	"github.com/hieplam/horus/internal/settings"
)

// When HORUS_FAKE_WORKFLOW is set, TestMain acts as `horus workflow`. With
// the value "real" it runs the real command (internal/scripts/workflow). With
// "sleep", an install starts a grandchild that shares stdout, prints the line
// of the file that it downloads, writes the .part file and sleeps 60 s; the
// other actions run the real command.
func TestMain(m *testing.M) {
	mode := os.Getenv("HORUS_FAKE_WORKFLOW")
	switch {
	case mode == "":
		os.Exit(m.Run())
	case mode == "leaf":
		time.Sleep(60 * time.Second)
		os.Exit(0)
	case len(os.Args) < 3 || os.Args[1] != "workflow":
		fmt.Fprintln(os.Stderr, "the fake is only horus workflow:", os.Args)
		os.Exit(3)
	case mode == "sleep" && os.Args[2] == "install":
		leaf := exec.Command(os.Args[0])
		leaf.Env = append(os.Environ(), "HORUS_FAKE_WORKFLOW=leaf")
		leaf.Stdout, leaf.Stderr = os.Stdout, os.Stderr
		if err := leaf.Start(); err != nil {
			os.Exit(1)
		}
		dest := os.Getenv("HORUS_FAKE_DEST")
		fmt.Println("-- diffusion -> " + dest)
		os.WriteFile(dest+".part", make([]byte, 2_500_000), 0o644)
		time.Sleep(60 * time.Second)
		os.Exit(0)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(workflow.Main(ctx, os.Args[2:], os.Stdout, workflow.Options{}))
}

// lockedBuffer is a log target that the test reads while the server writes.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuffer) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

type sandbox struct {
	dir, models string
	files       []string // the stand-in model files
}

// newSandbox copies workflows/it-illustration into a temporary folder, writes
// small stand-in model files and a models.json with their sizes and SHA-256,
// and makes the test binary the `horus workflow` child.
func newSandbox(t *testing.T, mode string) *sandbox {
	t.Helper()
	dir := t.TempDir()
	s := &sandbox{dir: dir, models: filepath.Join(dir, "comfy", "ComfyUI", "models")}
	src := filepath.Join("..", "..", "workflows", "it-illustration")
	dst := filepath.Join(dir, "workflows", "it-illustration")
	os.MkdirAll(dst, 0o755)
	for _, name := range []string{"manifest.json", "workflow.json"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dst, name), data, 0o644)
	}
	var m map[string]any
	data, _ := os.ReadFile(filepath.Join(src, "models.json"))
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, x := range m["files"].([]any) {
		f := x.(map[string]any)
		parts := strings.Split(f["repo_path"].(string), "/")
		body := []byte("stand-in for " + f["repo_path"].(string))
		p := filepath.Join(s.models, f["folder"].(string), parts[len(parts)-1])
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, body, 0o644)
		sum := sha256.Sum256(body)
		f["bytes"], f["sha256"] = len(body), hex.EncodeToString(sum[:])
		s.files = append(s.files, p)
	}
	out, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(dst, "models.json"), out, 0o644)
	os.WriteFile(filepath.Join(dir, "comfy", "ComfyUI", "main.py"), []byte("# fake\n"), 0o644)

	t.Setenv("HORUS_ROOT", dir)
	t.Setenv("HORUS_WORKFLOWS_DIR", filepath.Join(dir, "workflows"))
	t.Setenv("HORUS_COMFYUI_DIR", filepath.Join(dir, "comfy"))
	t.Setenv("HORUS_MODELS_DIR", s.models)
	t.Setenv("HORUS_TEXT_ENCODER_DIR", filepath.Join(s.models, "text_encoders"))
	t.Setenv("HORUS_COMFYUI_URL", "http://127.0.0.1:9") // nothing answers: list says backend_running false
	t.Setenv("HORUS_AUTOSTART", "0")
	t.Setenv("HORUS_FAKE_WORKFLOW", mode)
	settings.Reset()
	t.Cleanup(settings.Reset)
	old := models.Exe
	models.Exe = func() (string, error) { return os.Args[0], nil }
	t.Cleanup(func() { models.Exe = old })
	logTo(&lockedBuffer{})
	t.Cleanup(func() { logTo(os.Stderr) })
	return s
}

func (s *sandbox) present() int {
	n := 0
	for _, p := range s.files {
		if _, err := os.Stat(p); err == nil {
			n++
		}
	}
	return n
}

func states(files any) string {
	var out []string
	for _, x := range files.([]any) {
		out = append(out, x.(map[string]any)["state"].(string))
	}
	return strings.Join(out, ",")
}

func TestModelToolsOnASandbox(t *testing.T) {
	s := newSandbox(t, "real")
	cs := connect(t, New(), nil)

	env := envelopeOf(t, callTool(t, cs, "list_workflows", map[string]any{}))
	r, _ := env["result"].(map[string]any)
	if env["ok"] != true || r["backend_running"] != false {
		t.Fatalf("list: %v", env)
	}
	rows := r["workflows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["install_state"] != "installed" || rows[0].(map[string]any)["download_bytes"] != float64(172) {
		t.Fatalf("list rows: %v", rows)
	}

	res := callTool(t, cs, "describe_workflow", map[string]any{"workflow": "it-illustration"})
	env = envelopeOf(t, res)
	r, _ = env["result"].(map[string]any)
	if env["ok"] != true || r["install_state"] != "installed" || states(r["files"]) != "present,present,present" {
		t.Fatalf("describe: %v", env)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	for _, word := range []string{"workflow.json", "KSampler", "CLIPTextEncode", "class_type"} {
		if strings.Contains(text, word) {
			t.Errorf("describe_workflow names %s", word)
		}
	}

	env = envelopeOf(t, callTool(t, cs, "install_workflow", map[string]any{"workflow": "it-illustration"}))
	r, _ = env["result"].(map[string]any)
	if env["ok"] != true || r["already_installed"] != true || states(r["files"]) != "present,present,present" {
		t.Fatalf("install of an installed workflow: %v", env)
	}

	env = envelopeOf(t, callTool(t, cs, "uninstall_workflow", map[string]any{"workflow": "it-illustration"}))
	e, _ := env["error"].(map[string]any)
	d, _ := e["details"].(map[string]any)
	if e["code"] != "CONFIRMATION_REQUIRED" || states(d["files"]) != "would_delete,would_delete,would_delete" ||
		d["would_free_bytes"] != float64(172) || s.present() != 3 {
		t.Fatalf("uninstall plan: %v, %d files on disk", env, s.present())
	}

	env = envelopeOf(t, callTool(t, cs, "uninstall_workflow", map[string]any{"workflow": "it-illustration", "confirm": true}))
	r, _ = env["result"].(map[string]any)
	if env["ok"] != true || states(r["files"]) != "deleted,deleted,deleted" || r["freed_bytes"] != float64(172) || s.present() != 0 {
		t.Fatalf("uninstall: %v, %d files on disk", env, s.present())
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

func TestSizeText(t *testing.T) {
	for n, want := range map[int64]string{172: "0.00 MB", 99_994_999: "99.99 MB", 100_000_000: "0.1 GB", 11_022_645_311: "11.0 GB", 5_050_000_000: "5.0 GB"} {
		if got := sizeText(n); got != want {
			t.Errorf("sizeText(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestInstallReportsProgressAndACancelStopsTheChild runs an install whose
// child sleeps and whose grandchild holds the stdout pipe. The tool reports
// the size of the .part file. A cancel of the call kills the process tree:
// the tool returns long before WaitDelay (30 s here), and the .part file
// stays.
func TestInstallReportsProgressAndACancelStopsTheChild(t *testing.T) {
	s := newSandbox(t, "sleep")
	for _, p := range s.files {
		os.Remove(p)
	}
	dest := filepath.Join(s.models, "diffusion_models", "flux3-image-klein-int8-convrot.safetensors")
	t.Setenv("HORUS_FAKE_DEST", dest)
	log := &lockedBuffer{}
	logTo(log)
	oldTick, oldWait := ProgressInterval, models.WaitDelay
	ProgressInterval, models.WaitDelay = 20*time.Millisecond, 30*time.Second
	t.Cleanup(func() { ProgressInterval, models.WaitDelay = oldTick, oldWait })

	got := make(chan string, 100)
	cs := connect(t, New(), &mcp.ClientOptions{ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
		select {
		case got <- fmt.Sprintf("%v/%v %s", r.Params.Progress, r.Params.Total, r.Params.Message):
		default:
		}
	}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	params := &mcp.CallToolParams{Name: "install_workflow", Arguments: map[string]any{"workflow": "it-illustration", "accept_license": true}}
	params.SetProgressToken("tok-install")
	done := make(chan error, 1)
	go func() {
		_, err := cs.CallTool(ctx, params)
		done <- err
	}()
	want := "2.5e+06/172 downloading flux3-image-klein-int8-convrot.safetensors: 0.00 GB"
	deadline := time.After(30 * time.Second)
	for seen := false; !seen; {
		select {
		case m := <-got:
			seen = m == want
		case err := <-done:
			t.Fatalf("the call ended before a progress report: %v\n%s", err, log)
		case <-deadline:
			t.Fatalf("no progress report %q\n%s", want, log)
		}
	}
	cancelled := time.Now()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("CallTool: %v", err)
	}
	for !strings.Contains(log.String(), "call install_workflow crashed\ncontext canceled") {
		if time.Since(cancelled) > 10*time.Second {
			t.Fatalf("the tool did not return 10 s after the cancel: a process of the tree kept the pipe open\n%s", log)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(dest + ".part"); err != nil {
		t.Errorf("the .part file must stay: %v", err)
	}
}

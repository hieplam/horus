package checksetup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The test binary is also the fake "horus workflow check" program: when
// HORUS_FAKE_WORKFLOW is set, TestMain acts as that program. It records its
// arguments and two environment variables, prints two text lines and the
// HORUS_JSON line, and exits with HORUS_FAKE_EXIT. It runs on every system.
func TestMain(m *testing.M) {
	if rec := os.Getenv("HORUS_FAKE_WORKFLOW"); rec != "" {
		os.WriteFile(rec, []byte("args "+strings.Join(os.Args[1:], " ")+"\ncomfyui "+os.Getenv("HORUS_COMFYUI_DIR")+
			"\nmodels "+os.Getenv("HORUS_MODELS_DIR")+"\n"), 0o644)
		fmt.Println("OK        vae vae/x.safetensors")
		fmt.Println("MISSING   unet unet/y.safetensors")
		fmt.Println(`HORUS_JSON {"ok":false}`)
		n, _ := strconv.Atoi(os.Getenv("HORUS_FAKE_EXIT"))
		os.Exit(n)
	}
	os.Exit(m.Run())
}

type fakeModels struct {
	ok    bool
	err   error
	asked []string
}

func (f *fakeModels) Check(ctx context.Context, workflow string) ([]string, bool, error) {
	f.asked = append(f.asked, workflow)
	return []string{"OK        vae", "MISSING   unet"}, f.ok, f.err
}

type rig struct {
	opts Options
	out  *bytes.Buffer
}

// newRig has a ComfyUI folder, a fake server, a venv with comfy-mcp, a
// .mcp.json and a PATH that holds horus. Every check passes.
func newRig(t *testing.T) *rig {
	t.Helper()
	tmp := t.TempDir()
	comfy := filepath.Join(tmp, "comfy")
	os.MkdirAll(filepath.Join(comfy, "ComfyUI"), 0o755)
	os.WriteFile(filepath.Join(comfy, "ComfyUI", "main.py"), []byte("x"), 0o644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/system_stats" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"system":{"comfyui_version":"0.9.9"}}`)
	}))
	t.Cleanup(srv.Close)

	venv := filepath.Join(tmp, "venv")
	os.MkdirAll(venvBin(venv), 0o755)
	mcp := filepath.Join(venvBin(venv), exe("comfy-mcp"))
	os.WriteFile(mcp, []byte("x"), 0o755)
	os.WriteFile(filepath.Join(venvBin(venv), exe("comfy")), []byte("x"), 0o755)

	bin := filepath.Join(tmp, "bin")
	os.MkdirAll(bin, 0o755)
	horus := filepath.Join(bin, exe("horus"))
	os.WriteFile(horus, []byte("x"), 0o755)

	repo := filepath.Join(tmp, "repo")
	os.MkdirAll(repo, 0o755)
	cfg, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{
		"comfy-mcp": map[string]any{"command": mcp},
		"horus":     map[string]any{"command": "horus", "args": []string{"mcp"}},
	}})
	os.WriteFile(filepath.Join(repo, ".mcp.json"), cfg, 0o644)

	r := &rig{out: &bytes.Buffer{}}
	r.opts = Options{ComfyUIDir: comfy, ServerURL: srv.URL, VenvPath: venv, RepoRoot: repo, Home: tmp, Stdout: r.out,
		LookPath: func(name string) (string, error) {
			if name == "horus" {
				return horus, nil
			}
			return "", errors.New("not found")
		}}
	return r
}

func lines(r *rig) []string { return strings.Split(strings.TrimSpace(r.out.String()), "\n") }

func TestAllChecksPass(t *testing.T) {
	r := newRig(t)
	if failed := Check(context.Background(), r.opts); failed != 0 {
		t.Fatalf("failed %d:\n%s", failed, r.out)
	}
	ls := lines(r)
	if len(ls) != 8 || ls[7] != "All checks passed." {
		t.Fatalf("output:\n%s", r.out)
	}
	for _, l := range ls[:7] {
		if !strings.HasPrefix(l, "PASS  ") {
			t.Fatalf("not a PASS line: %q", l)
		}
	}
	if !strings.Contains(ls[1], "(ComfyUI 0.9.9)") {
		t.Fatalf("server line: %q", ls[1])
	}
	if !strings.HasPrefix(ls[3], "PASS  comfy-cli - ") {
		t.Fatalf("comfy-cli line: %q", ls[3])
	}
}

// A venv that has comfy-mcp but no comfy program: the comfy MCP server fails
// when it calls comfy-cli, so the check must fail by the name "comfy-cli".
func TestMissingComfyCliFails(t *testing.T) {
	r := newRig(t)
	os.Remove(filepath.Join(venvBin(r.opts.VenvPath), exe("comfy")))
	failed := Check(context.Background(), r.opts)
	text := r.out.String()
	if failed != 1 || !strings.Contains(text, "FAIL  comfy-cli - ") || !strings.Contains(text, "1 check(s) failed.") {
		t.Fatalf("failed=%d:\n%s", failed, text)
	}
}

// The repo .mcp.json starts comfy-mcp from a path with ${USERPROFILE} and has
// COMFY_BIN and ${VAR:-default} in its env. Both must expand.
func TestMcpJSONExpandsVariablesAndChecksComfyBin(t *testing.T) {
	r := newRig(t)
	rel, _ := filepath.Rel(r.opts.Home, venvBin(r.opts.VenvPath))
	rel = filepath.ToSlash(rel)
	r.opts.Getenv = func(n string) string {
		if n == "USERPROFILE" {
			return r.opts.Home
		}
		return ""
	}
	write := func(comfyBin string) {
		cfg, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{
			"comfy-mcp": map[string]any{"command": "${USERPROFILE}/" + rel + "/" + exe("comfy-mcp"),
				"env": map[string]any{"COMFY_BIN": comfyBin, "COMFY_LOCAL_URL": "${COMFY_LOCAL_URL:-http://127.0.0.1:8188}"}},
			"horus": map[string]any{"command": "horus"},
		}})
		os.WriteFile(filepath.Join(r.opts.RepoRoot, ".mcp.json"), cfg, 0o644)
	}
	write("${USERPROFILE}/" + rel + "/" + exe("comfy"))
	if failed := Check(context.Background(), r.opts); failed != 0 {
		t.Fatalf("failed %d:\n%s", failed, r.out)
	}
	if !strings.Contains(r.out.String(), "COMFY_BIN at ") {
		t.Fatalf("no COMFY_BIN in:\n%s", r.out)
	}

	// COMFY_BIN points at nothing: FAIL by name, even if comfy sits next to comfy-mcp.
	r.out.Reset()
	write("${USERPROFILE}/nope/" + exe("comfy"))
	failed := Check(context.Background(), r.opts)
	if failed != 1 || !strings.Contains(r.out.String(), "FAIL  comfy-cli - COMFY_BIN of .mcp.json is ") {
		t.Fatalf("failed=%d:\n%s", failed, r.out)
	}

	// ${VAR:-default} takes the default for an unset variable and the value for a set one.
	get := func(n string) string {
		if n == "SET" {
			return "value"
		}
		return ""
	}
	if got := expand("a${SET:-x}/${UNSET:-def}/${UNSET}/${SET}", get); got != "avalue/def//value" {
		t.Fatalf("expand: %q", got)
	}
}

// comfy-mcp on the PATH and a bare command name in .mcp.json (the form of
// Phase 3 of issue #4): comfy-cli is found next to it, or else on the PATH.
func TestBareCommandOnThePath(t *testing.T) {
	r := newRig(t)
	bin := t.TempDir()
	mcp := filepath.Join(bin, exe("comfy-mcp"))
	os.WriteFile(mcp, []byte("x"), 0o755)
	os.WriteFile(filepath.Join(r.opts.RepoRoot, ".mcp.json"),
		[]byte(`{"mcpServers":{"comfy-mcp":{"command":"comfy-mcp"},"horus":{"command":"horus"}}}`), 0o644)
	horus, _ := r.opts.LookPath("horus")
	r.opts.VenvPath = filepath.Join(t.TempDir(), "no-venv")
	r.opts.LookPath = func(n string) (string, error) {
		switch n {
		case "horus":
			return horus, nil
		case "comfy-mcp":
			return mcp, nil
		}
		return "", errors.New("not found")
	}
	if failed := Check(context.Background(), r.opts); failed != 1 || !strings.Contains(r.out.String(), "FAIL  comfy-cli - ") {
		t.Fatalf("no comfy anywhere, failed=%d:\n%s", failed, r.out)
	}
	r.out.Reset()
	comfy := filepath.Join(bin, exe("comfy"))
	os.WriteFile(comfy, []byte("x"), 0o755)
	if failed := Check(context.Background(), r.opts); failed != 0 || !strings.Contains(r.out.String(), "PASS  comfy-cli - "+comfy) {
		t.Fatalf("comfy next to comfy-mcp, failed=%d:\n%s", failed, r.out)
	}
	os.Remove(comfy)
	r.out.Reset()
	other := filepath.Join(t.TempDir(), exe("comfy"))
	os.WriteFile(other, []byte("x"), 0o755)
	look := r.opts.LookPath
	r.opts.LookPath = func(n string) (string, error) {
		if n == "comfy" {
			return other, nil
		}
		return look(n)
	}
	if failed := Check(context.Background(), r.opts); failed != 0 || !strings.Contains(r.out.String(), "PASS  comfy-cli - "+other) {
		t.Fatalf("comfy on the PATH, failed=%d:\n%s", failed, r.out)
	}
}

func TestOneMissingPieceGivesOneFailLine(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(r *rig)
		want   string
	}{
		{"no ComfyUI", func(r *rig) { os.Remove(filepath.Join(r.opts.ComfyUIDir, "ComfyUI", "main.py")) }, "FAIL  ComfyUI folder - "},
		{"no server", func(r *rig) { r.opts.ServerURL = "http://127.0.0.1:1" }, "FAIL  ComfyUI server - "},
		{"no horus on PATH", func(r *rig) {
			r.opts.LookPath = func(string) (string, error) { return "", errors.New("not found") }
		}, "FAIL  horus on PATH - "},
		{"entry of .mcp.json points at nothing", func(r *rig) {
			cfg, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{
				"comfy-mcp": map[string]any{"command": filepath.Join(r.opts.Home, "nope", exe("comfy-mcp"))},
				"horus":     map[string]any{"command": "horus"},
			}})
			os.WriteFile(filepath.Join(r.opts.RepoRoot, ".mcp.json"), cfg, 0o644)
		}, "FAIL  .mcp.json - "},
		{"no horus entry", func(r *rig) {
			os.WriteFile(filepath.Join(r.opts.RepoRoot, ".mcp.json"), []byte(`{"mcpServers":{}}`), 0o644)
		}, "FAIL  .mcp.json horus - "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			c.break_(r)
			failed := Check(context.Background(), r.opts)
			text := r.out.String()
			if !strings.Contains(text, c.want) {
				t.Fatalf("no line %q in:\n%s", c.want, text)
			}
			if failed < 1 || strings.Contains(text, "All checks passed.") || !strings.Contains(text, fmt.Sprintf("%d check(s) failed.", failed)) {
				t.Fatalf("failed=%d:\n%s", failed, text)
			}
		})
	}
}

func TestModelsLinesAreIndentedAndFailureCounts(t *testing.T) {
	r := newRig(t)
	m := &fakeModels{ok: false}
	r.opts.Workflow, r.opts.Models = "it-illustration", m
	if failed := Check(context.Background(), r.opts); failed != 1 {
		t.Fatalf("failed %d:\n%s", failed, r.out)
	}
	text := r.out.String()
	for _, want := range []string{"      OK        vae\n", "      MISSING   unet\n",
		"FAIL  models for 'it-illustration' - see lines above. Fix: horus workflow install it-illustration\n", "1 check(s) failed.\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("no %q in:\n%s", want, text)
		}
	}
	if len(m.asked) != 1 || m.asked[0] != "it-illustration" {
		t.Fatalf("asked: %v", m.asked)
	}

	// The quick check reads sizes only, so its line must not say "verified".
	r = newRig(t)
	r.opts.Workflow, r.opts.Models = "it-illustration", &fakeModels{ok: true}
	if failed := Check(context.Background(), r.opts); failed != 0 || !strings.Contains(r.out.String(), "PASS  models for 'it-illustration' - all files present, size checked") {
		t.Fatalf("failed %d:\n%s", failed, r.out)
	}
	if strings.Contains(r.out.String(), "verified") {
		t.Fatalf("size-only check says verified:\n%s", r.out)
	}

	r = newRig(t)
	r.opts.Workflow, r.opts.Models, r.opts.FullModels = "it-illustration", &fakeModels{ok: true}, true
	if failed := Check(context.Background(), r.opts); failed != 0 || !strings.Contains(r.out.String(), "PASS  models for 'it-illustration' - all files present and verified (SHA-256 checked)") {
		t.Fatalf("failed %d:\n%s", failed, r.out)
	}
}

func TestNoWorkflowFlagMeansNoModelCheck(t *testing.T) {
	r := newRig(t)
	m := &fakeModels{}
	r.opts.Models = m
	Check(context.Background(), r.opts)
	if len(m.asked) != 0 {
		t.Fatalf("asked: %v", m.asked)
	}
}

// TestExecModelsStartsTheWorkflowSubcommand runs the test binary as the fake
// "horus workflow check", and reads what it recorded.
func TestExecModelsStartsTheWorkflowSubcommand(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		exit   string
		full   bool
		wantOK bool
		args   string
	}{{"0", false, true, "args workflow check it-illustration -quick -json"}, {"1", true, false, "args workflow check it-illustration -json"}} {
		rec := filepath.Join(t.TempDir(), "record.txt")
		t.Setenv("HORUS_FAKE_WORKFLOW", rec)
		t.Setenv("HORUS_FAKE_EXIT", c.exit)
		m := ExecModels{Exe: self, Full: c.full, ComfyUIDir: "the-comfy-dir", ModelsDir: "the-models-dir"}
		got, ok, err := m.Check(context.Background(), "it-illustration")
		if err != nil || ok != c.wantOK {
			t.Fatalf("exit %s: ok=%v err=%v", c.exit, ok, err)
		}
		if len(got) != 2 || got[0] != "OK        vae vae/x.safetensors" || strings.Contains(strings.Join(got, "\n"), "HORUS_JSON") {
			t.Fatalf("lines: %q", got)
		}
		data, err := os.ReadFile(rec)
		if err != nil {
			t.Fatalf("the fake program did not run: %v", err)
		}
		for _, want := range []string{c.args + "\n", "comfyui the-comfy-dir\n", "models the-models-dir\n"} {
			if !strings.Contains(string(data), want) {
				t.Fatalf("record lacks %q:\n%s", want, data)
			}
		}
	}
}

func TestExecModelsReportsAProgramThatDoesNotStart(t *testing.T) {
	_, ok, err := ExecModels{Exe: filepath.Join(t.TempDir(), "absent")}.Check(context.Background(), "x")
	if err == nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestRunRejectsAnUnknownFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"-nope"}, &out, &errOut); code != 2 {
		t.Fatalf("exit %d", code)
	}
}

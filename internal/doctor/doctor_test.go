// Tests of the checks on fakes of the server, the install and the PATH. The
// venv and .mcp.json are files in a temporary folder.
package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/platform"
)

type fakeModels struct {
	ok    bool
	err   error
	asked []string
	quick []bool
}

func (f *fakeModels) Check(_ context.Context, workflow string, quick bool) ([]string, bool, error) {
	f.asked = append(f.asked, workflow)
	f.quick = append(f.quick, quick)
	return []string{"OK        vae", "MISSING   unet"}, f.ok, f.err
}

// fakeServer answers with version, or with err.
type fakeServer struct {
	version string
	err     error
}

func (f fakeServer) Version(context.Context) (string, error) { return f.version, f.err }

// fakeInstall is the install folder and its state.
type fakeInstall struct {
	dir string
	ok  bool
}

func (f fakeInstall) Installed() (string, bool) { return f.dir, f.ok }

// lookFunc is a fake PATH.
type lookFunc func(string) (string, error)

func (f lookFunc) LookPath(name string) (string, error) { return f(name) }

type rig struct {
	server  fakeServer
	install fakeInstall
	look    lookFunc
	models  Models
	opts    Options
}

func (r *rig) check() []Result {
	return New(r.server, r.install, r.look, r.models).Check(context.Background(), r.opts)
}

// text writes the results as horus doctor does, for the messages of a test.
func text(rs []Result) string {
	var b strings.Builder
	for _, r := range rs {
		for _, l := range r.Lines {
			b.WriteString("      " + l + "\n")
		}
		word := "FAIL"
		if r.OK {
			word = "PASS"
		}
		fmt.Fprintf(&b, "%s  %s - %s\n", word, r.Name, r.Detail)
	}
	return b.String()
}

// newRig has a ComfyUI install, a server, a venv with comfy-mcp, a .mcp.json
// and a PATH that holds horus. Every check passes.
func newRig(t *testing.T) *rig {
	t.Helper()
	tmp := t.TempDir()
	venv := filepath.Join(tmp, "venv")
	mcp := platform.VenvBin(venv, "comfy-mcp")
	os.MkdirAll(filepath.Dir(mcp), 0o755)
	os.WriteFile(mcp, []byte("x"), 0o755)
	os.WriteFile(platform.VenvBin(venv, "comfy"), []byte("x"), 0o755)

	bin := filepath.Join(tmp, "bin")
	os.MkdirAll(bin, 0o755)
	horus := filepath.Join(bin, platform.ExeName("horus"))
	os.WriteFile(horus, []byte("x"), 0o755)

	repo := filepath.Join(tmp, "repo")
	os.MkdirAll(repo, 0o755)
	cfg, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{
		"comfy-mcp": map[string]any{"command": mcp},
		"horus":     map[string]any{"command": "horus", "args": []string{"mcp"}},
	}})
	os.WriteFile(filepath.Join(repo, ".mcp.json"), cfg, 0o644)

	return &rig{
		server:  fakeServer{version: "0.9.9"},
		install: fakeInstall{dir: filepath.Join(tmp, "comfy"), ok: true},
		look: func(name string) (string, error) {
			if name == "horus" {
				return horus, nil
			}
			return "", errors.New("not found")
		},
		opts: Options{ServerURL: "http://127.0.0.1:8188", VenvPath: venv, RepoRoot: repo, Home: tmp},
	}
}

func venvBin(venv string) string { return filepath.Dir(platform.VenvBin(venv, "x")) }

func TestAllChecksPass(t *testing.T) {
	r := newRig(t)
	rs := r.check()
	if Failed(rs) != 0 || len(rs) != 7 {
		t.Fatalf("%d results, %d failed:\n%s", len(rs), Failed(rs), text(rs))
	}
	want := []string{"ComfyUI folder", "ComfyUI server", "comfy-mcp environment", "comfy-cli", ".mcp.json", "horus on PATH", ".mcp.json horus"}
	for i, w := range want {
		if rs[i].Name != w {
			t.Errorf("check %d is %q, want %q", i+1, rs[i].Name, w)
		}
	}
	if rs[0].Detail != r.install.dir {
		t.Errorf("folder line: %q", rs[0].Detail)
	}
	if rs[1].Detail != "answers at http://127.0.0.1:8188 (ComfyUI 0.9.9)" {
		t.Errorf("server line: %q", rs[1].Detail)
	}
	if rs[3].Detail != platform.VenvBin(r.opts.VenvPath, "comfy") {
		t.Errorf("comfy-cli line: %q", rs[3].Detail)
	}
}

// A venv that has comfy-mcp but no comfy program: the comfy MCP server fails
// when it calls comfy-cli, so the check must fail by the name "comfy-cli".
func TestMissingComfyCliFails(t *testing.T) {
	r := newRig(t)
	os.Remove(platform.VenvBin(r.opts.VenvPath, "comfy"))
	rs := r.check()
	if Failed(rs) != 1 || !strings.Contains(text(rs), "FAIL  comfy-cli - ") {
		t.Fatalf("failed=%d:\n%s", Failed(rs), text(rs))
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
			"comfy-mcp": map[string]any{"command": "${USERPROFILE}/" + rel + "/" + platform.ExeName("comfy-mcp"),
				"env": map[string]any{"COMFY_BIN": comfyBin, "COMFY_LOCAL_URL": "${COMFY_LOCAL_URL:-http://127.0.0.1:8188}"}},
			"horus": map[string]any{"command": "horus"},
		}})
		os.WriteFile(filepath.Join(r.opts.RepoRoot, ".mcp.json"), cfg, 0o644)
	}
	write("${USERPROFILE}/" + rel + "/" + platform.ExeName("comfy"))
	rs := r.check()
	if Failed(rs) != 0 {
		t.Fatalf("failed %d:\n%s", Failed(rs), text(rs))
	}
	if !strings.Contains(text(rs), "COMFY_BIN at ") {
		t.Fatalf("no COMFY_BIN in:\n%s", text(rs))
	}

	// COMFY_BIN points at nothing: FAIL by name, even if comfy sits next to comfy-mcp.
	write("${USERPROFILE}/nope/" + platform.ExeName("comfy"))
	rs = r.check()
	if Failed(rs) != 1 || !strings.Contains(text(rs), "FAIL  comfy-cli - COMFY_BIN of .mcp.json is ") {
		t.Fatalf("failed=%d:\n%s", Failed(rs), text(rs))
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
	mcp := filepath.Join(bin, platform.ExeName("comfy-mcp"))
	os.WriteFile(mcp, []byte("x"), 0o755)
	os.WriteFile(filepath.Join(r.opts.RepoRoot, ".mcp.json"),
		[]byte(`{"mcpServers":{"comfy-mcp":{"command":"comfy-mcp"},"horus":{"command":"horus"}}}`), 0o644)
	horus, _ := r.look("horus")
	r.opts.VenvPath = filepath.Join(t.TempDir(), "no-venv")
	r.look = func(n string) (string, error) {
		switch n {
		case "horus":
			return horus, nil
		case "comfy-mcp":
			return mcp, nil
		}
		return "", errors.New("not found")
	}
	if rs := r.check(); Failed(rs) != 1 || !strings.Contains(text(rs), "FAIL  comfy-cli - ") {
		t.Fatalf("no comfy anywhere, failed=%d:\n%s", Failed(rs), text(rs))
	}
	comfy := filepath.Join(bin, platform.ExeName("comfy"))
	os.WriteFile(comfy, []byte("x"), 0o755)
	if rs := r.check(); Failed(rs) != 0 || !strings.Contains(text(rs), "PASS  comfy-cli - "+comfy) {
		t.Fatalf("comfy next to comfy-mcp, failed=%d:\n%s", Failed(rs), text(rs))
	}
	os.Remove(comfy)
	other := filepath.Join(t.TempDir(), platform.ExeName("comfy"))
	os.WriteFile(other, []byte("x"), 0o755)
	look := r.look
	r.look = func(n string) (string, error) {
		if n == "comfy" {
			return other, nil
		}
		return look(n)
	}
	if rs := r.check(); Failed(rs) != 0 || !strings.Contains(text(rs), "PASS  comfy-cli - "+other) {
		t.Fatalf("comfy on the PATH, failed=%d:\n%s", Failed(rs), text(rs))
	}
}

func TestOneMissingPieceGivesOneFailLine(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(r *rig)
		want   string
	}{
		{"no ComfyUI", func(r *rig) { r.install.ok = false }, "FAIL  ComfyUI folder - " + filepath.Join("ComfyUI", "main.py") + " not found in "},
		{"no server", func(r *rig) { r.server = fakeServer{err: errors.New("refused")} },
			"FAIL  ComfyUI server - no answer at http://127.0.0.1:8188. Fix: horus start-comfyui"},
		{"no horus on PATH", func(r *rig) {
			r.look = func(string) (string, error) { return "", errors.New("not found") }
		}, "FAIL  horus on PATH - "},
		{"entry of .mcp.json points at nothing", func(r *rig) {
			cfg, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{
				"comfy-mcp": map[string]any{"command": filepath.Join(r.opts.Home, "nope", platform.ExeName("comfy-mcp"))},
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
			rs := r.check()
			if !strings.Contains(text(rs), c.want) {
				t.Fatalf("no line %q in:\n%s", c.want, text(rs))
			}
			if Failed(rs) < 1 {
				t.Fatalf("failed=%d:\n%s", Failed(rs), text(rs))
			}
		})
	}
}

func TestModelsLinesAndFailureCount(t *testing.T) {
	r := newRig(t)
	m := &fakeModels{ok: false}
	r.opts.Workflow, r.models = "it-illustration", m
	rs := r.check()
	if Failed(rs) != 1 || len(rs) != 8 {
		t.Fatalf("failed %d:\n%s", Failed(rs), text(rs))
	}
	last := rs[7]
	if last.Name != "models for 'it-illustration'" || last.Detail != "see lines above. Fix: horus workflow install it-illustration" ||
		strings.Join(last.Lines, "|") != "OK        vae|MISSING   unet" {
		t.Fatalf("model result: %+v", last)
	}
	if len(m.asked) != 1 || m.asked[0] != "it-illustration" || m.quick[0] {
		t.Fatalf("asked: %v, quick %v", m.asked, m.quick)
	}

	// The quick check reads sizes only, so its line must not say "verified".
	r = newRig(t)
	m = &fakeModels{ok: true}
	r.opts.Workflow, r.models, r.opts.Quick = "it-illustration", m, true
	rs = r.check()
	if Failed(rs) != 0 || !strings.Contains(text(rs), "PASS  models for 'it-illustration' - all files present, size checked") || !m.quick[0] {
		t.Fatalf("failed %d, quick %v:\n%s", Failed(rs), m.quick, text(rs))
	}
	if strings.Contains(text(rs), "verified") {
		t.Fatalf("size-only check says verified:\n%s", text(rs))
	}

	r = newRig(t)
	r.opts.Workflow, r.models = "it-illustration", &fakeModels{ok: true}
	if rs := r.check(); Failed(rs) != 0 || !strings.Contains(text(rs), "PASS  models for 'it-illustration' - all files present and verified (SHA-256 checked)") {
		t.Fatalf("failed %d:\n%s", Failed(rs), text(rs))
	}

	// A check that cannot run fails by name.
	r = newRig(t)
	r.opts.Workflow, r.models = "it-illustration", &fakeModels{err: errors.New("boom")}
	if rs := r.check(); Failed(rs) != 1 || !strings.Contains(text(rs), "FAIL  models for 'it-illustration' - cannot run the model check: boom") {
		t.Fatalf("failed %d:\n%s", Failed(rs), text(rs))
	}
}

func TestNoWorkflowFlagMeansNoModelCheck(t *testing.T) {
	r := newRig(t)
	m := &fakeModels{}
	r.models = m
	if rs := r.check(); len(rs) != 7 || len(m.asked) != 0 {
		t.Fatalf("%d results, asked: %v", len(rs), m.asked)
	}
}

// TestCheckChangesNoFile runs every check, with a model check, and compares
// the files of the rig before and after: horus doctor is read-only.
func TestCheckChangesNoFile(t *testing.T) {
	r := newRig(t)
	r.opts.Workflow, r.models = "it-illustration", &fakeModels{ok: true}
	snap := func() string {
		var b strings.Builder
		filepath.WalkDir(r.opts.Home, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// The name only: Windows can update the time of a
				// folder late, after the files of the rig were written.
				fmt.Fprintf(&b, "%s/\n", p)
				return nil
			}
			fi, _ := d.Info()
			fmt.Fprintf(&b, "%s %d %v\n", p, fi.Size(), fi.ModTime().UnixNano())
			return nil
		})
		return b.String()
	}
	before := snap()
	r.check()
	if after := snap(); after != before {
		t.Fatalf("the files changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

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
	opts    Options
}

func (r *rig) check() []Result {
	return New(r.server, r.install, r.look).Check(context.Background(), r.opts)
}

// text writes the results as horus doctor does, for the messages of a test.
func text(rs []Result) string {
	var b strings.Builder
	for _, r := range rs {
		word := map[Level]string{Pass: "PASS", Warn: "WARN"}[r.Level]
		if word == "" {
			word = "FAIL"
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

// TestTheMcpJSONOfTheRepoPasses: the .mcp.json of this repo starts both
// servers through horus (`horus comfy-mcp`, `horus mcp`). With the venv and
// horus on the PATH every check passes, and comfy-cli is the comfy of the
// venv. Without horus on the PATH, the comfy-mcp entry fails with the fix
// for horus. An entry `horus mcp` for comfy-mcp fails.
func TestTheMcpJSONOfTheRepoPasses(t *testing.T) {
	r := newRig(t)
	repoCfg, err := os.ReadFile(filepath.Join("..", "..", ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(r.opts.RepoRoot, ".mcp.json")
	os.WriteFile(cfgPath, repoCfg, 0o644)
	horus, _ := r.look("horus")
	rs := r.check()
	if Failed(rs) != 0 || !strings.Contains(text(rs), "PASS  .mcp.json - command points at "+horus+" comfy-mcp\n") ||
		!strings.Contains(text(rs), "PASS  comfy-cli - "+platform.VenvBin(r.opts.VenvPath, "comfy")+"\n") {
		t.Fatalf("the .mcp.json of the repo, failed=%d:\n%s", Failed(rs), text(rs))
	}

	look := r.look
	r.look = func(n string) (string, error) {
		if n == "horus" {
			return "", errors.New("not found")
		}
		return look(n)
	}
	want := "FAIL  .mcp.json - command is 'horus' and does not exist. Fix: put horus on the PATH, or edit .mcp.json.\n"
	if rs := r.check(); !strings.Contains(text(rs), want) {
		t.Fatalf("no horus on the PATH: no line %q in:\n%s", want, text(rs))
	}
	r.look = look

	os.WriteFile(cfgPath, []byte(`{"mcpServers":{"comfy-mcp":{"command":"horus","args":["mcp"]},"horus":{"command":"horus","args":["mcp"]}}}`), 0o644)
	want = "FAIL  .mcp.json - the entry runs `horus mcp`, which is not the comfy MCP server."
	if rs := r.check(); Failed(rs) != 1 || !strings.Contains(text(rs), want) {
		t.Fatalf("comfy-mcp entry `horus mcp`: failed=%d, want 1 and %q:\n%s", Failed(rs), want, text(rs))
	}
}

// TestComfyOfThePathWhenTheVenvHasNone: horus comfy-mcp sets no COMFY_BIN
// when the venv has no comfy, so comfy-mcp runs the comfy of the PATH. The
// check finds the same program.
func TestComfyOfThePathWhenTheVenvHasNone(t *testing.T) {
	r := newRig(t)
	os.Remove(platform.VenvBin(r.opts.VenvPath, "comfy"))
	other := filepath.Join(t.TempDir(), platform.ExeName("comfy"))
	os.WriteFile(other, []byte("x"), 0o755)
	look := r.look
	r.look = func(n string) (string, error) {
		if n == "comfy" {
			return other, nil
		}
		return look(n)
	}
	if rs := r.check(); Failed(rs) != 0 || !strings.Contains(text(rs), "PASS  comfy-cli - "+other+"\n") {
		t.Fatalf("failed=%d:\n%s", Failed(rs), text(rs))
	}
}

// The repo .mcp.json of Phase 2 starts comfy-mcp from a path with
// ${USERPROFILE} and has COMFY_BIN and ${VAR:-default} in its env. Both must
// expand. Other users still have this file: it passes when the files exist.
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

// TestCheckChangesNoFile runs every check and compares
// the files of the rig before and after: horus doctor is read-only.
func TestCheckChangesNoFile(t *testing.T) {
	r := newRig(t)
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

// TestTCCCheckWarnsForEachPathInATCCFolder: the doctor asks the TCC function
// for each path of the setup (the repo root, the install folder, the models
// folder, the text encoder folder, the output folder) as an absolute path.
// Each path inside a TCC folder gives one result of level Warn, after the 7
// checks. The line names the absolute path, the folder and the fix. A Warn
// is not a failed check. With no path inside, the doctor gives the 7 results
// and no Warn.
func TestTCCCheckWarnsForEachPathInATCCFolder(t *testing.T) {
	r := newRig(t)
	desktop := filepath.Join(r.opts.Home, "Desktop")
	repo := filepath.Join(desktop, "horus")
	os.MkdirAll(repo, 0o755)
	os.Rename(filepath.Join(r.opts.RepoRoot, ".mcp.json"), filepath.Join(repo, ".mcp.json"))
	r.opts.RepoRoot = repo
	r.opts.ModelsDir = filepath.Join(r.opts.Home, "Documents", "models")
	r.opts.TextEncoderDir = filepath.Join(r.opts.Home, "te") // outside
	r.opts.OutputDir = filepath.Join(repo, ".scratch", "outputs")
	r.install.dir = filepath.Join(r.opts.Home, "Downloads", "comfy")
	var asked []string
	r.opts.InTCCFolder = func(p string) (string, bool) {
		asked = append(asked, p)
		for _, name := range []string{"Desktop", "Documents", "Downloads"} {
			f := filepath.Join(r.opts.Home, name)
			if strings.HasPrefix(p, f+string(filepath.Separator)) {
				return f, true
			}
		}
		return "", false
	}
	rs := r.check()
	if len(asked) != 5 {
		t.Errorf("the doctor asked for %d paths, want 5: %q", len(asked), asked)
	}
	for _, p := range asked {
		if !filepath.IsAbs(p) {
			t.Errorf("the doctor asked for %q, want an absolute path", p)
		}
	}
	want := []struct{ what, path, folder string }{
		{"the repo root", repo, desktop},
		{"the install folder", r.install.dir, filepath.Join(r.opts.Home, "Downloads")},
		{"the models folder", r.opts.ModelsDir, filepath.Join(r.opts.Home, "Documents")},
		{"the output folder", r.opts.OutputDir, desktop},
	}
	if len(rs) != 7+len(want) || Failed(rs) != 0 {
		t.Fatalf("%d results, %d failed, want %d and 0:\n%s", len(rs), Failed(rs), 7+len(want), text(rs))
	}
	for i, w := range want {
		got := rs[7+i]
		detail := fmt.Sprintf("%s %s is inside %s, a folder that macOS protects. ComfyUI and comfy-mcp can fail to read it. "+
			"Fix: move it out of %s, or grant access in System Settings > Privacy & Security.", w.what, w.path, w.folder, w.folder)
		if got.Level != Warn || got.Name != "macOS privacy folder" || got.Detail != detail {
			t.Errorf("result %d = %+v, want Warn %q", 8+i, got, detail)
		}
	}

	// A relative path is checked as its absolute path.
	wd, _ := os.Getwd()
	r.opts.OutputDir = "out"
	asked = nil
	r.check()
	if len(asked) != 5 || asked[4] != filepath.Join(wd, "out") {
		t.Errorf("relative output folder: asked %q, want the last one %s", asked, filepath.Join(wd, "out"))
	}

	// No path inside a TCC folder: no Warn.
	r.opts.InTCCFolder = func(string) (string, bool) { return "", false }
	if rs := r.check(); len(rs) != 7 || Failed(rs) != 0 {
		t.Errorf("no TCC path: %d results, %d failed:\n%s", len(rs), Failed(rs), text(rs))
	}
}

// TestAWarnIsNotAFailedCheck: Failed counts only the results of level Fail,
// and the zero value of a level is Fail.
func TestAWarnIsNotAFailedCheck(t *testing.T) {
	rs := []Result{{Name: "a", Level: Pass}, {Name: "b", Level: Warn}, {Name: "c", Level: Fail}, {Name: "d"}}
	if n := Failed(rs); n != 2 {
		t.Errorf("Failed = %d, want 2 (c and d)", n)
	}
	if Failed(rs[:2]) != 0 {
		t.Error("a Warn counts as failed")
	}
}

// Tests of install_workflow on a machine without ComfyUI: the setup consent
// (accept_setup, SETUP_CONFIRMATION_REQUIRED), the order of the two consents,
// the setup steps of the plan and their progress messages. The setup service
// is the real provision.Service with fakes of the programs of the machine, of
// the ComfyUI install and of the downloader. No test runs a real program.
package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/provision"
)

// wingetArgs are the arguments of the 7-Zip step in the spec.
var wingetArgs = []string{"install", "--id", "7zip.7zip", "-e", "--accept-source-agreements", "--accept-package-agreements"}

// setupLog is the log of what the fakes did, in order.
type setupLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *setupLog) add(s string) { l.mu.Lock(); l.lines = append(l.lines, s); l.mu.Unlock() }
func (l *setupLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.lines)
}

// emptyMachine is a machine without ComfyUI. The package manager winget is
// on the PATH (when manager is true), and 7-Zip is not installed: winget
// puts 7z (7z.exe on Windows) into pkgDir.
type emptyMachine struct {
	log     *setupLog
	comfy   string // the install folder of ComfyUI
	pkgDir  string // the folder of 7-Zip
	models  string // the models folder
	root    string // the folder that holds every file of the test
	bodies  map[string][]byte
	winget  string
	manager bool
}

// fakeMachinePrograms is a fake of the programs of the machine.
type fakeMachinePrograms struct{ m *emptyMachine }

func (p fakeMachinePrograms) LookPath(name string) (string, error) {
	if name == "winget" && p.m.manager {
		return p.m.winget, nil
	}
	return "", errors.New("not found")
}

func (p fakeMachinePrograms) Run(_ context.Context, c platform.Command) (int, error) {
	p.m.log.add("run " + filepath.Base(c.Path) + " " + strings.Join(c.Args, " "))
	if filepath.Base(c.Path) == filepath.Base(p.m.winget) && slices.Equal(c.Args, wingetArgs) {
		os.MkdirAll(p.m.pkgDir, 0o755)
		os.WriteFile(filepath.Join(p.m.pkgDir, platform.ExeName("7z")), []byte("fake"), 0o755)
	}
	return 0, nil
}

// fakeInstall is a fake of the ComfyUI install: Install writes
// <comfy>/ComfyUI/main.py.
type fakeInstall struct{ m *emptyMachine }

func (b fakeInstall) Installed() (string, bool) {
	return b.m.comfy, platform.IsFile(filepath.Join(b.m.comfy, "ComfyUI", "main.py"))
}

func (b fakeInstall) Install(context.Context, io.Writer) error {
	b.m.log.add("install ComfyUI")
	dir := filepath.Join(b.m.comfy, "ComfyUI")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "main.py"), []byte("# fake\n"), 0o644)
}

// fakeDownloads writes the stand-in body of each model file of the sandbox.
type fakeDownloads struct{ m *emptyMachine }

func (d fakeDownloads) Download(_ context.Context, f provision.Fetch, _ io.Writer) (bool, error) {
	d.m.log.add("download " + filepath.Base(f.Dest))
	body, ok := d.m.bodies[filepath.Base(f.Dest)]
	if !ok {
		return false, errors.New("no body for " + f.Dest)
	}
	if err := os.MkdirAll(filepath.Dir(f.Dest), 0o755); err != nil {
		return false, err
	}
	return false, os.WriteFile(f.Dest, body, 0o644)
}

// newEmptyMachine makes the sandbox of setupSandbox, with no ComfyUI and no
// model file. The service sees the system of Windows: 7-Zip with winget.
func newEmptyMachine(t *testing.T, manager bool) (*emptyMachine, *provision.Service) {
	t.Helper()
	models := setupSandbox(t)
	root := filepath.Dir(models)
	m := &emptyMachine{log: &setupLog{}, comfy: filepath.Join(root, "ComfyUI_windows_portable"), pkgDir: filepath.Join(root, "Program Files", "7-Zip"),
		models: models, root: root, bodies: map[string][]byte{}, winget: filepath.Join(root, "bin", "winget.exe"), manager: manager}
	data, err := os.ReadFile(filepath.Join(root, "workflows", "it-illustration", "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Files []struct {
			RepoPath string `json:"repo_path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatal(err)
	}
	for _, f := range list.Files {
		parts := strings.Split(f.RepoPath, "/")
		m.bodies[parts[len(parts)-1]] = []byte("stand-in for " + f.RepoPath)
	}
	pkg := &platform.Package{Name: "7-Zip", Program: "7z", Dirs: []string{m.pkgDir}, Manager: "winget", Args: wingetArgs, URL: "https://www.7-zip.org/"}
	svc := provision.New(diskFiles{models}, fakeDownloads{m}, downComfy{}, fakeInstall{m}, "").
		WithSystem(provision.System{Programs: fakeMachinePrograms{m}, Package: pkg})
	return m, svc
}

// tree is the path, size and SHA-256 of each file under root.
func tree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, _ := os.ReadFile(p)
		h := sha256.Sum256(b)
		out = append(out, p+" "+hex.EncodeToString(h[:]))
		return nil
	})
	return out
}

func errorOf(t *testing.T, env map[string]any) (code string, details map[string]any, e map[string]any) {
	t.Helper()
	e, _ = env["error"].(map[string]any)
	code, _ = e["code"].(string)
	details, _ = e["details"].(map[string]any)
	return code, details, e
}

// TestInstallWorkflowWithoutAcceptSetupAnswersSetupConfirmationRequired: on
// a machine without ComfyUI, install_workflow without accept_setup answers
// SETUP_CONFIRMATION_REQUIRED with each step of the plan, also when the call
// sends accept_license=true. It runs no program, installs nothing and writes
// no file.
func TestInstallWorkflowWithoutAcceptSetupAnswersSetupConfirmationRequired(t *testing.T) {
	m, svc := newEmptyMachine(t, true)
	before := tree(t, m.root)
	cs := connect(t, New(&fakeImages{}, svc), nil)
	for _, args := range []map[string]any{
		{"workflow": "it-illustration"},
		{"workflow": "it-illustration", "accept_license": true},
	} {
		res := callTool(t, cs, "install_workflow", args)
		code, d, e := errorOf(t, envelopeOf(t, res))
		if !res.IsError || code != "SETUP_CONFIRMATION_REQUIRED" || e["action_by"] != "human" || e["retryable"] != false {
			t.Fatalf("%v: %v", args, e)
		}
		if ns, _ := e["next_step"].(string); !strings.Contains(ns, "accept_setup=true") {
			t.Errorf("next_step %q does not name accept_setup=true", ns)
		}
		steps, _ := d["steps"].([]any)
		var kinds []string
		for _, x := range steps {
			kinds = append(kinds, x.(map[string]any)["kind"].(string))
		}
		if strings.Join(kinds, ",") != "package,backend,file,file,file" {
			t.Fatalf("details.steps kinds %v, want package,backend and the 3 files: %v", kinds, d)
		}
		pkg, backend := steps[0].(map[string]any), steps[1].(map[string]any)
		var command []string
		for _, x := range pkg["command"].([]any) {
			command = append(command, x.(string))
		}
		if pkg["name"] != "7-Zip" || !slices.Equal(command, append([]string{m.winget}, wingetArgs...)) || pkg["dir"] != m.pkgDir {
			t.Errorf("package step %v, want 7-Zip with %v in %s", pkg, wingetArgs, m.pkgDir)
		}
		if backend["name"] != "ComfyUI" || backend["dir"] != m.comfy || backend["command"] != nil {
			t.Errorf("backend step %v, want ComfyUI in %s", backend, m.comfy)
		}
		if msg, _ := e["message"].(string); !strings.Contains(msg, "7-Zip") || !strings.Contains(msg, "ComfyUI") {
			t.Errorf("message %q does not name the steps", msg)
		}
	}
	if l := m.log.all(); len(l) != 0 {
		t.Errorf("the tool ran something before the consent: %v", l)
	}
	if after := tree(t, m.root); !slices.Equal(before, after) {
		t.Errorf("the files changed before the consent:\nbefore %v\nafter  %v", before, after)
	}
}

// TestInstallWorkflowAsksTheSetupConsentBeforeTheLicense: accept_setup=true
// without accept_license answers LICENSE_CONFIRMATION_REQUIRED when the plan
// downloads model files, and still changes nothing.
func TestInstallWorkflowAsksTheSetupConsentBeforeTheLicense(t *testing.T) {
	m, svc := newEmptyMachine(t, true)
	before := tree(t, m.root)
	cs := connect(t, New(&fakeImages{}, svc), nil)
	code, d, _ := errorOf(t, envelopeOf(t, callTool(t, cs, "install_workflow", map[string]any{"workflow": "it-illustration", "accept_setup": true})))
	if code != "LICENSE_CONFIRMATION_REQUIRED" || d["download_bytes"] == nil {
		t.Fatalf("got %s %v, want LICENSE_CONFIRMATION_REQUIRED", code, d)
	}
	if l := m.log.all(); len(l) != 0 {
		t.Errorf("the tool ran something before the license consent: %v", l)
	}
	if after := tree(t, m.root); !slices.Equal(before, after) {
		t.Errorf("the files changed before the license consent")
	}
}

// TestInstallWorkflowWithAcceptSetupRunsTheSetupSteps: with both consents
// the tool runs the plan in order (the package manager, the ComfyUI install,
// the downloads), sends a progress message that names each setup step, and
// answers installed. A second call finds the workflow installed.
func TestInstallWorkflowWithAcceptSetupRunsTheSetupSteps(t *testing.T) {
	m, svc := newEmptyMachine(t, true)
	var mu sync.Mutex
	var messages []string
	cs := connect(t, New(&fakeImages{}, svc), &mcp.ClientOptions{ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
		mu.Lock()
		messages = append(messages, r.Params.Message)
		mu.Unlock()
	}})
	params := &mcp.CallToolParams{Name: "install_workflow", Arguments: map[string]any{"workflow": "it-illustration", "accept_setup": true, "accept_license": true}}
	params.SetProgressToken("tok-setup")
	res, err := cs.CallTool(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	env := envelopeOf(t, res)
	r, _ := env["result"].(map[string]any)
	if env["ok"] != true || r["install_state"] != "installed" || r["already_installed"] != false || stateList(r["files"]) != "downloaded,downloaded,downloaded" {
		t.Fatalf("got %v", env)
	}
	want := []string{"run winget.exe " + strings.Join(wingetArgs, " "), "install ComfyUI"}
	if l := m.log.all(); len(l) != 5 || !slices.Equal(l[:2], want) || !strings.HasPrefix(l[2], "download ") {
		t.Fatalf("the steps ran in this order: %v, want %v, then 3 downloads", l, want)
	}
	if !platform.IsFile(filepath.Join(m.comfy, "ComfyUI", "main.py")) || !platform.IsFile(filepath.Join(m.pkgDir, platform.ExeName("7z"))) {
		t.Errorf("the install folder or 7-Zip is missing")
	}
	mu.Lock()
	got := strings.Join(messages, "\n")
	mu.Unlock()
	for _, w := range []string{"setup step 1 of 2: install the system package 7-Zip: " + m.winget, "setup step 2 of 2: install ComfyUI in " + m.comfy} {
		if !strings.Contains(got, w) {
			t.Errorf("no progress message %q in:\n%s", w, got)
		}
	}
	env = envelopeOf(t, callTool(t, cs, "install_workflow", map[string]any{"workflow": "it-illustration"}))
	if r, _ := env["result"].(map[string]any); env["ok"] != true || r["already_installed"] != true {
		t.Errorf("second call: %v", env)
	}
}

// TestInstallWorkflowWithoutAPackageManagerAnswersBackendNotInstalled: a
// step for a person gives BACKEND_NOT_INSTALLED whose next_step names the
// package, also with both consents, and changes nothing.
func TestInstallWorkflowWithoutAPackageManagerAnswersBackendNotInstalled(t *testing.T) {
	m, svc := newEmptyMachine(t, false)
	before := tree(t, m.root)
	cs := connect(t, New(&fakeImages{}, svc), nil)
	code, d, e := errorOf(t, envelopeOf(t, callTool(t, cs, "install_workflow",
		map[string]any{"workflow": "it-illustration", "accept_setup": true, "accept_license": true})))
	ns, _ := e["next_step"].(string)
	if code != "BACKEND_NOT_INSTALLED" || e["action_by"] != "human" || !strings.Contains(ns, "7-Zip") || !strings.Contains(ns, "https://www.7-zip.org/") || d["manager"] != "winget" {
		t.Fatalf("got %v", e)
	}
	if l := m.log.all(); len(l) != 0 {
		t.Errorf("the tool ran something: %v", l)
	}
	if after := tree(t, m.root); !slices.Equal(before, after) {
		t.Errorf("the files changed")
	}
}

// TestInstallWorkflowNamesTheSetupStepThatFailed: a package step whose
// program is not found after the package manager ends gives
// BACKEND_NOT_INSTALLED with the step, and the next steps do not run.
func TestInstallWorkflowNamesTheSetupStepThatFailed(t *testing.T) {
	m, svc := newEmptyMachine(t, true)
	m.pkgDir = filepath.Join(m.root, "elsewhere") // winget puts 7z where the plan does not look
	cs := connect(t, New(&fakeImages{}, svc), nil)
	code, d, e := errorOf(t, envelopeOf(t, callTool(t, cs, "install_workflow",
		map[string]any{"workflow": "it-illustration", "accept_setup": true, "accept_license": true})))
	step, _ := d["step"].(map[string]any)
	if code != "BACKEND_NOT_INSTALLED" || step["kind"] != "package" || !strings.Contains(e["message"].(string), "7z was not found") {
		t.Fatalf("got %v", e)
	}
	if l := m.log.all(); len(l) != 1 {
		t.Errorf("steps after the failed step ran: %v", l)
	}
}

// stateList is the state of each file of a result, joined with commas.
func stateList(files any) string {
	var out []string
	list, _ := files.([]any)
	for _, x := range list {
		out = append(out, x.(map[string]any)["state"].(string))
	}
	return strings.Join(out, ",")
}

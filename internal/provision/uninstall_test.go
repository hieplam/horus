// Tests of horus uninstall on a sandbox. Each test builds its own sandbox
// with t.TempDir: the home folder, the user folder, the repo root and the
// PATH are in it, and the PATH holds only the fake claude and the fake codex.
// Before a test runs the plan, it checks that every path of the plan is in
// the sandbox (planInside). No test reaches the real home folder, the real
// registry or the real program.
package provision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

// diskFiles is the model files on the disk: every model file of folder F is
// in <root>/F.
type diskFiles struct{ root string }

func (d diskFiles) Dirs(folder string) []string { return []string{filepath.Join(d.root, folder)} }
func (d diskFiles) Own(folder string) string    { return filepath.Join(d.root, folder) }
func (d diskFiles) Mapped(string) []string      { return nil }
func (d diskFiles) Map([][2]string) error       { return nil }
func (d diskFiles) Delete(path string) error    { return os.Remove(path) }
func (d diskFiles) Size(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return fi.Size()
}
func (d diskFiles) SHA256(path string) (string, error) {
	b, err := os.ReadFile(path)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), err
}

// fakeComfyInstall is the ComfyUI install: a folder, a server that answers
// or not, and a record of the calls of Stop.
type fakeComfyInstall struct {
	folder           string
	answers, started bool
	stops            int
	folderAtStop     bool // the folder was still there when Stop ran
}

func (f *fakeComfyInstall) Folder() string                       { return f.folder }
func (f *fakeComfyInstall) Running(context.Context) (bool, bool) { return f.answers, f.started }
func (f *fakeComfyInstall) Stop(context.Context, io.Writer) error {
	f.stops++
	f.folderAtStop = exists(f.folder)
	return nil
}

// fakeOS is the OS: the PATH entry is a marked line in the profile file of
// the sandbox, and the program is a file of the sandbox.
type fakeOS struct{ profile, program string }

func (o *fakeOS) Program() (string, error) { return o.program, nil }
func (o *fakeOS) PathEntry(string) ([]string, error) {
	b, err := os.ReadFile(o.profile)
	if err != nil {
		return nil, nil
	}
	if _, changed := platform.RemoveMarkedLines(string(b)); changed {
		return []string{o.profile}, nil
	}
	return nil, nil
}
func (o *fakeOS) RemovePathEntry(string) error {
	b, err := os.ReadFile(o.profile)
	if err != nil {
		return err
	}
	out, _ := platform.RemoveMarkedLines(string(b))
	return os.WriteFile(o.profile, []byte(out), 0o644)
}
func (o *fakeOS) RemoveProgram() error { return os.Remove(o.program) }

// unSandbox is one machine after a horus install, in a temporary folder.
type unSandbox struct {
	root, home, user, comfy, venv, workflows, models, program, profile, ask string
	vae, te                                                                 string
	comfyFake                                                               *fakeComfyInstall
	svc                                                                     *Service
	read                                                                    func() string // the record of the fake clients
}

// sandboxOptions picks what the sandbox has.
type sandboxOptions struct {
	noRecord bool // the ComfyUI folder and the venv have no install record
	known    bool // claude and codex know the server horus
}

func mkfile(t *testing.T, p string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// newUnSandbox makes the sandbox and points every setting of horus into it:
// the home folder (USERPROFILE, HOME), HORUS_HOME, HORUS_ROOT, LOCALAPPDATA,
// APPDATA, CODEX_HOME and the PATH. HORUS_MODELS_DIR, HORUS_TEXT_ENCODER_DIR,
// HORUS_COMFYUI_DIR and HORUS_SPEECH_DIR are empty, so the speech folder is
// <user folder>/speech.
func newUnSandbox(t *testing.T, o sandboxOptions) *unSandbox {
	t.Helper()
	root, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &unSandbox{root: root, home: filepath.Join(root, "home"), ask: filepath.Join(root, "ask")}
	s.user = filepath.Join(s.home, "horus")
	bin := filepath.Join(root, "bin")
	for _, d := range []string{s.home, s.ask, bin, filepath.Join(root, "repo")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for k, v := range map[string]string{
		"USERPROFILE": s.home, "HOME": s.home, "HORUS_HOME": s.user, "HORUS_ROOT": filepath.Join(root, "repo"),
		"LOCALAPPDATA": filepath.Join(s.home, "AppData", "Local"), "APPDATA": filepath.Join(s.home, "AppData", "Roaming"),
		"CODEX_HOME": filepath.Join(s.home, ".codex"), "PATH": bin,
		"HORUS_MODELS_DIR": "", "HORUS_TEXT_ENCODER_DIR": "", "HORUS_COMFYUI_DIR": "", "HORUS_SPEECH_DIR": "",
	} {
		t.Setenv(k, v)
	}
	settings.Reset()
	t.Cleanup(settings.Reset)

	// The workflow demo, and its two model files in the models folder of ComfyUI.
	s.workflows = filepath.Join(root, "repo", "workflows")
	writeDemo(t, filepath.Join(s.workflows, "demo"))
	s.comfy = filepath.Join(s.home, "ComfyUI_windows_portable")
	s.models = filepath.Join(s.comfy, "ComfyUI", "models")
	s.vae = filepath.Join(s.models, "vae", "demo-vae.safetensors")
	s.te = filepath.Join(s.models, "text_encoders", "demo-te.safetensors")
	mkfile(t, s.vae, vaeData)
	mkfile(t, s.te, teData)
	mkfile(t, filepath.Join(s.comfy, "ComfyUI", "main.py"), []byte("# fake\n"))
	s.venv = filepath.Join(s.home, ".venvs", "comfy-mcp")
	mkfile(t, platform.VenvBin(s.venv, "python"), []byte("fake"))
	if !o.noRecord {
		for _, c := range []struct{ dir, kind string }{{s.comfy, platform.RecordComfyUI}, {s.venv, platform.RecordComfyMCP}} {
			if err := platform.WriteInstallRecord(c.dir, c.kind); err != nil {
				t.Fatal(err)
			}
		}
	}
	// The user folder: the copy of the built-in workflows, a temporary copy,
	// and the data of the user.
	mkfile(t, filepath.Join(s.user, "builtin-workflows", "demo", "README.md"), []byte("# demo\n"))
	mkfile(t, filepath.Join(s.user, "builtin-workflows.tmp-123", "x"), []byte("x"))
	mkfile(t, filepath.Join(s.user, ".env"), []byte("HORUS_AUTOSTART=0\n"))
	mkfile(t, filepath.Join(s.user, "outputs", "a.png"), []byte("png"))
	// The program and the PATH entry of the install script.
	s.program = filepath.Join(s.home, "AppData", "Local", "Programs", "horus", platform.ExeName("horus"))
	mkfile(t, s.program, []byte("program"))
	s.profile = filepath.Join(s.home, ".zprofile")
	mkfile(t, s.profile, []byte("export A=1\n\nexport PATH=\"x:$PATH\" "+platform.ProfileMarker+"\n"))

	// The fake clients: copies of the test binary, the only programs of the PATH.
	for _, name := range []string{"claude", "codex"} {
		_, p := fakeClient(t, name)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bin, filepath.Base(p)), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	known := ""
	if o.known {
		known = "1"
	}
	t.Setenv("HORUS_FAKE_KNOWN", known)
	recFile := filepath.Join(root, "record", "record.txt")
	if err := os.MkdirAll(filepath.Dir(recFile), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HORUS_FAKE_CLAUDE", recFile)
	s.read = func() string { b, _ := os.ReadFile(recFile); return string(b) }

	s.comfyFake = &fakeComfyInstall{folder: s.comfy}
	s.svc = New(diskFiles{root: s.models}, nil, nil, nil, "").WithUninstall(UninstallSystem{
		Comfy: s.comfyFake, OS: &fakeOS{profile: s.profile, program: s.program}, Programs: platform.Programs{}, AskDir: s.ask})
	return s
}

// writeDemo writes the workflow demo with its two model files into dir.
func writeDemo(t *testing.T, dir string) {
	t.Helper()
	mkfile(t, filepath.Join(dir, "models.json"), []byte(fmt.Sprintf(`{"repo":"org/repo","revision":"abc","license":"test license","files":[`+
		`{"role":"vae","repo_path":"vae/demo-vae.safetensors","folder":"vae","bytes":%d,"sha256":"%s"},`+
		`{"role":"text_encoder","repo_path":"te/demo-te.safetensors","folder":"text_encoders","bytes":%d,"sha256":"%s"}]}`,
		len(vaeData), sum(vaeData), len(teData), sum(teData))))
	mkfile(t, filepath.Join(dir, "README.md"), []byte("# demo\n"))
}

// inside reports if p is the folder root or a path in it.
func inside(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && filepath.IsAbs(p) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// planInside fails the test when a path of the plan is not in the sandbox.
// Every test of uninstall calls it before RunUninstall (the uninstall rule of
// the spec).
func planInside(t *testing.T, root string, u *Uninstall) {
	t.Helper()
	for _, p := range u.Paths() {
		if !inside(root, p) {
			t.Fatalf("the plan names %s, outside the sandbox %s", p, root)
		}
	}
}

func (s *unSandbox) plan(t *testing.T, req UninstallRequest) *Uninstall {
	t.Helper()
	req.WorkflowsDir = s.workflows
	u, err := s.svc.UninstallPlan(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	planInside(t, s.root, u)
	return u
}

// run makes the plan, checks that it is in the sandbox, then runs it.
func (s *unSandbox) run(t *testing.T, req UninstallRequest) (*Uninstall, error) {
	t.Helper()
	u := s.plan(t, req)
	return u, s.svc.RunUninstall(context.Background(), u, io.Discard)
}

// files is every file and folder under the home folder, with the size and
// the time of each file.
func (s *unSandbox) files(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(s.home, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			fmt.Fprintf(&b, "%s/\n", p)
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s %d %d\n", p, fi.Size(), fi.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func itemLines(u *Uninstall) string {
	var b strings.Builder
	for _, it := range u.Items {
		w := "KEEP"
		if it.Remove {
			w = "REMOVE"
		}
		fmt.Fprintf(&b, "%s %s %s | %s\n", w, it.Part, it.Path, it.Why)
	}
	return b.String()
}

func findItem(u *Uninstall, path string) *UninstallItem {
	for i := range u.Items {
		if u.Items[i].Path == path {
			return &u.Items[i]
		}
	}
	return nil
}

// TestUninstallPlanWithoutYesChangesNothing: the plan names each part, and
// no file of the sandbox changes. The clients get only "mcp get".
func TestUninstallPlanWithoutYesChangesNothing(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{known: true})
	before := s.files(t)
	u := s.plan(t, UninstallRequest{})
	if after := s.files(t); after != before {
		t.Fatalf("the plan changed files.\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if strings.Contains(s.read(), "mcp remove") {
		t.Errorf("the plan ran mcp remove:\n%s", s.read())
	}
	if s.comfyFake.stops != 0 {
		t.Error("the plan stopped ComfyUI")
	}
	for _, p := range []string{s.vae, s.te, s.comfy, s.venv, filepath.Join(s.user, "builtin-workflows"), s.profile, s.program} {
		if it := findItem(u, p); it == nil || !it.Remove {
			t.Errorf("no REMOVE item for %s:\n%s", p, itemLines(u))
		}
	}
	if n := u.Removes(); n != 10 {
		t.Errorf("%d REMOVE items, want 10 (2 model files, ComfyUI, venv, 2 clients, 2 user files, PATH, program):\n%s", n, itemLines(u))
	}
}

// TestUninstallYesRemovesEachRemoveItem: after the run, no REMOVE item is
// left, and the run reports no error. The ComfyUI that horus started is
// stopped before its folder goes.
func TestUninstallYesRemovesEachRemoveItem(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{known: true})
	s.comfyFake.answers, s.comfyFake.started = true, true
	u, err := s.run(t, UninstallRequest{})
	if err != nil {
		t.Fatalf("RunUninstall: %v\n%s", err, itemLines(u))
	}
	for _, it := range u.Items {
		if !it.Remove {
			continue
		}
		if !it.Done || it.Err != nil {
			t.Errorf("%s %s: done %v, err %v", it.What, it.Path, it.Done, it.Err)
		}
		if it.Part != PartMCP && it.Part != PartPath && exists(it.Path) {
			t.Errorf("%s is still there", it.Path)
		}
	}
	if s.comfyFake.stops != 1 || !s.comfyFake.folderAtStop {
		t.Errorf("Stop ran %d times, folder there at the stop: %v; want 1 time, before the removal", s.comfyFake.stops, s.comfyFake.folderAtStop)
	}
	b, _ := os.ReadFile(s.profile)
	if string(b) != "export A=1\n\n" {
		t.Errorf("the profile = %q", b)
	}
}

// TestUninstallKeepsAFolderWithoutAnInstallRecord: a ComfyUI folder and a
// venv without the install record are KEEP items, and they stay with all
// their files.
func TestUninstallKeepsAFolderWithoutAnInstallRecord(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{noRecord: true})
	u, err := s.run(t, UninstallRequest{})
	if err != nil {
		t.Fatalf("RunUninstall: %v\n%s", err, itemLines(u))
	}
	for _, p := range []string{s.comfy, s.venv} {
		it := findItem(u, p)
		if it == nil || it.Remove || !strings.Contains(it.Why, "no install record") {
			t.Errorf("%s: want a KEEP item with the reason of the record:\n%s", p, itemLines(u))
		}
	}
	if !platform.IsFile(filepath.Join(s.comfy, "ComfyUI", "main.py")) || !platform.IsFile(platform.VenvBin(s.venv, "python")) {
		t.Error("a folder without an install record was deleted")
	}
}

// TestUninstallKeepsTheEnvFileAndTheOutputsFolder: the plan names .env and
// outputs/ as KEEP items, and both stay after the run.
func TestUninstallKeepsTheEnvFileAndTheOutputsFolder(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{})
	env, outputs := filepath.Join(s.user, ".env"), filepath.Join(s.user, "outputs")
	u, err := s.run(t, UninstallRequest{})
	if err != nil {
		t.Fatalf("RunUninstall: %v\n%s", err, itemLines(u))
	}
	for _, p := range []string{env, outputs} {
		if it := findItem(u, p); it == nil || it.Remove || it.Why != whyUserData {
			t.Errorf("no KEEP item for %s:\n%s", p, itemLines(u))
		}
	}
	if !platform.IsFile(env) || !platform.IsFile(filepath.Join(outputs, "a.png")) {
		t.Error(".env or an image of outputs/ was deleted")
	}
	if exists(filepath.Join(s.user, "builtin-workflows")) || exists(filepath.Join(s.user, "builtin-workflows.tmp-123")) {
		t.Error("the copy of the built-in workflows is still there")
	}
}

// TestUninstallModelsLeavesComfyUIAndTheRegistration: with -models only the
// model files go. ComfyUI, the venv, the user folder, the program and the
// registration stay.
func TestUninstallModelsLeavesComfyUIAndTheRegistration(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{known: true})
	u, err := s.run(t, UninstallRequest{Models: true})
	if err != nil {
		t.Fatalf("RunUninstall: %v\n%s", err, itemLines(u))
	}
	for _, it := range u.Items {
		if it.Part != PartModels {
			t.Errorf("an item of part %s: %s", it.Part, it.Path)
		}
	}
	if exists(s.vae) || exists(s.te) {
		t.Error("a model file is still there")
	}
	for _, p := range []string{filepath.Join(s.comfy, "ComfyUI", "main.py"), platform.VenvBin(s.venv, "python"), s.program,
		filepath.Join(s.user, "builtin-workflows", "demo", "README.md")} {
		if !platform.IsFile(p) {
			t.Errorf("%s was deleted", p)
		}
	}
	if got := s.read(); strings.Contains(got, "mcp ") {
		t.Errorf("-models ran a client:\n%s", got)
	}
}

// TestUninstallMCPLeavesTheFiles: with -mcp only the registration goes, and
// no file of the home folder changes.
func TestUninstallMCPLeavesTheFiles(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{known: true})
	before := s.files(t)
	u, err := s.run(t, UninstallRequest{MCP: true})
	if err != nil {
		t.Fatalf("RunUninstall: %v\n%s", err, itemLines(u))
	}
	if n := u.Removes(); n != 2 || len(u.Items) != 2 {
		t.Errorf("%d items, %d REMOVE, want the 2 clients:\n%s", len(u.Items), n, itemLines(u))
	}
	if after := s.files(t); after != before {
		t.Fatalf("-mcp changed files.\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestUninstallKeepsAComfyUIThatAnswersAndHorusDidNotStart: a ComfyUI that
// answers and that horus did not start keeps its folder, also with an
// install record, and horus does not stop it.
func TestUninstallKeepsAComfyUIThatAnswersAndHorusDidNotStart(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{})
	s.comfyFake.answers = true
	u, err := s.run(t, UninstallRequest{})
	if err != nil {
		t.Fatalf("RunUninstall: %v\n%s", err, itemLines(u))
	}
	it := findItem(u, s.comfy)
	if it == nil || it.Remove || !strings.Contains(it.Why, "horus did not start it") {
		t.Fatalf("want a KEEP item for the ComfyUI folder:\n%s", itemLines(u))
	}
	if !platform.IsFile(filepath.Join(s.comfy, "ComfyUI", "main.py")) || s.comfyFake.stops != 0 {
		t.Errorf("the folder was deleted, or Stop ran %d times", s.comfyFake.stops)
	}
}

// TestUninstallRunsMCPRemoveOfBothClients: the fake claude and the fake codex
// get the "mcp remove" arguments of their own command line.
func TestUninstallRunsMCPRemoveOfBothClients(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{known: true})
	u, err := s.run(t, UninstallRequest{MCP: true})
	if err != nil {
		t.Fatalf("RunUninstall: %v\n%s", err, itemLines(u))
	}
	got := s.read()
	for _, w := range []string{"args mcp remove --scope user horus\n", "args mcp remove horus\n"} {
		if strings.Count(got, w) != 1 {
			t.Errorf("the clients did not get %q one time:\n%s", w, got)
		}
	}
	for _, it := range u.Items {
		if !it.Done {
			t.Errorf("%s is not done", it.What)
		}
	}

	// A client that does not know the server gets no remove.
	s2 := newUnSandbox(t, sandboxOptions{})
	u2, err := s2.run(t, UninstallRequest{MCP: true})
	if err != nil || len(u2.Items) != 0 || strings.Contains(s2.read(), "mcp remove") {
		t.Errorf("unknown server: err %v, items:\n%s\nrecord:\n%s", err, itemLines(u2), s2.read())
	}
}

// TestUninstallReportsAnItemThatStays: a model file that cannot be deleted
// makes the run fail and names the item.
func TestUninstallReportsAnItemThatStays(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{})
	undo, err := platform.BlockDelete(s.vae)
	if err != nil {
		t.Skip("cannot block a delete here:", err)
	}
	defer undo()
	u, err := s.run(t, UninstallRequest{Models: true})
	if err == nil || !strings.Contains(err.Error(), "1 of 2 REMOVE item(s) are still there") {
		t.Fatalf("err = %v", err)
	}
	if it := findItem(u, s.vae); it == nil || it.Done || it.Err == nil {
		t.Errorf("the blocked file is not reported: %+v", it)
	}
}

// TestUninstallKeepsAModelFileOfAnotherSize: a file whose size differs from
// models.json is a KEEP item, by the rules of horus workflow remove.
func TestUninstallKeepsAModelFileOfAnotherSize(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{})
	mkfile(t, s.vae, []byte("a file of a person, not the listed file"))
	u, err := s.run(t, UninstallRequest{Models: true})
	if err != nil {
		t.Fatalf("RunUninstall: %v\n%s", err, itemLines(u))
	}
	if it := findItem(u, s.vae); it == nil || it.Remove {
		t.Fatalf("want a KEEP item for %s:\n%s", s.vae, itemLines(u))
	}
	if !platform.IsFile(s.vae) || exists(s.te) {
		t.Error("the file of another size was deleted, or the listed file stayed")
	}
}

// TestUninstallPlanSaysThatAKeptFileInTheComfyUIFolderGoesWithIt: the pure
// plan. A model file of another size is a KEEP item, but when it is in a
// ComfyUI folder that goes, its line says that it goes with the folder.
func TestUninstallPlanSaysThatAKeptFileInTheComfyUIFolderGoesWithIt(t *testing.T) {
	comfy := filepath.Join(string(filepath.Separator), "c", "ComfyUI_windows_portable")
	inside := filepath.Join(comfy, "ComfyUI", "models", "vae", "v.safetensors")
	outside := filepath.Join(string(filepath.Separator), "m", "te.safetensors")
	f := UninstallFacts{Parts: UninstallRequest{}.parts(), ComfyFolder: comfy, ComfyExists: true, ComfyRecord: true,
		Models: []ModelFact{
			{Workflow: "w", Item: Item{Status: StatusRefused, File: "v.safetensors", Path: inside, Detail: "another size"}},
			{Workflow: "w", Item: Item{Status: StatusRefused, File: "te.safetensors", Path: outside, Detail: "another size"}},
		}}
	items := planUninstall(f)
	if items[0].Remove || !strings.HasSuffix(items[0].Why, "so it goes with that folder") {
		t.Errorf("the file in the folder: %+v", items[0])
	}
	if items[1].Why != "another size" {
		t.Errorf("the file outside the folder: %+v", items[1])
	}
	if !items[2].Remove || items[2].Part != PartComfyUI || !strings.Contains(items[2].Why, "its target stays") {
		t.Errorf("the ComfyUI folder: %+v", items[2])
	}
	f.ComfyRecord = false
	if items := planUninstall(f); items[0].Why != "another size" {
		t.Errorf("a kept ComfyUI folder: the file line says %q", items[0].Why)
	}
}

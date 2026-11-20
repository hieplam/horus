// Tests of the uninstall of a speech workflow: Remove (horus workflow remove
// and uninstall_workflow) finds the model files in the speech folder, and
// horus uninstall removes each file of the speech setup (ADR 0032, ADR 0035).
// The disk tests use the sandbox of uninstall_test.go: every path of each plan
// is in a temporary folder.
package provision

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/speechsetup"
)

// TestRemoveOfASpeechWorkflowFindsItsFilesInTheSpeechFolder: the plan of
// Remove names the model file and its unfinished download in the speech
// folder, and Yes deletes both. An image workflow that lists the same folder
// and file name does not share the file: its file is in another folder.
func TestRemoveOfASpeechWorkflowFindsItsFilesInTheSpeechFolder(t *testing.T) {
	r := newSpeechRig(t)
	pic := filepath.Join(r.dir, "pic")
	if err := os.MkdirAll(pic, 0o755); err != nil {
		t.Fatal(err)
	}
	models, _ := json.Marshal(map[string]any{"repo": "org/pic", "revision": "abc", "license": "test", "files": []any{
		map[string]any{"role": "x", "repo_path": "model.bin", "folder": "tts", "bytes": len(speechData), "sha256": sum(speechData)}}})
	if err := os.WriteFile(filepath.Join(pic, "models.json"), models, 0o644); err != nil {
		t.Fatal(err)
	}
	r.files.files[filepath.Join(r.files.root, "tts", "model.bin")] = speechData // the file of pic
	r.files.files[r.model+".part"] = []byte("half")

	res, err := r.svc.Remove(context.Background(), Request{WorkflowsDir: r.dir, Workflow: "talk"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, it := range res.Items {
		got[it.Path] = it.Status
	}
	want := map[string]string{r.model: StatusWouldDelete, r.model + ".part": StatusWouldDelete}
	if len(got) != 2 || got[r.model] != want[r.model] || got[r.model+".part"] != want[r.model+".part"] {
		t.Fatalf("the plan: %v, want %v", got, want)
	}

	if _, err := r.svc.Remove(context.Background(), Request{WorkflowsDir: r.dir, Workflow: "talk", Yes: true}); err != nil {
		t.Fatal(err)
	}
	if r.files.Size(r.model) >= 0 || r.files.Size(r.model+".part") >= 0 {
		t.Errorf("a file of the speech folder is still there: %v", r.files.deleted)
	}
	if r.files.Size(filepath.Join(r.files.root, "tts", "model.bin")) < 0 {
		t.Error("the file of the image workflow was deleted")
	}
	if r.state(t) != "partially_installed" {
		t.Errorf("after the remove, talk is %s, want partially_installed (the speech venv stays)", r.state(t))
	}
}

// speechSetup is the speech setup of a sandbox: the speech workflow talk,
// and in the speech folder the speech venv, the model file voice.bin, the
// unfinished download of extra.bin and the file of the speech worker. Python
// 3.12 and FFmpeg are files in LOCALAPPDATA, as a package manager puts them.
type speechSetup struct {
	place          speechsetup.Place
	voice, extra   string // voice.bin, and extra.bin.part
	python, ffmpeg string // the system packages
}

func addSpeechSetup(t *testing.T, s *unSandbox, record bool) speechSetup {
	t.Helper()
	talk := filepath.Join(s.workflows, "talk")
	manifest, _ := json.Marshal(map[string]any{"kind": "speech"})
	models, _ := json.Marshal(map[string]any{"repo": "org/speech", "revision": "abc", "license": "test", "files": []any{
		map[string]any{"role": "speech", "repo_path": "voice.bin", "folder": "tts", "bytes": len(speechData), "sha256": sum(speechData)},
		map[string]any{"role": "codec", "repo_path": "extra.bin", "folder": "codec", "bytes": 99, "sha256": strings.Repeat("0", 64)}}})
	mkfile(t, filepath.Join(talk, "manifest.json"), manifest)
	mkfile(t, filepath.Join(talk, "models.json"), models)
	sp := speechSetup{place: speechsetup.Here()}
	if !inside(s.root, sp.place.Dir) {
		t.Fatalf("the speech folder %s is outside the sandbox %s", sp.place.Dir, s.root)
	}
	sp.voice = filepath.Join(sp.place.Models, "tts", "voice.bin")
	sp.extra = filepath.Join(sp.place.Models, "codec", "extra.bin.part")
	mkfile(t, sp.voice, speechData)
	mkfile(t, sp.extra, []byte("half"))
	mkfile(t, sp.place.Python, []byte("python"))
	mkfile(t, filepath.Join(sp.place.Venv, "Lib", "site-packages", "vieneu", "__init__.py"), []byte("# package"))
	if record {
		if err := platform.WriteInstallRecord(sp.place.Venv, platform.RecordSpeechVenv, speechsetup.Packages...); err != nil {
			t.Fatal(err)
		}
	}
	mkfile(t, sp.place.Script, []byte("# the speech worker"))
	local := filepath.Join(s.home, "AppData", "Local")
	sp.python = filepath.Join(local, "Programs", "Python", "Python312", "python.exe")
	sp.ffmpeg = filepath.Join(local, "Microsoft", "WinGet", "Links", "ffmpeg.exe")
	mkfile(t, sp.python, []byte("system python"))
	mkfile(t, sp.ffmpeg, []byte("system ffmpeg"))
	return sp
}

// filesUnder lists every file under root ("" for a missing root).
func filesUnder(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// TestUninstallRemovesEachFileOfTheSpeechSetup: the plan names the model
// file, the unfinished download, the speech venv and the file of the speech
// worker as REMOVE items. After the run the speech folder is gone, with 0
// files, and Python 3.12 and FFmpeg stay.
func TestUninstallRemovesEachFileOfTheSpeechSetup(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{})
	sp := addSpeechSetup(t, s, true)
	u := s.plan(t, UninstallRequest{})
	for _, p := range []string{sp.voice, sp.extra, sp.place.Venv, sp.place.Script} {
		if it := findItem(u, p); it == nil || !it.Remove {
			t.Errorf("no REMOVE item for %s:\n%s", p, itemLines(u))
		}
	}
	for _, it := range u.Items {
		if strings.Contains(it.Path, "Python312") || strings.Contains(it.Path, "WinGet") {
			t.Errorf("the plan names a system package: %s", it.Path)
		}
	}
	if err := s.svc.RunUninstall(context.Background(), u, io.Discard); err != nil {
		t.Fatalf("RunUninstall: %v\n%s", err, itemLines(u))
	}
	if left := filesUnder(t, sp.place.Dir); len(left) != 0 {
		t.Errorf("files of the speech setup are left: %v", left)
	}
	if exists(sp.place.Dir) {
		t.Errorf("the empty speech folder %s is still there", sp.place.Dir)
	}
	if !platform.IsFile(sp.python) || !platform.IsFile(sp.ffmpeg) {
		t.Error("a system package was removed")
	}
}

// TestUninstallKeepsASpeechVenvWithoutTheInstallRecord: a speech venv that
// horus did not make is a KEEP item and stays with its files. The model
// files and the file of the speech worker go.
func TestUninstallKeepsASpeechVenvWithoutTheInstallRecord(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{})
	sp := addSpeechSetup(t, s, false)
	u, err := s.run(t, UninstallRequest{})
	if err != nil {
		t.Fatalf("RunUninstall: %v\n%s", err, itemLines(u))
	}
	if it := findItem(u, sp.place.Venv); it == nil || it.Remove || it.Why != whyNoRecord {
		t.Errorf("want a KEEP item for the speech venv:\n%s", itemLines(u))
	}
	if !platform.IsFile(sp.place.Python) {
		t.Error("the speech venv without the install record was deleted")
	}
	for _, p := range []string{sp.voice, sp.extra, sp.place.Script} {
		if exists(p) {
			t.Errorf("%s is still there", p)
		}
	}
}

// TestUninstallModelsRemovesOnlyTheSpeechModelFiles: with -models the model
// files of the speech folder go, and the speech venv and the file of the
// speech worker stay.
func TestUninstallModelsRemovesOnlyTheSpeechModelFiles(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{})
	sp := addSpeechSetup(t, s, true)
	u, err := s.run(t, UninstallRequest{Models: true})
	if err != nil {
		t.Fatalf("RunUninstall: %v\n%s", err, itemLines(u))
	}
	if exists(sp.voice) || exists(sp.extra) {
		t.Error("a speech model file is still there")
	}
	if !platform.IsFile(sp.place.Python) || !platform.IsFile(sp.place.Script) {
		t.Error("-models removed the speech venv or the file of the speech worker")
	}
}

// TestUninstallNeverEntersALinkInTheSpeechFolder: a link (or a junction) in
// the speech folder to a folder outside it stays, and so does the file of
// that folder. The speech folder stays, because the link is in it.
func TestUninstallNeverEntersALinkInTheSpeechFolder(t *testing.T) {
	s := newUnSandbox(t, sandboxOptions{})
	sp := addSpeechSetup(t, s, true)
	outside := filepath.Join(s.root, "outside")
	keep := filepath.Join(outside, "a.txt")
	mkfile(t, keep, []byte("data of a person"))
	link := filepath.Join(sp.place.Models, "outside")
	if os.Symlink(outside, link) != nil {
		shell := os.Getenv("ComSpec") // the PATH of the sandbox holds only fakes
		if shell == "" {
			t.Skip("no symbolic link and no cmd here")
		}
		if out, err := exec.Command(shell, "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
			t.Skipf("no link: %v %s", err, out)
		}
	}
	u, err := s.run(t, UninstallRequest{})
	if err != nil {
		t.Fatalf("RunUninstall: %v\n%s", err, itemLines(u))
	}
	if !platform.IsFile(keep) {
		t.Error("the file behind the link was deleted")
	}
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("the link was removed: %v", err)
	}
	if exists(sp.voice) || exists(sp.place.Venv) {
		t.Error("a file of the speech setup is still there")
	}
}

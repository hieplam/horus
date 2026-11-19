// Tests of the setup plan of a speech workflow (speechplan.go) on a speech
// folder on the disk and a fake of the programs of the machine. The fake
// package manager and the fake Python make the files that the real programs
// make. No test runs a real program or downloads a file.
package provision

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

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/speechsetup"
)

// planRig is the speech workflow "talk" of newSpeechRig, with an empty speech
// folder on the disk, the packages of the speech setup in a temporary folder,
// and a fake winget. Run of the fake programs makes what the real ones make:
// winget makes python.exe or ffmpeg and ffprobe in the folders of the
// package, "python -m venv" makes the Python of the venv.
type planRig struct {
	dir     string // the workflows folder
	place   speechsetup.Place
	progs   *fakePrograms
	log     []string
	dl      *fakeDownloader
	svc     *Service
	sys     System
	pyDir   string // the folder of Python 3.12
	ffDir   string // the folder of FFmpeg
	manager string // the path of winget
	pipCode int    // the exit code of the pip install
}

// pathPrograms is fakePrograms whose LookPath also finds a program in the
// folders of the PATH of this process, as exec.LookPath does.
type pathPrograms struct{ *fakePrograms }

func (p pathPrograms) LookPath(name string) (string, error) {
	if path, err := p.fakePrograms.LookPath(name); err == nil {
		return path, nil
	}
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if f := filepath.Join(d, platform.ExeName(name)); platform.IsFile(f) {
			return f, nil
		}
	}
	return "", errors.New("not found")
}

func newPlanRig(t *testing.T) *planRig {
	t.Helper()
	sr := newSpeechRig(t)
	tmp := t.TempDir()
	t.Setenv("PATH", filepath.Join(tmp, "empty")) // no real ffmpeg of this machine
	r := &planRig{dir: sr.dir, place: speechsetup.Here(), pyDir: filepath.Join(tmp, "Python312"),
		ffDir:   filepath.Join(tmp, "WinGet", "Packages", "Gyan.FFmpeg_x", "ffmpeg-9.0-full_build", "bin"),
		manager: filepath.Join(tmp, "bin", "winget.exe")}
	r.progs = &fakePrograms{paths: map[string]string{"winget": r.manager}, log: &r.log}
	r.progs.onRun = func(c platform.Command) int {
		args := strings.Join(c.Args, " ")
		switch {
		case c.Path == r.manager && strings.Contains(args, "Python.Python.3.12"):
			touch(t, filepath.Join(r.pyDir, "python.exe"))
		case c.Path == r.manager && strings.Contains(args, "Gyan.FFmpeg"):
			touch(t, filepath.Join(r.ffDir, platform.ExeName("ffmpeg")))
			touch(t, filepath.Join(r.ffDir, platform.ExeName("ffprobe")))
		case strings.Contains(args, "-m venv"):
			touch(t, platform.VenvBin(c.Args[len(c.Args)-1], "python"))
		case strings.Contains(args, "-m pip install"):
			return r.pipCode
		}
		return 0
	}
	r.dl = &fakeDownloader{fn: func(_ context.Context, f Fetch, _ io.Writer) (bool, error) {
		r.log = append(r.log, "download "+filepath.Base(f.Dest))
		if err := os.MkdirAll(filepath.Dir(f.Dest), 0o755); err != nil {
			return false, err
		}
		return false, os.WriteFile(f.Dest, speechData, 0o644)
	}}
	r.sys = System{Programs: pathPrograms{r.progs}, Speech: platform.SpeechPackages{
		Python: &platform.Package{Name: "Python 3.12", Program: "python3.12", Exe: "python.exe", Dirs: []string{r.pyDir},
			Manager: "winget", Args: []string{"install", "--id", "Python.Python.3.12"}, URL: "https://www.python.org/downloads/"},
		FFmpeg: &platform.Package{Name: "FFmpeg", Program: "ffmpeg",
			Dirs:    []string{filepath.Join(tmp, "WinGet", "Links"), filepath.Join(tmp, "WinGet", "Packages", "Gyan.FFmpeg_*", "ffmpeg-*", "bin")},
			Manager: "winget", Args: []string{"install", "--id", "Gyan.FFmpeg"}, URL: "https://ffmpeg.org/download.html"},
	}}
	files := diskFiles{root: filepath.Join(tmp, "comfy-models")}
	r.svc = New(files, r.dl, nil, &noComfyUI{newMemFiles()}, "").WithSystem(r.sys)
	return r
}

func (r *planRig) req() Request { return Request{WorkflowsDir: r.dir, Workflow: "talk"} }

func (r *planRig) plan(t *testing.T) Plan {
	t.Helper()
	pl, err := r.svc.Plan(context.Background(), r.req())
	if err != nil {
		t.Fatal(err)
	}
	return pl
}

// snapshot is each file under the folder root.
func snapshot(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// TestSpeechPlanOfAnEmptyMachine: no Python 3.12, no FFmpeg and no speech
// setup give the 4 steps of the spec in order, with no ComfyUI step, and the
// plan changes nothing.
func TestSpeechPlanOfAnEmptyMachine(t *testing.T) {
	r := newPlanRig(t)
	before := snapshot(t, filepath.Dir(r.place.Dir))
	pl := r.plan(t)
	if got := kinds(pl.Steps); got != "package, package, speech-venv, file speech" {
		t.Fatalf("steps: %s", got)
	}
	if pl.Steps[0].Name != "Python 3.12" || pl.Steps[1].Name != "FFmpeg" || pl.Steps[0].Command[0] != r.manager {
		t.Errorf("package steps: %+v %+v", pl.Steps[0], pl.Steps[1])
	}
	venv := pl.Steps[2]
	if venv.Dir != r.place.Venv || !slices.Equal(venv.Command, append([]string{r.place.Python, "-m", "pip", "install", "--disable-pip-version-check"}, speechsetup.Packages...)) {
		t.Errorf("venv step: %+v", venv)
	}
	if f := pl.Steps[3].File; f.Path != filepath.Join(r.place.Models, "tts", "model.bin") || f.File != "model.bin" {
		t.Errorf("file step: %+v", f)
	}
	if pl.Speech == nil || pl.BackendDir != "" || pl.Package != nil {
		t.Errorf("a speech plan has image facts: %+v", pl)
	}
	if len(r.progs.runs()) != 0 || !slices.Equal(snapshot(t, filepath.Dir(r.place.Dir)), before) {
		t.Errorf("the plan changed something: runs %v", r.progs.runs())
	}
}

// TestSpeechPlanSkipsWhatIsThere: Python 3.12 in the folder of its package,
// ffmpeg and ffprobe on the PATH give only the venv step and the file step.
// A ready venv and the model file give 0 steps.
func TestSpeechPlanSkipsWhatIsThere(t *testing.T) {
	r := newPlanRig(t)
	touch(t, filepath.Join(r.pyDir, "python.exe"))
	r.progs.paths["ffmpeg"], r.progs.paths["ffprobe"] = "/bin/ffmpeg", "/bin/ffprobe"
	pl := r.plan(t)
	if got := kinds(pl.Steps); got != "speech-venv, file speech" || pl.Speech.PythonPath != filepath.Join(r.pyDir, "python.exe") {
		t.Fatalf("steps: %s, Python %q", got, pl.Speech.PythonPath)
	}
	touch(t, r.place.Python)
	if err := platform.WriteInstallRecord(r.place.Venv, platform.RecordSpeechVenv, speechsetup.Packages...); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.place.Models, "tts", "model.bin"), speechData, 0o644); err != nil {
		if err = os.MkdirAll(filepath.Join(r.place.Models, "tts"), 0o755); err == nil {
			err = os.WriteFile(filepath.Join(r.place.Models, "tts", "model.bin"), speechData, 0o644)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if pl = r.plan(t); len(pl.Steps) != 0 || !pl.Speech.VenvReady {
		t.Errorf("a ready machine: steps %s", kinds(pl.Steps))
	}
}

// TestSpeechPlanOfAVenvWhosePipInstallDidNotEnd: a venv with its Python and an
// install record without the packages gets the venv step again, and no Python
// step: the venv has its Python.
func TestSpeechPlanOfAVenvWhosePipInstallDidNotEnd(t *testing.T) {
	r := newPlanRig(t)
	r.progs.paths["ffmpeg"], r.progs.paths["ffprobe"] = "/bin/ffmpeg", "/bin/ffprobe"
	touch(t, r.place.Python)
	if err := platform.WriteInstallRecord(r.place.Venv, platform.RecordSpeechVenv); err != nil {
		t.Fatal(err)
	}
	pl := r.plan(t)
	if got := kinds(pl.Steps); got != "speech-venv, file speech" || !strings.Contains(pl.Speech.VenvWhy, "the pip install did not end") {
		t.Fatalf("steps: %s, why %q", got, pl.Speech.VenvWhy)
	}
	res, err := r.svc.Check(context.Background(), Request{WorkflowsDir: r.dir, Workflow: "talk", Quick: true})
	if err == nil || !strings.HasPrefix(statuses(res), "MISSING venv ") || !strings.Contains(res.Items[0].Detail, "the pip install did not end") {
		t.Errorf("check: %v %q", err, statuses(res))
	}
}

// TestSpeechPlanWithoutAPackageManagerIsAStepForAPerson: no winget gives a
// step for a person for each missing package, and RunPlan refuses the plan
// before any step.
func TestSpeechPlanWithoutAPackageManagerIsAStepForAPerson(t *testing.T) {
	r := newPlanRig(t)
	delete(r.progs.paths, "winget")
	pl := r.plan(t)
	if got := kinds(pl.Steps); got != "person, person, speech-venv, file speech" {
		t.Fatalf("steps: %s", got)
	}
	var person *PersonStepError
	if _, err := r.svc.RunPlan(context.Background(), Request{WorkflowsDir: r.dir, Workflow: "talk", WithPackages: true}, pl, Watch{}); !errors.As(err, &person) {
		t.Fatalf("RunPlan: %v", err)
	}
	if len(r.progs.runs()) != 0 || len(r.dl.calls) != 0 {
		t.Errorf("RunPlan ran %v and downloaded %d files", r.progs.runs(), len(r.dl.calls))
	}
}

// TestRunSpeechPlanRunsTheStepsInOrder: the plan of an empty machine runs
// winget for Python 3.12 and for FFmpeg, makes the venv with the new Python,
// runs the pip install of the pinned packages, writes the install record with
// them, then downloads the model file into the speech folder. The folder of
// FFmpeg is then first on the PATH of this process, each part of the speech
// setup is there, and the next plan has 0 steps.
func TestRunSpeechPlanRunsTheStepsInOrder(t *testing.T) {
	r := newPlanRig(t)
	pl := r.plan(t)
	if _, err := r.svc.RunPlan(context.Background(), r.req(), pl, Watch{}); !errors.As(err, new(*PackageApprovalError)) {
		t.Fatalf("RunPlan without WithPackages: %v", err)
	}
	if len(r.log) != 0 {
		t.Fatalf("RunPlan without WithPackages ran %v", r.log)
	}
	req := r.req()
	req.WithPackages = true
	var out strings.Builder
	res, err := r.svc.RunPlan(context.Background(), req, pl, Watch{Out: &out})
	if err != nil {
		t.Fatalf("RunPlan: %v\n%s", err, out.String())
	}
	py := filepath.Join(r.pyDir, "python.exe")
	want := []string{
		"run " + r.manager + " install --id Python.Python.3.12",
		"run " + r.manager + " install --id Gyan.FFmpeg",
		"run " + py + " -m venv " + r.place.Venv,
		"run " + strings.Join(pipCommand(r.place.Python), " "),
		"download model.bin",
	}
	if !slices.Equal(r.log, want) {
		t.Fatalf("the steps:\n%s\nwant:\n%s", strings.Join(r.log, "\n"), strings.Join(want, "\n"))
	}
	if got, ok := platform.InstallRecordPackages(r.place.Venv, platform.RecordSpeechVenv); !ok || !slices.Equal(got, speechsetup.Packages) {
		t.Errorf("the install record: %v %v", got, ok)
	}
	if first := filepath.SplitList(os.Getenv("PATH"))[0]; first != r.ffDir {
		t.Errorf("the PATH starts with %q, want the folder of FFmpeg %q", first, r.ffDir)
	}
	if !res.OK || statuses(res) != "DOWNLOADED download speech, OK venv , OK program , OK program , OK file speech" {
		t.Errorf("items: %q", statuses(res))
	}
	if pl = r.plan(t); len(pl.Steps) != 0 {
		t.Errorf("the next plan: %s", kinds(pl.Steps))
	}
}

// TestRunSpeechPlanEndsWithTheCheckOfEachPart: a plan whose steps end
// without each part (no FFmpeg on an OS without its package) fails with the
// missing part, so install_workflow does not answer "installed" when
// generate_speech would answer WORKFLOW_NOT_INSTALLED.
func TestRunSpeechPlanEndsWithTheCheckOfEachPart(t *testing.T) {
	r := newPlanRig(t)
	r.sys.Speech.FFmpeg = nil
	r.svc.WithSystem(r.sys)
	touch(t, filepath.Join(r.pyDir, "python.exe"))
	pl := r.plan(t)
	if got := kinds(pl.Steps); got != "speech-venv, file speech" {
		t.Fatalf("steps: %s", got)
	}
	res, err := r.svc.RunPlan(context.Background(), r.req(), pl, Watch{})
	if err == nil || !strings.Contains(err.Error(), "2 part(s) of the speech setup are missing") ||
		!strings.Contains(statuses(res), "MISSING program , MISSING program ") {
		t.Fatalf("RunPlan: %v, items %q", err, statuses(res))
	}
	// A plan with no file step also ends with the check.
	pl = r.plan(t)
	if got := kinds(pl.Steps); got != "" {
		t.Fatalf("steps: %s", got)
	}
	pl.Steps = []Step{{Kind: StepSpeechVenv, Name: "speech venv", Dir: r.place.Venv}}
	if _, err := r.svc.RunPlan(context.Background(), r.req(), pl, Watch{}); err == nil {
		t.Errorf("a plan without FFmpeg and without a file step ends without an error")
	}
}

// TestSpeechVenvStepWritesTheRecordBeforeAndAfterThePipInstall: a pip install
// that fails leaves the install record without packages, so the uninstall can
// remove the venv and the next plan has the venv step again. The next run
// does not make the venv again.
func TestSpeechVenvStepWritesTheRecordBeforeAndAfterThePipInstall(t *testing.T) {
	r := newPlanRig(t)
	r.pipCode = 1
	st := Step{Kind: StepSpeechVenv, Dir: r.place.Venv}
	py := filepath.Join(r.pyDir, "python.exe")
	err := r.svc.installSpeechVenv(context.Background(), st, py, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "pip install vieneu==3.8.4 faster-whisper==1.2.1 failed (exit code 1)") {
		t.Fatalf("err %v", err)
	}
	if got, ok := platform.InstallRecordPackages(r.place.Venv, platform.RecordSpeechVenv); !ok || len(got) != 0 {
		t.Fatalf("after a failed pip install: record %v %v", got, ok)
	}
	r.pipCode, r.log = 0, nil
	if err := r.svc.installSpeechVenv(context.Background(), st, py, io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(r.log) != 1 || !strings.Contains(r.log[0], "-m pip install") {
		t.Errorf("the second run: %v", r.log)
	}
	if got, _ := platform.InstallRecordPackages(r.place.Venv, platform.RecordSpeechVenv); !slices.Equal(got, speechsetup.Packages) {
		t.Errorf("the record: %v", got)
	}
}

// TestSpeechVenvStepRefusesAFolderThatHorusDidNotMake: a venv folder with
// files and without the install record stays as it is.
func TestSpeechVenvStepRefusesAFolderThatHorusDidNotMake(t *testing.T) {
	r := newPlanRig(t)
	touch(t, filepath.Join(r.place.Venv, "notes.txt"))
	err := r.svc.installSpeechVenv(context.Background(), Step{Kind: StepSpeechVenv, Dir: r.place.Venv}, filepath.Join(r.pyDir, "python.exe"), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "it is not a speech venv that horus made") || len(r.log) != 0 {
		t.Fatalf("err %v, runs %v", err, r.log)
	}
	err = r.svc.installSpeechVenv(context.Background(), Step{Kind: StepSpeechVenv, Dir: filepath.Join(t.TempDir(), "venv")}, "", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "Python 3.12 was not found") {
		t.Errorf("no Python: %v", err)
	}
}

// TestPackageStepAcceptsAnErrorWhenTheProgramIsThere: winget ends with an
// error when the package is already installed. When the program is in a
// folder of the package after the run, the step goes on.
func TestPackageStepAcceptsAnErrorWhenTheProgramIsThere(t *testing.T) {
	r := newPlanRig(t)
	touch(t, filepath.Join(r.ffDir, platform.ExeName("ffmpeg")))
	touch(t, filepath.Join(r.ffDir, platform.ExeName("ffprobe")))
	r.progs.onRun = func(platform.Command) int { return -1978335189 }
	st := packageStep(r.sys.Speech.FFmpeg, r.manager)
	var out strings.Builder
	p, err := r.svc.installPackage(context.Background(), st, &out)
	if err != nil || p != filepath.Join(r.ffDir, platform.ExeName("ffmpeg")) || !strings.Contains(out.String(), "winget ended with exit code -1978335189, but ") {
		t.Fatalf("installPackage: %q %v\n%s", p, err, out.String())
	}
	if err := r.svc.ffmpegOnPath(st, p, &out); err != nil || filepath.SplitList(os.Getenv("PATH"))[0] != r.ffDir {
		t.Errorf("ffmpegOnPath: %v, PATH %q", err, os.Getenv("PATH"))
	}
	// Without the program, the error stays.
	os.RemoveAll(r.ffDir)
	if _, err := r.svc.installPackage(context.Background(), st, io.Discard); err == nil || !strings.Contains(err.Error(), "failed (exit code -1978335189)") {
		t.Errorf("no program: %v", err)
	}
}

// TestInstallOfASpeechWorkflowNeedsNoComfyUI: Install puts the model file
// into the speech folder on a machine without ComfyUI, and writes no
// extra_model_paths.yaml.
func TestInstallOfASpeechWorkflowNeedsNoComfyUI(t *testing.T) {
	r := newPlanRig(t)
	res, err := r.svc.Install(context.Background(), r.req(), Watch{})
	if errors.Is(err, ErrNoComfyUI) || len(r.dl.calls) != 1 || r.dl.calls[0].Dest != filepath.Join(r.place.Models, "tts", "model.bin") {
		t.Fatalf("Install: %v, downloads %+v", err, r.dl.calls)
	}
	if len(res.Mappings) != 0 || len(snapshot(t, filepath.Dir(r.place.Dir))) == 0 {
		t.Errorf("extra_model_paths.yaml got %v", res.Mappings)
	}
}

// TestFindProgramReadsAPatternAndTheExeName: a folder of a package can be a
// pattern of filepath.Glob, and the file of the program in a folder is Exe.
func TestFindProgramReadsAPatternAndTheExeName(t *testing.T) {
	r := newPlanRig(t)
	if p := r.svc.findProgram(r.sys.Speech.FFmpeg, r.manager); p != "" {
		t.Fatalf("no FFmpeg: %q", p)
	}
	touch(t, filepath.Join(r.ffDir, platform.ExeName("ffmpeg")))
	if p := r.svc.findProgram(r.sys.Speech.FFmpeg, r.manager); p != filepath.Join(r.ffDir, platform.ExeName("ffmpeg")) {
		t.Errorf("FFmpeg in the folder of the pattern: %q", p)
	}
	touch(t, filepath.Join(r.pyDir, "python3.12.exe")) // not the Exe of the package
	if p := r.svc.findProgram(r.sys.Speech.Python, r.manager); p != "" {
		t.Errorf("Python found as %q", p)
	}
	touch(t, filepath.Join(r.pyDir, "python.exe"))
	if p := r.svc.findProgram(r.sys.Speech.Python, r.manager); p != filepath.Join(r.pyDir, "python.exe") {
		t.Errorf("Python: %q", p)
	}
}

// TestEachFileOfViSpeechHasTheURLOfItsOwnRepo: the 19 files of vi-speech come
// from 3 repos. The URL of each file names the repo and the revision of its
// own line of models.json.
func TestEachFileOfViSpeechHasTheURLOfItsOwnRepo(t *testing.T) {
	path := filepath.Join("..", "..", "workflows", "vi-speech", "models.json")
	m, err := catalog.ReadModelList(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Files []struct {
			Repo     string `json:"repo"`
			Revision string `json:"revision"`
			RepoPath string `json:"repo_path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || len(raw.Files) != len(m.Files) {
		t.Fatalf("models.json: %v, %d lines", err, len(raw.Files))
	}
	lines := raw.Files
	repos := map[string]bool{}
	for i, f := range m.Files {
		l := lines[i]
		want := "https://huggingface.co/" + l.Repo + "/resolve/" + l.Revision + "/" + l.RepoPath
		if l.Repo == "" || l.Revision == "" || f.URL != want {
			t.Errorf("file %d: URL %s, want %s", i, f.URL, want)
		}
		repos[l.Repo] = true
	}
	if len(m.Files) != 19 || len(repos) != 3 {
		t.Errorf("%d files from %d repos, want 19 from 3", len(m.Files), len(repos))
	}
}

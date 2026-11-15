// Tests of the package steps and the comfy-cli step of the setup plan, on
// the rig of setup_test.go and a fake of the programs of the machine. The
// packages are the packages of the spec (docs/specs/2026-11-15-release-install.md,
// "The setup plan"). No test runs a real program.
package provision

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/platform"
)

// The arguments of the package managers in the spec.
var (
	wingetArgs = []string{"install", "--id", "7zip.7zip", "-e", "--accept-source-agreements", "--accept-package-agreements"}
	brewArgs   = []string{"install", "python@3.12"}
)

// fakePrograms is a fake of the programs of the machine. LookPath finds only
// the programs of paths. Run appends "run <program> <args>" to the log of
// the rig and calls onRun, which gives the exit code.
type fakePrograms struct {
	paths map[string]string
	log   *[]string
	onRun func(c platform.Command) int
}

func (f *fakePrograms) LookPath(name string) (string, error) {
	if p, ok := f.paths[name]; ok {
		return p, nil
	}
	return "", errors.New("not found")
}

func (f *fakePrograms) Run(_ context.Context, c platform.Command) (int, error) {
	*f.log = append(*f.log, "run "+strings.TrimSpace(c.Path+" "+strings.Join(c.Args, " ")))
	if f.onRun != nil {
		return f.onRun(c), nil
	}
	return 0, nil
}

func (f *fakePrograms) runs() []string {
	var out []string
	for _, l := range *f.log {
		if strings.HasPrefix(l, "run ") {
			out = append(out, l)
		}
	}
	return out
}

// osRig is the setup rig with the system of one OS: Windows (7-Zip with
// winget) or macOS (Python 3.12 with brew, and comfy-cli). manager is true
// when the package manager is on the PATH.
type osRig struct {
	*setupRig
	progs   *fakePrograms
	bin     string // the folder of the package manager
	pkgDir  string // the folder where winget puts 7-Zip (Windows)
	venv    string // the venv of comfy-cli (macOS)
	manager string // the path of the package manager
}

func newOSRig(t *testing.T, goos string, installed, manager bool) *osRig {
	t.Helper()
	tmp := t.TempDir()
	r := &osRig{setupRig: newSetupRig(t, installed, false), bin: filepath.Join(tmp, "bin"),
		pkgDir: filepath.Join(tmp, "Program Files", "7-Zip"), venv: filepath.Join(tmp, "venvs", "comfy-mcp")}
	r.progs = &fakePrograms{paths: map[string]string{}, log: &r.log}
	sys := System{Programs: r.progs, VenvPath: r.venv}
	switch goos {
	case "windows":
		sys.Package = &platform.Package{Name: "7-Zip", Program: "7z", Dirs: []string{r.pkgDir}, Manager: "winget",
			Args: wingetArgs, URL: "https://www.7-zip.org/"}
	case "darwin":
		sys.ComfyCLI = true
		sys.Package = &platform.Package{Name: "Python 3.12", Program: "python3.12", InManagerDir: true, Manager: "brew",
			Args: brewArgs, URL: "https://www.python.org/downloads/"}
	}
	if manager {
		r.manager = filepath.Join(r.bin, platform.ExeName(sys.Package.Manager))
		r.progs.paths[sys.Package.Manager] = r.manager
	}
	r.svc.WithSystem(sys)
	return r
}

func (r *osRig) plan(t *testing.T) Plan {
	t.Helper()
	pl, err := r.svc.Plan(context.Background(), r.req)
	if err != nil {
		t.Fatal(err)
	}
	return pl
}

const files3 = "file diffusion, file text_encoder, file vae"

// TestPlanOfAnEmptyMachineListsTheStepsInTheOrderOfTheSpec: a machine with
// no ComfyUI, no 7-Zip, no comfy, no Python 3.12 and no model file. On
// Windows the plan is 7-Zip with winget, ComfyUI, then the files. On macOS it
// is Python 3.12 with brew, comfy-cli, ComfyUI, then the files. A package step
// holds the command of the spec and its folder. The plan runs no program and
// changes nothing.
func TestPlanOfAnEmptyMachineListsTheStepsInTheOrderOfTheSpec(t *testing.T) {
	for _, c := range []struct{ goos, kinds string }{
		{"windows", "package, backend, " + files3},
		{"darwin", "package, comfy-cli, backend, " + files3},
	} {
		t.Run(c.goos, func(t *testing.T) {
			r := newOSRig(t, c.goos, false, true)
			pl := r.plan(t)
			if got := kinds(pl.Steps); got != c.kinds {
				t.Fatalf("steps: %s\nwant:  %s", got, c.kinds)
			}
			pkg := pl.Steps[0]
			wantArgs, wantName, wantDir := wingetArgs, "7-Zip", r.pkgDir
			if c.goos == "darwin" {
				wantArgs, wantName, wantDir = brewArgs, "Python 3.12", r.bin
				if st := pl.Steps[1]; st.Dir != r.venv || st.Name != "comfy-cli" {
					t.Errorf("comfy-cli step: %+v", st)
				}
			}
			if want := append([]string{r.manager}, wantArgs...); !slices.Equal(pkg.Command, want) || pkg.Name != wantName || pkg.Dir != wantDir {
				t.Errorf("package step: %+v\nwant command %q, name %s, folder %s", pkg, want, wantName, wantDir)
			}
			if pl.ManagerPath != r.manager || pl.ProgramPath != "" || pl.Package == nil {
				t.Errorf("facts: %+v", pl)
			}
			if b := pl.Steps[len(pl.Steps)-4]; b.Kind != StepBackend || b.Dir != r.backend.dir || b.Name != "ComfyUI" {
				t.Errorf("backend step: %+v", b)
			}
			if len(r.log) != 0 || len(r.dl.calls) != 0 || len(r.files.changes) != 0 {
				t.Errorf("the plan changed something: log %q, downloads %d, changes %q", r.log, len(r.dl.calls), r.files.changes)
			}
		})
	}
}

// TestPlanWithoutAPackageManagerHasAStepForAPerson: without winget (or brew)
// the package step is a step for a person with the package and where to get
// it. -yes with -with-packages then refuses before any step and changes
// nothing.
func TestPlanWithoutAPackageManagerHasAStepForAPerson(t *testing.T) {
	for _, c := range []struct{ goos, kinds, url string }{
		{"windows", "person, backend, " + files3, "https://www.7-zip.org/"},
		{"darwin", "person, comfy-cli, backend, " + files3, "https://www.python.org/downloads/"},
	} {
		t.Run(c.goos, func(t *testing.T) {
			r := newOSRig(t, c.goos, false, false)
			pl := r.plan(t)
			if got := kinds(pl.Steps); got != c.kinds {
				t.Fatalf("steps: %s\nwant:  %s", got, c.kinds)
			}
			if st := pl.Steps[0]; st.Package == nil || st.Package.URL != c.url || len(st.Command) != 0 {
				t.Fatalf("the step for a person: %+v", st)
			}
			req := r.req
			req.WithPackages = true
			_, err := r.svc.RunPlan(context.Background(), req, pl, Watch{})
			var person *PersonStepError
			if !errors.As(err, &person) || !strings.Contains(err.Error(), c.url) {
				t.Fatalf("err %v, want a *PersonStepError that names %s", err, c.url)
			}
			if len(r.log) != 0 || len(r.dl.calls) != 0 || len(r.files.changes) != 0 {
				t.Errorf("the refused plan changed something: log %q, downloads %d", r.log, len(r.dl.calls))
			}
		})
	}
}

// TestPlanOfAMachineWithComfyUIHasNoPackageStep: when ComfyUI is installed
// the plan is the plan of Phase 3, also without 7-Zip, comfy and a package
// manager. It reads no fact of the system.
func TestPlanOfAMachineWithComfyUIHasNoPackageStep(t *testing.T) {
	for _, goos := range []string{"windows", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			r := newOSRig(t, goos, true, false)
			pl := r.plan(t)
			if got := kinds(pl.Steps); got != files3 || pl.Package != nil || pl.ComfyCLI {
				t.Fatalf("steps %s, facts %+v", got, pl)
			}
		})
	}
}

// TestPlanOfAMacWithPythonOrComfy: Python 3.12 on the PATH removes the
// package step, and comfy (on the PATH or in the venv) also removes the
// comfy-cli step.
func TestPlanOfAMacWithPythonOrComfy(t *testing.T) {
	r := newOSRig(t, "darwin", false, true)
	r.progs.paths["python3.12"] = "/x/python3.12"
	if got := kinds(r.plan(t).Steps); got != "comfy-cli, backend, "+files3 {
		t.Fatalf("with Python 3.12: %s", got)
	}
	delete(r.progs.paths, "python3.12")
	comfy := platform.VenvBin(r.venv, "comfy")
	must(t, os.MkdirAll(filepath.Dir(comfy), 0o755))
	must(t, os.WriteFile(comfy, nil, 0o755))
	if got := kinds(r.plan(t).Steps); got != "backend, "+files3 {
		t.Fatalf("with comfy in the venv: %s", got)
	}
}

// TestRunPlanWithoutWithPackagesRunsNoProgram: a plan with a package step
// and a request without WithPackages is refused before any step: no
// program runs, ComfyUI is not installed, no file is downloaded.
func TestRunPlanWithoutWithPackagesRunsNoProgram(t *testing.T) {
	r := newOSRig(t, "windows", false, true)
	pl := r.plan(t)
	_, err := r.svc.RunPlan(context.Background(), r.req, pl, Watch{})
	var approval *PackageApprovalError
	if !errors.As(err, &approval) || len(approval.Steps) != 1 || approval.Steps[0].Name != "7-Zip" {
		t.Fatalf("err %v, want a *PackageApprovalError for 7-Zip", err)
	}
	if len(r.log) != 0 || len(r.dl.calls) != 0 {
		t.Errorf("the refused plan ran: %q, %d downloads", r.log, len(r.dl.calls))
	}
	if pl.Refusal(true) != nil {
		t.Errorf("WithPackages must approve the plan: %v", pl.Refusal(true))
	}
}

// TestRunPlanWithPackagesOnWindows: winget runs with the arguments of the
// spec. It puts 7z in the 7-Zip folder, which is not on the PATH, and the
// service finds it there. Then ComfyUI is installed and the files are
// downloaded, in that order. The next plan has 0 steps.
func TestRunPlanWithPackagesOnWindows(t *testing.T) {
	r := newOSRig(t, "windows", false, true)
	seven := filepath.Join(r.pkgDir, platform.ExeName("7z"))
	r.progs.onRun = func(c platform.Command) int {
		must(t, os.MkdirAll(r.pkgDir, 0o755))
		must(t, os.WriteFile(seven, nil, 0o755))
		return 0
	}
	pl := r.plan(t)
	req := r.req
	req.WithPackages = true
	var out strings.Builder
	var started []string
	res, err := r.svc.RunPlan(context.Background(), req, pl, Watch{Out: &out, Step: func(st Step) { started = append(started, st.Kind) }})
	if err != nil || res == nil || !res.OK {
		t.Fatalf("err %v, result %+v\n%s", err, res, out.String())
	}
	want := "run " + r.manager + " " + strings.Join(wingetArgs, " ") + "|install|download flux3-image-klein-int8-convrot.safetensors|download qwen3vl_4b_int8_convrot.safetensors|download flux2-vae.safetensors"
	if got := strings.Join(r.log, "|"); got != want {
		t.Fatalf("order:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(out.String(), "OK: 7-Zip is installed: "+seven+"\n") || strings.Join(started, ",") != "package,backend" {
		t.Errorf("out %q, started %q", out.String(), started)
	}
	if pl := r.plan(t); len(pl.Steps) != 0 {
		t.Errorf("after the run the plan has steps: %s", kinds(pl.Steps))
	}
}

// TestRunPlanWithPackagesOnMac: brew runs with the arguments of the spec and
// puts python3.12 in its own folder. comfy-cli is then installed with that
// Python (the PATH of the process does not have it), then ComfyUI.
func TestRunPlanWithPackagesOnMac(t *testing.T) {
	r := newOSRig(t, "darwin", false, true)
	python := filepath.Join(r.bin, platform.ExeName("python3.12"))
	r.progs.onRun = func(c platform.Command) int {
		switch {
		case c.Path == r.manager:
			must(t, os.MkdirAll(r.bin, 0o755))
			must(t, os.WriteFile(python, nil, 0o755))
		case slices.Contains(c.Args, "venv"):
			py := platform.VenvBin(r.venv, "python")
			must(t, os.MkdirAll(filepath.Dir(py), 0o755))
			must(t, os.WriteFile(py, nil, 0o755))
		case slices.Contains(c.Args, "pip"):
			for _, p := range venvPrograms(r.venv) {
				must(t, os.WriteFile(p, nil, 0o755))
			}
		case strings.Contains(strings.Join(c.Args, " "), "importlib"):
			return 1
		}
		return 0
	}
	pl := r.plan(t)
	req := r.req
	req.WithPackages = true
	if _, err := r.svc.RunPlan(context.Background(), req, pl, Watch{}); err != nil {
		t.Fatal(err)
	}
	runs := r.progs.runs()
	if len(runs) < 3 || runs[0] != "run "+r.manager+" install python@3.12" || runs[1] != "run "+python+" -c import sys" ||
		runs[2] != "run "+python+" -m venv "+r.venv {
		t.Fatalf("runs: %q", runs)
	}
	if i := slices.Index(r.log, "install"); i < len(runs) {
		t.Errorf("ComfyUI must be installed after comfy-cli: %q", r.log)
	}
	if pl := r.plan(t); len(pl.Steps) != 0 {
		t.Errorf("after the run the plan has steps: %s", kinds(pl.Steps))
	}
}

// TestRunPlanStopsWhenThePackageManagerFails: a package manager that exits
// 1 starts no other step, and the error names the command and the URL.
func TestRunPlanStopsWhenThePackageManagerFails(t *testing.T) {
	r := newOSRig(t, "windows", false, true)
	r.progs.onRun = func(platform.Command) int { return 1 }
	req := r.req
	req.WithPackages = true
	_, err := r.svc.RunPlan(context.Background(), req, r.plan(t), Watch{})
	if err == nil || !strings.Contains(err.Error(), "winget") || !strings.Contains(err.Error(), "exit code 1") ||
		!strings.Contains(err.Error(), "https://www.7-zip.org/") {
		t.Fatalf("err %v", err)
	}
	if len(r.log) != 1 || len(r.dl.calls) != 0 {
		t.Errorf("more steps ran after the failure: %q", r.log)
	}
}

// TestRunPlanFailsWhenTheProgramIsNotFoundAfterThePackageStep: a package
// manager that exits 0 but puts no program gives an error that names the
// folder, and ComfyUI is not installed.
func TestRunPlanFailsWhenTheProgramIsNotFoundAfterThePackageStep(t *testing.T) {
	r := newOSRig(t, "windows", false, true)
	req := r.req
	req.WithPackages = true
	_, err := r.svc.RunPlan(context.Background(), req, r.plan(t), Watch{})
	if err == nil || !strings.Contains(err.Error(), r.pkgDir) || slices.Contains(r.log, "install") {
		t.Fatalf("err %v, log %q", err, r.log)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

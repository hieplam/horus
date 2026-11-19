// The setup plan of a speech workflow (spec docs/specs/2026-11-20-text-to-speech.md,
// "Install and uninstall"). A speech workflow needs no ComfyUI. Its steps, in
// order:
//
//  1. a package step for Python 3.12, only when the speech venv must be made
//     and no Python 3.12 is found;
//  2. a package step for FFmpeg, only when ffmpeg or ffprobe is not on the PATH;
//  3. the speech venv: make it with Python 3.12 when its Python is missing,
//     then pip installs the pinned packages (speechsetup.Packages). horus writes
//     the install record into the venv before the pip install, and again with
//     the packages after it;
//  4. each missing model file, into the speech folder (ADR 0032).
//
// The plan reads the facts and changes nothing. RunPlan runs the steps.
package provision

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/speechsetup"
)

// StepSpeechVenv makes the speech venv and installs its Python packages.
const StepSpeechVenv = "speech-venv"

// SpeechFacts are the facts of the plan of a speech workflow.
type SpeechFacts struct {
	Dir        string // the speech folder
	Venv       string // the speech venv
	VenvPython string // the Python of the speech venv
	HasPython  bool   // the Python of the speech venv is there
	VenvReady  bool   // HasPython, and the venv has the pinned packages
	VenvWhy    string // why a venv with a Python is not ready
	// Python 3.12: its package (nil on an OS without one), the program
	// (PythonPath, "" when it is not found) and the package manager.
	Python                       *platform.Package
	PythonPath, PythonManager    string
	FFmpeg                       *platform.Package // nil on an OS without one
	FFmpegPath, FFprobePath      string            // the programs on the PATH, or ""
	FFmpegManager                string
	Packages                     []string // the pinned Python packages of the speech venv
	PythonVersion, PythonProgram string   // "3.12" and the program name of the PATH ("python3.12")
}

// speechPlan is Plan for the speech workflow in dir.
func (s *Service) speechPlan(req Request, dir string) (Plan, error) {
	parts, err := s.speechParts(req.Workflow, dir)
	if err != nil {
		return Plan{Workflow: req.Workflow}, err
	}
	place := speechsetup.Here()
	sf := &SpeechFacts{Dir: place.Dir, Venv: place.Venv, VenvPython: place.Python, Packages: speechsetup.Packages,
		PythonVersion: speechsetup.PythonVersion, PythonProgram: "python" + speechsetup.PythonVersion}
	pl := Plan{Workflow: req.Workflow, Speech: sf}
	for _, sp := range parts {
		switch sp.part.Kind {
		case speechsetup.KindVenv:
			sf.HasPython, sf.VenvReady, sf.VenvWhy = sp.size >= 0, sp.found, sp.why
		case speechsetup.KindModelFile:
			pl.Files = append(pl.Files, FileState{Role: sp.file.Role, File: sp.part.Name, Path: sp.part.Path, Bytes: sp.part.Bytes, Present: sp.found})
		}
	}
	if s.sys.Programs != nil {
		sf.FFmpegPath, sf.FFprobePath = s.lookPath("ffmpeg"), s.lookPath("ffprobe")
		if pkg := s.sys.Speech.Python; pkg != nil {
			sf.Python = pkg
			sf.PythonManager = s.lookPath(pkg.Manager)
			sf.PythonPath = s.findProgram(pkg, sf.PythonManager)
		} else {
			sf.PythonPath = s.lookPath(sf.PythonProgram)
		}
		if pkg := s.sys.Speech.FFmpeg; pkg != nil {
			sf.FFmpeg = pkg
			sf.FFmpegManager = s.lookPath(pkg.Manager)
		}
	}
	pl.Steps = speechSteps(pl)
	return pl, nil
}

// speechSteps gives the steps of the speech facts of pl, in the order of the
// spec. It is a pure function.
func speechSteps(pl Plan) []Step {
	sf := pl.Speech
	var out []Step
	if !sf.HasPython && sf.PythonPath == "" && sf.Python != nil {
		out = append(out, packageStep(sf.Python, sf.PythonManager))
	}
	if (sf.FFmpegPath == "" || sf.FFprobePath == "") && sf.FFmpeg != nil {
		out = append(out, packageStep(sf.FFmpeg, sf.FFmpegManager))
	}
	if !sf.VenvReady {
		out = append(out, Step{Kind: StepSpeechVenv, Name: "speech venv", Dir: sf.Venv, Command: pipCommand(sf.VenvPython)})
	}
	for _, f := range pl.Files {
		if !f.Present {
			out = append(out, Step{Kind: StepFile, Name: f.File, File: f})
		}
	}
	return out
}

// pipCommand is the pip install of the speech venv with the Python python.
func pipCommand(python string) []string {
	return append([]string{python, "-m", "pip", "install", "--disable-pip-version-check"}, speechsetup.Packages...)
}

// ffmpegOnPath is what RunPlan does after the FFmpeg step st, which found
// ffmpeg at program: it finds ffprobe in the same folder. The PATH of this
// process is older than the install, so it puts that folder first on the PATH
// of this process when the PATH does not find both programs. The checks of
// generate_speech and the speech worker of this process then find them.
func (s *Service) ffmpegOnPath(st Step, program string, out io.Writer) error {
	dir := filepath.Dir(program)
	if probe := filepath.Join(dir, platform.ExeName("ffprobe")); !platform.IsFile(probe) {
		return fmt.Errorf("%s ended, but ffprobe is not next to ffmpeg in %s. Install %s by hand from %s, then run the setup again.",
			strings.Join(st.Command, " "), dir, st.Package.Name, st.Package.URL)
	}
	if s.lookPath("ffmpeg") == "" || s.lookPath("ffprobe") == "" {
		platform.AddToPath(dir)
		fmt.Fprintf(out, "The PATH of this horus process now starts with %s. Open a new terminal and restart the agent program, so that they also find ffmpeg.\n", dir)
	}
	return nil
}

// installSpeechVenv is the venv step st: it makes the speech venv at st.Dir
// with python (Python 3.12) when the venv has no Python, and writes the install
// record. Then pip installs the pinned packages, and the record gets them. A
// folder at st.Dir with files and without the install record is a folder that
// horus did not make: the step refuses it and changes nothing.
func (s *Service) installSpeechVenv(ctx context.Context, st Step, python string, out io.Writer) error {
	venv := st.Dir
	py := platform.VenvBin(venv, "python")
	run := func(prog string, args ...string) (int, error) {
		return s.sys.Programs.Run(ctx, platform.Command{Path: prog, Args: args, Stdout: out, Stderr: out})
	}
	if !platform.IsFile(py) {
		if entries, err := os.ReadDir(venv); err == nil && len(entries) > 0 && !platform.HasInstallRecord(venv, platform.RecordSpeechVenv) {
			return fmt.Errorf("%s has files, but it is not a speech venv that horus made (it has no %s). Inspect it, then delete it, and run the setup again.",
				venv, platform.InstallRecordName)
		}
		if python == "" {
			return fmt.Errorf("Python %s was not found, so horus cannot make the speech venv. Install Python %s from https://www.python.org/downloads/, then run the setup again.",
				speechsetup.PythonVersion, speechsetup.PythonVersion)
		}
		if err := os.MkdirAll(filepath.Dir(venv), 0o755); err != nil {
			return err
		}
		fmt.Fprintf(out, "Making the speech venv at %s ...\n", venv)
		if code, err := run(python, "-m", "venv", venv); err != nil || code != 0 {
			return fmt.Errorf("%s -m venv %s failed (%s).", python, venv, exitText(code, err))
		}
		// The install record: horus made this venv, so horus uninstall may
		// delete it. It has no packages until the pip install ends.
		if err := platform.WriteInstallRecord(venv, platform.RecordSpeechVenv); err != nil {
			return fmt.Errorf("The speech venv %s was made, but the install record was not written: %w", venv, err)
		}
	}
	fmt.Fprintf(out, "Installing %s in the speech venv ...\n", strings.Join(speechsetup.Packages, " "))
	cmd := pipCommand(py)
	if code, err := run(cmd[0], cmd[1:]...); err != nil || code != 0 {
		return fmt.Errorf("pip install %s failed (%s). Run the setup again: it runs the pip install again.",
			strings.Join(speechsetup.Packages, " "), exitText(code, err))
	}
	if !platform.IsFile(py) {
		return fmt.Errorf("The pip install ended, but the Python of the speech venv is missing: %s", py)
	}
	if err := platform.WriteInstallRecord(venv, platform.RecordSpeechVenv, speechsetup.Packages...); err != nil {
		return fmt.Errorf("The packages are installed, but the install record of %s was not written: %w", venv, err)
	}
	fmt.Fprintf(out, "OK: the speech venv is ready: %s\n", venv)
	return nil
}

// installSpeech is Install for the speech workflow in dir: it downloads each
// picked model file into the speech folder, then checks each part of the
// speech setup (the size only, because the downloader has verified each
// hash). There is no ComfyUI and no extra_model_paths.yaml. pick nil means the
// files of req.Only.
func (s *Service) installSpeech(ctx context.Context, res *Result, m catalog.ModelList, req Request, w Watch, dir string, pick func(path string) bool) (*Result, error) {
	parts, err := s.speechParts(req.Workflow, dir)
	if err != nil {
		return end(res, err, "")
	}
	var listed []catalog.ListedFile
	dest := map[catalog.ListedFile]string{}
	for _, sp := range parts {
		if sp.part.Kind == speechsetup.KindModelFile && (pick == nil || pick(sp.part.Path)) {
			listed = append(listed, sp.file)
			dest[sp.file] = sp.part.Path
		}
	}
	files := listed
	if pick == nil {
		if files, err = selectFiles(listed, req.Only); err != nil {
			return end(res, err, "")
		}
	}
	if w.Started != nil && len(files) > 0 {
		w.Started(m.License, len(files))
	}
	out := w.Out
	if out == nil {
		out = io.Discard
	}
	failures := 0
	for _, f := range files {
		d := dest[f]
		if w.Downloading != nil {
			w.Downloading(f.Role, d)
		}
		skipped, err := s.dl.Download(ctx, Fetch{URL: f.URL, Dest: d, Bytes: f.Bytes, SHA256: f.SHA256}, out)
		it := Item{Kind: "download", Role: f.Role, File: f.Name, Path: d, Bytes: f.Bytes}
		switch {
		case err != nil:
			failures++
			if w.Failed != nil {
				w.Failed(err)
			}
			it.Status, it.Detail, it.Err = StatusFailed, failDetail(err), err
		case skipped:
			it.Status, it.Detail = StatusPresent, "already present and verified"
		default:
			it.Status, it.Detail = StatusDownloaded, "downloaded and SHA-256 verified"
		}
		res.add(it)
		if ctx.Err() != nil {
			return end(res, ctx.Err(), "")
		}
	}
	if failures > 0 {
		return end(res, failf("%d of %d file(s) are not ready.", failures, len(files)), "")
	}
	n, err := s.checkSpeech(res, req.Workflow, dir, nil, true)
	if err == nil && n > 0 {
		err = failf("The files are installed, but %d part(s) of the speech setup are missing. See the lines above. Fix: horus setup %s", n, req.Workflow)
	}
	return end(res, err, fmt.Sprintf("workflow '%s' is installed and ready", req.Workflow))
}

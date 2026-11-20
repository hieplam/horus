// Tests of horus setup and horus doctor for a speech workflow through the
// wiring of cmd/horus, for Windows and for macOS (the test seam goos picks the
// speech packages of the OS). winget, brew and Python 3.12 are copies of the
// test binary (fakeProgram, setup_packages_test.go). The PATH holds only the
// folder of the fakes, %LOCALAPPDATA% and the home folder are in a temporary
// folder, and a fake https server gives the model file. No test runs a real
// package manager or a real pip, and no test reaches the network.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/cli"
	"github.com/hieplam/horus/internal/download"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
	"github.com/hieplam/horus/internal/speechsetup"
)

var speechModelData = []byte("stand-in speech model")

// speechRig is a machine without Python 3.12, FFmpeg and the speech setup,
// with the stand-in speech workflow.
type speechRig struct {
	root, workflows, bin string
	place                speechsetup.Place
	sm                   seams
	record, pip          string // the records of the package manager and of pip
	manager              string // the path of the fake package manager
	args                 []string
	put                  map[string][]string // the files of each package
	requests             *int
}

func newSpeechSetupRig(t *testing.T, goos string) *speechRig {
	t.Helper()
	root := t.TempDir()
	r := &speechRig{root: root, workflows: filepath.Join(root, "workflows"), bin: filepath.Join(root, "bin"),
		place:  speechsetup.At(filepath.Join(root, "home", "horus", "speech")),
		record: filepath.Join(root, "pkg.record"), pip: filepath.Join(root, "pip.record"), requests: new(int)}
	for _, k := range []string{"HORUS_MODELS_DIR", "HORUS_TEXT_ENCODER_DIR", "HORUS_COMFYUI_DIR", "HORUS_COMFYUI_URL",
		"HORUS_WORKFLOWS_DIR", "HORUS_OUTPUT_DIR", "HORUS_AUTOSTART", "HORUS_GENERATE_TIMEOUT", "HORUS_SPEECH_DIR"} {
		t.Setenv(k, "")
	}
	local := filepath.Join(root, "local")
	t.Setenv("HORUS_ROOT", root)
	t.Setenv("HORUS_HOME", filepath.Join(root, "home", "horus"))
	t.Setenv("HORUS_WORKFLOWS_DIR", r.workflows)
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("USERPROFILE", filepath.Join(root, "home"))
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("ProgramFiles", filepath.Join(root, "Program Files"))
	t.Setenv("PATH", r.bin)
	t.Setenv(envPkgRecord, r.record)
	t.Setenv(envPkgPut, "")
	t.Setenv(envPipRecord, r.pip)
	settings.Reset()
	t.Cleanup(settings.Reset)

	flow := filepath.Join(r.workflows, "stand-in-speech")
	must(t, os.MkdirAll(flow, 0o755))
	manifest, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "stand-in-speech", "manifest.json"))
	must(t, err)
	must(t, os.WriteFile(filepath.Join(flow, "manifest.json"), manifest, 0o644))
	writeJSON(t, filepath.Join(flow, "models.json"), map[string]any{"repo": "org/speech", "revision": "abc", "license": "Apache-2.0", "files": []any{
		map[string]any{"role": "speech", "repo": "org/other", "revision": "def", "repo_path": "onnx/model.bin", "folder": "tts",
			"bytes": len(speechModelData), "sha256": sum(speechModelData)}}})

	name := "winget"
	switch goos {
	case "windows":
		r.args = []string{"install --id Python.Python.3.12 -e --accept-source-agreements --accept-package-agreements",
			"install --id Gyan.FFmpeg -e --accept-source-agreements --accept-package-agreements"}
		links := filepath.Join(local, "Microsoft", "WinGet", "Links")
		r.put = map[string][]string{
			"Python.Python.3.12": {filepath.Join(local, "Programs", "Python", "Python312", "python.exe")},
			"Gyan.FFmpeg":        {filepath.Join(links, platform.ExeName("ffmpeg")), filepath.Join(links, platform.ExeName("ffprobe"))},
		}
	case "darwin":
		name = "brew"
		r.args = []string{"install python@3.12", "install ffmpeg"}
		r.put = map[string][]string{
			"python@3.12":    {filepath.Join(r.bin, platform.ExeName("python3.12"))},
			"install ffmpeg": {filepath.Join(r.bin, platform.ExeName("ffmpeg")), filepath.Join(r.bin, platform.ExeName("ffprobe"))},
		}
	}
	puts, _ := json.Marshal(r.put)
	t.Setenv(envPkgPuts, string(puts))
	exe, err := os.Executable()
	must(t, err)
	r.manager = filepath.Join(r.bin, platform.ExeName(name))
	must(t, copyFile(exe, r.manager))

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		*r.requests++
		if req.URL.Path == "/org/other/resolve/def/onnx/model.bin" {
			w.Write(speechModelData)
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)
	addr := srv.Listener.Addr().String()
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	r.sm = seams{goos: goos, download: download.Options{Transport: tr, Root: root, Sleep: func(time.Duration) {},
		FreeSpace: func(string) (uint64, error) { return 1 << 50, nil }}}
	return r
}

func (r *speechRig) setup(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	code := cli.RunSetup(context.Background(), append([]string{"stand-in-speech"}, args...), &out, &out,
		func(p cli.SetupPlace) cli.SetupSteps { return newSetup(p, r.sm) })
	return code, out.String()
}

func readText(p string) string {
	b, _ := os.ReadFile(p)
	return string(b)
}

// missingParts is the check of generate_speech: the parts of the speech setup
// that are not there, with the programs of the PATH of this process.
func (r *speechRig) missingParts(t *testing.T) []string {
	t.Helper()
	w := &catalog.Workflow{Name: "stand-in-speech", Dir: filepath.Join(r.workflows, "stand-in-speech")}
	parts, err := speechsetup.Parts(w, r.place)
	must(t, err)
	size := func(p string) int64 {
		fi, err := os.Stat(p)
		if err != nil {
			return -1
		}
		return fi.Size()
	}
	found := func(name string) bool { _, err := exec.LookPath(name); return err == nil }
	var names []string
	for _, p := range speechsetup.Missing(parts, size, found) {
		names = append(names, p.Name)
	}
	return names
}

// TestSpeechSetupPrintsTheStepsOfTheSpecAndChangesNothing: horus setup of a
// speech workflow on an empty machine prints Python 3.12, FFmpeg, the speech
// venv and the model file, in that order, and no ComfyUI. Without -yes, and
// with -yes but without -with-packages, nothing changes and nothing runs.
func TestSpeechSetupPrintsTheStepsOfTheSpecAndChangesNothing(t *testing.T) {
	for _, goos := range goosCases {
		t.Run(goos, func(t *testing.T) {
			r := newSpeechSetupRig(t, goos)
			before := snapshot(t, r.root)
			code, out := r.setup(t)
			steps := []string{
				"  1. install the system package Python 3.12 (needs -with-packages): " + r.manager + " " + r.args[0] + "\n",
				"  2. install the system package FFmpeg (needs -with-packages): " + r.manager + " " + r.args[1] + "\n",
				"  3. make the speech venv in " + r.place.Venv + ", then run: " + r.place.Python + " -m pip install --disable-pip-version-check vieneu==3.8.4 faster-whisper==1.2.1\n",
				"  4. download speech model.bin (0.00 GB) to " + filepath.Join(r.place.Models, "tts", "model.bin") + "\n",
			}
			last := -1
			for _, s := range steps {
				i := strings.Index(out, s)
				if i <= last {
					t.Fatalf("the plan lacks %q after the step before:\n%s", s, out)
				}
				last = i
			}
			if code != 0 || strings.Contains(out, "ComfyUI") || !strings.Contains(out, "Plan: 4 step(s):") ||
				!strings.Contains(out, "MISSING   speech venv in "+r.place.Venv) {
				t.Fatalf("exit %d:\n%s", code, out)
			}
			code, out = r.setup(t, "-yes")
			if code != 1 || !strings.Contains(out, "Nothing changed. The plan installs a system package with its package manager:") {
				t.Fatalf("-yes without -with-packages: exit %d:\n%s", code, out)
			}
			if after := snapshot(t, r.root); after != before || *r.requests != 0 {
				t.Errorf("%d requests; files before:\n%s\nafter:\n%s", *r.requests, before, after)
			}
		})
	}
}

// TestSpeechSetupYesWithPackagesMakesTheSpeechSetup: horus setup -yes
// -with-packages runs the package manager for Python 3.12, then for FFmpeg,
// makes the speech venv with the new Python, runs the pip install of the
// pinned packages, writes the install record with them, and downloads the
// model file from the URL of its own repo into the speech folder. Then the
// check of generate_speech finds each part, horus doctor passes the model
// check without a ComfyUI check, and the next plan has 0 steps.
func TestSpeechSetupYesWithPackagesMakesTheSpeechSetup(t *testing.T) {
	for _, goos := range goosCases {
		t.Run(goos, func(t *testing.T) {
			r := newSpeechSetupRig(t, goos)
			code, out := r.setup(t, "-yes", "-with-packages")
			if code != 0 || !strings.HasSuffix(out, "OK: this machine is set up for workflow 'stand-in-speech'.\n") {
				t.Fatalf("exit %d:\n%s", code, out)
			}
			if got, want := readText(r.record), r.args[0]+"\n"+r.args[1]+"\n"; got != want {
				t.Errorf("the package manager got %q, want %q", got, want)
			}
			if got := readText(r.pip); got != "-m pip install --disable-pip-version-check vieneu==3.8.4 faster-whisper==1.2.1\n" {
				t.Errorf("pip got %q", got)
			}
			order := []string{"-- install the system package Python 3.12", "-- install the system package FFmpeg",
				"Making the speech venv at " + r.place.Venv, "OK: the speech venv is ready: ", "-- speech -> "}
			last := -1
			for _, s := range order {
				i := strings.Index(out, s)
				if i <= last {
					t.Fatalf("the output lacks %q after the step before:\n%s", s, out)
				}
				last = i
			}
			if got, ok := platform.InstallRecordPackages(r.place.Venv, platform.RecordSpeechVenv); !ok || strings.Join(got, " ") != "vieneu==3.8.4 faster-whisper==1.2.1" {
				t.Errorf("the install record: %v %v", got, ok)
			}
			if *r.requests != 1 {
				t.Errorf("%d requests, want 1", *r.requests)
			}
			if got := r.missingParts(t); len(got) != 0 {
				t.Errorf("generate_speech finds these parts missing: %v\nPATH %s", got, os.Getenv("PATH"))
			}
			if code, out = r.setup(t); code != 0 || !strings.HasSuffix(out, "Plan: 0 steps. Nothing is missing.\n") {
				t.Errorf("the next plan: exit %d:\n%s", code, out)
			}
			var doc bytes.Buffer
			cli.RunDoctor(context.Background(), "doctor", []string{"-workflow", "stand-in-speech", "-quick"}, &doc, &doc, doctorAt)
			text := doc.String()
			if strings.Contains(text, "ComfyUI folder") || strings.Contains(text, "ComfyUI server") ||
				!strings.Contains(text, "Workflow 'stand-in-speech' is of the kind speech. It needs no ComfyUI") ||
				!strings.Contains(text, "PASS  models for 'stand-in-speech'") {
				t.Errorf("horus doctor:\n%s", text)
			}
		})
	}
}

// TestSpeechSetupThenUninstallLeavesNoFileOfTheSpeechSetup: horus setup
// -yes -with-packages makes the speech setup, and the speech worker file is
// there as after a generate_speech. horus uninstall names the model file,
// the speech venv and the worker file as REMOVE items and changes nothing.
// horus uninstall -yes leaves 0 files in the speech folder and removes the
// folder. Python 3.12 and FFmpeg, the system packages, stay.
func TestSpeechSetupThenUninstallLeavesNoFileOfTheSpeechSetup(t *testing.T) {
	for _, goos := range goosCases {
		t.Run(goos, func(t *testing.T) {
			r := newSpeechSetupRig(t, goos)
			if code, out := r.setup(t, "-yes", "-with-packages"); code != 0 {
				t.Fatalf("setup: exit %d:\n%s", code, out)
			}
			must(t, os.WriteFile(r.place.Script, []byte("# the speech worker"), 0o644))
			program := filepath.Join(r.root, "program", platform.ExeName("horus"))
			must(t, os.MkdirAll(filepath.Dir(program), 0o755))
			must(t, os.WriteFile(program, []byte("program"), 0o755))
			sm := r.sm
			sm.installer = sandboxOS{profile: filepath.Join(r.root, "home", ".zprofile"), program: program}
			place := cli.SetupPlace{ComfyUIDir: filepath.Join(r.root, "no-comfyui"), ServerURL: "http://127.0.0.1:9"}
			args := []string{"-workflows-dir", r.workflows, "-comfyui-dir", place.ComfyUIDir, "-server-url", place.ServerURL}
			at := func(p cli.SetupPlace) cli.Uninstaller { return newUninstall(p, sm) }

			u, err := newUninstall(place, sm).UninstallPlan(context.Background(), provision.UninstallRequest{WorkflowsDir: r.workflows})
			must(t, err)
			planInsideSandbox(t, []string{r.root}, u)
			before := snapshot(t, r.root)
			var buf bytes.Buffer
			code := cli.RunUninstall(context.Background(), args, &buf, &buf, at)
			for _, w := range []string{"REMOVE  " + filepath.Join(r.place.Models, "tts", "model.bin") + "  (model file model.bin of workflow stand-in-speech)",
				"REMOVE  " + r.place.Venv + "  (the speech venv", "REMOVE  " + r.place.Script + "  (the file of the speech worker",
				"A system package (7-Zip, Python, FFmpeg) stays."} {
				if !strings.Contains(buf.String(), w) {
					t.Errorf("the plan lacks %q:\n%s", w, buf.String())
				}
			}
			if after := snapshot(t, r.root); code != 0 || after != before {
				t.Fatalf("uninstall without -yes: exit %d, files changed: %v\n%s", code, after != before, buf.String())
			}

			buf.Reset()
			if code := cli.RunUninstall(context.Background(), append(args, "-yes"), &buf, &buf, at); code != 0 {
				t.Fatalf("uninstall -yes: exit %d:\n%s", code, buf.String())
			}
			if _, err := os.Lstat(r.place.Dir); !os.IsNotExist(err) {
				t.Errorf("the speech folder is still there (%v): %v\n%s", err, files(t, r.place.Dir), buf.String())
			}
			for _, pkg := range r.put {
				for _, p := range pkg {
					if !platform.IsFile(p) {
						t.Errorf("the system package file %s was removed", p)
					}
				}
			}
		})
	}
}

// TestSpeechDoctorNamesEachMissingPart: on the empty machine, horus doctor
// -workflow names the speech venv, ffmpeg, ffprobe and the model file, and
// its fix is horus setup.
func TestSpeechDoctorNamesEachMissingPart(t *testing.T) {
	r := newSpeechSetupRig(t, "windows")
	var doc bytes.Buffer
	code := cli.RunDoctor(context.Background(), "doctor", []string{"-workflow", "stand-in-speech", "-quick"}, &doc, &doc, doctorAt)
	text := doc.String()
	for _, w := range []string{
		"MISSING    speech venv (not at " + r.place.Python + ")",
		"MISSING    ffmpeg (not found)", "MISSING    ffprobe (not found)",
		"MISSING   speech model.bin",
		"FAIL  models for 'stand-in-speech' - see lines above. Fix: horus setup stand-in-speech\n",
	} {
		if !strings.Contains(text, w) {
			t.Errorf("horus doctor lacks %q:\n%s", w, text)
		}
	}
	if code != 1 || strings.Contains(text, "ComfyUI folder") {
		t.Errorf("exit %d:\n%s", code, text)
	}
}

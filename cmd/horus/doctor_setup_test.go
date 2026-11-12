// Tests of horus doctor and horus setup through the wiring of cmd/horus, on
// the sandbox of workflow_test.go: the real model files adapter, the real
// downloader with the fake HTTPS server, and a ComfyUI folder in a
// temporary folder. No test reaches the network.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/backend/comfyclitest"
	"github.com/hieplam/horus/internal/cli"
	"github.com/hieplam/horus/internal/doctor"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

// snapshot is every file under root with its size and time, and every folder
// by its name only (Windows can update the time of a folder late).
func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
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
	must(t, err)
	return b.String()
}

// useComfyURL makes the setting HORUS_COMFYUI_URL the fake ComfyUI of the
// sandbox, so a check that reads the setting never asks a real ComfyUI.
func useComfyURL(t *testing.T, s *sandbox) {
	t.Helper()
	t.Setenv("HORUS_COMFYUI_URL", s.comfyUI.URL)
	settings.Reset()
}

// runDoctor runs horus doctor on the services of doctorAt, with the venv in
// a temporary folder.
func (s *sandbox) runDoctor(t *testing.T, args ...string) (int, string) {
	t.Helper()
	full := append(args, "-comfyui-dir", s.comfy, "-server-url", s.comfyUI.URL, "-models-dir", s.models,
		"-venv-path", filepath.Join(t.TempDir(), "venv"))
	var out bytes.Buffer
	code := cli.RunDoctor(context.Background(), "doctor", full, &out, &out, doctorAt)
	return code, out.String()
}

// TestDoctorThroughTheWiring: the doctor service of doctorAt finds the
// ComfyUI folder through the backend and asks the server. The setup service
// of doctorAt runs the model check of horus workflow check, and the handler
// writes its lines with an indent.
func TestDoctorThroughTheWiring(t *testing.T) {
	s := newSandbox(t)
	useComfyURL(t, s)
	place := cli.DoctorPlace{ComfyUIDir: s.comfy, ServerURL: s.comfyUI.URL, ModelsDir: s.models}
	tmp := t.TempDir()
	o := doctor.Options{ServerURL: s.comfyUI.URL, VenvPath: filepath.Join(tmp, "venv"), RepoRoot: tmp, Home: tmp}
	doc, _ := doctorAt(place)
	rs := doc.Check(context.Background(), o)
	if len(rs) != 7 || rs[0].Level != doctor.Pass || rs[0].Detail != s.comfy || rs[1].Level != doctor.Pass {
		t.Fatalf("results: %+v", rs)
	}

	before := snapshot(t, s.root)
	code, out := s.runDoctor(t, "-workflow", "demo")
	if after := snapshot(t, s.root); after != before {
		t.Fatalf("horus doctor changed a file:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if code != 1 || !strings.Contains(out, "\n      MISSING   vae demo-vae.safetensors") ||
		!strings.Contains(out, "\n      Checking workflow 'demo': 2 model file(s).\n") ||
		!strings.Contains(out, "\nFAIL  models for 'demo' - see lines above. Fix: horus workflow install demo\n") ||
		strings.Contains(out, cli.Mark) {
		t.Fatalf("model check: exit %d\n%s", code, out)
	}

	// After an install the check passes; -quick passes -quick to the check.
	if code, ls, _ := s.run(t, s.seams(1<<50), "install", "demo"); code != 0 {
		t.Fatalf("install: exit %d\n%s", code, strings.Join(ls, "\n"))
	}
	_, out = s.runDoctor(t, "-workflow", "demo", "-quick")
	if !strings.Contains(out, "\n      Checking workflow 'demo': 2 model file(s), size only.\n") ||
		!strings.Contains(out, "\n      OK: workflow 'demo' is ready.\nPASS  models for 'demo' - all files present, size checked") {
		t.Fatalf("model check after the install:\n%s", out)
	}
}

// TestDoctorModelCheckOfAnUnknownWorkflow: a model check that cannot start
// (no such workflow) is a failed check with the FAIL line of horus workflow
// check, indented.
func TestDoctorModelCheckOfAnUnknownWorkflow(t *testing.T) {
	s := newSandbox(t)
	useComfyURL(t, s)
	code, out := s.runDoctor(t, "-workflow", "other", "-quick")
	want := "\n      FAIL: No workflow 'other'. Known: demo\nFAIL  models for 'other' - see lines above. Fix: horus workflow install other\n"
	if code != 1 || !strings.Contains(out, want) {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

// runSetup runs horus setup on the sandbox.
func (s *sandbox) runSetup(t *testing.T, sm seams, args ...string) (int, string) {
	t.Helper()
	full := append(args, "-workflows-dir", s.workflows, "-comfyui-dir", s.comfy, "-models-dir", s.models, "-server-url", s.comfyUI.URL)
	var out bytes.Buffer
	code := cli.RunSetup(context.Background(), full, &out, &out, func(p cli.SetupPlace) cli.SetupSteps { return newSetup(p, sm) })
	return code, out.String()
}

// TestSetupWithoutYesChangesNoFile: without -yes, horus setup prints the
// plan, makes no request to the download server and changes no file, also
// when ComfyUI is missing. With -yes it downloads the missing files, and
// then the plan has 0 steps.
func TestSetupWithoutYesChangesNoFile(t *testing.T) {
	s := newSandbox(t)
	sm := s.seams(1 << 50)
	before := snapshot(t, s.root)
	code, out := s.runSetup(t, sm, "demo")
	if code != 0 || !strings.Contains(out, "Plan: 2 step(s):\n") || !strings.Contains(out, "PRESENT   ComfyUI in "+s.comfy) ||
		!strings.Contains(out, "Nothing changed. Run horus setup demo -yes to run the plan.") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if after := snapshot(t, s.root); after != before || s.requests() != 0 {
		t.Fatalf("%d requests; files before:\n%s\nafter:\n%s", s.requests(), before, after)
	}

	mainPy := filepath.Join(s.comfy, "ComfyUI", "main.py")
	must(t, os.Remove(mainPy))
	before = snapshot(t, s.root)
	code, out = s.runSetup(t, sm, "demo")
	if code != 0 || !strings.Contains(out, "MISSING   ComfyUI in "+s.comfy) || !strings.Contains(out, "Plan: 3 step(s):\n  1. install ComfyUI in "+s.comfy+"\n") {
		t.Fatalf("no ComfyUI: exit %d:\n%s", code, out)
	}
	if after := snapshot(t, s.root); after != before || s.requests() != 0 {
		t.Fatalf("no ComfyUI: %d requests; files before:\n%s\nafter:\n%s", s.requests(), before, after)
	}
	must(t, os.WriteFile(mainPy, []byte("# fake\n"), 0o644))

	code, out = s.runSetup(t, sm, "demo", "-yes")
	vae := filepath.Join(full(t, s.models), "vae", "demo-vae.safetensors")
	if code != 0 || !strings.Contains(out, "-- vae -> "+vae+"\n") || !strings.Contains(out, "OK: this machine is set up for workflow 'demo'.") {
		t.Fatalf("-yes: exit %d:\n%s", code, out)
	}
	if b, err := os.ReadFile(vae); err != nil || !bytes.Equal(b, vaeData) {
		t.Fatalf("the VAE file: %q, %v", b, err)
	}
	if code, out = s.runSetup(t, sm, "demo"); code != 0 || !strings.Contains(out, "Plan: 0 steps. Nothing is missing.") {
		t.Fatalf("after -yes: exit %d:\n%s", code, out)
	}
}

// TestMain makes the test binary also three other programs:
//
//   - HORUS_TEST_AS_HORUS set: horus itself, for a test that runs a command
//     as a process (comfymcp_test.go). It removes the variable first, so a
//     program that horus starts does not run as horus too.
//   - HORUS_FAKE_COMFY_MCP set to an exit code: the fake comfy-mcp
//     (fakeComfyMCP, comfymcp_test.go).
//   - the fake comfy (the program of comfy-cli) of the tests:
//     comfyclitest.Main runs it when the environment asks for it.
func TestMain(m *testing.M) {
	if os.Getenv("HORUS_TEST_AS_HORUS") != "" {
		os.Unsetenv("HORUS_TEST_AS_HORUS")
		os.Exit(run(os.Args[1:]))
	}
	if code := os.Getenv("HORUS_FAKE_COMFY_MCP"); code != "" {
		os.Exit(fakeComfyMCP(code))
	}
	comfyclitest.Main()
	os.Exit(m.Run())
}

// TestSetupYesWithTheFakeComfyThenASecondPlanHasZeroSteps: horus setup demo
// -yes with the comfy-cli backend of macOS (on every OS), on a sandbox
// without ComfyUI, with the fake comfy as the only program of the PATH and
// the fake download server. The plan has 3 steps: install ComfyUI, then the 2
// model files. comfy runs one time, with the install command of the spec,
// and ComfyUI is installed before the first download. Then a second
// horus setup demo -yes prints "Plan: 0 steps" and runs no comfy and no
// download.
func TestSetupYesWithTheFakeComfyThenASecondPlanHasZeroSteps(t *testing.T) {
	s := newSandbox(t)
	must(t, os.RemoveAll(filepath.Join(s.comfy, "ComfyUI")))
	tmp := t.TempDir()
	bin, home := filepath.Join(tmp, "bin"), filepath.Join(tmp, "home")
	must(t, os.MkdirAll(home, 0o755))
	fake := comfyclitest.New(t)
	fake.Put(t, bin)
	t.Setenv("PATH", bin)
	t.Setenv("HOME", home) // the venv of install-comfy-mcp is in the empty home folder
	t.Setenv("USERPROFILE", home)
	sm := s.seams(1 << 50)
	sm.goos = "darwin"
	recorded := fake.Recorded

	code, out := s.runSetup(t, sm, "demo", "-yes")
	vae := filepath.Join(full(t, s.models), "vae", "demo-vae.safetensors")
	te := filepath.Join(full(t, s.models), "text_encoders", "demo-te.safetensors")
	ui := filepath.Join(full(t, s.comfy), "ComfyUI")
	if code != 0 || !strings.Contains(out, "MISSING   ComfyUI in "+s.comfy+"\n") ||
		!strings.Contains(out, "Plan: 3 step(s):\n  1. install ComfyUI in "+s.comfy+"\n  2. download vae ") ||
		!strings.Contains(out, "OK: this machine is set up for workflow 'demo'.") {
		t.Fatalf("-yes: exit %d:\n%s", code, out)
	}
	if got, want := recorded(), "--workspace "+ui+" install --m-series --version 0.44.0\nstdin closed\n"; got != want {
		t.Fatalf("the fake comfy got %q, want %q", got, want)
	}
	installed := strings.Index(out, "OK: ComfyUI v0.44.0 is installed in "+full(t, s.comfy)+"\n")
	first := strings.Index(out, "-- vae -> "+vae+"\n")
	if strings.Index(out, "fake comfy install line\n") < 0 || installed < 0 || first < 0 || installed > first ||
		!strings.Contains(out, "-- text_encoder -> "+te+"\n") {
		t.Fatalf("ComfyUI must be installed first, then the files:\n%s", out)
	}
	if !platform.IsFile(filepath.Join(ui, "main.py")) {
		t.Fatalf("no %s after the install", filepath.Join(ui, "main.py"))
	}
	for p, want := range map[string][]byte{vae: vaeData, te: teData} {
		if b, err := os.ReadFile(p); err != nil || !bytes.Equal(b, want) {
			t.Fatalf("%s: %q, %v", p, b, err)
		}
	}

	requests := s.requests()
	code, out = s.runSetup(t, sm, "demo", "-yes")
	if code != 0 || !strings.Contains(out, "PRESENT   ComfyUI in "+s.comfy+"\n") ||
		!strings.HasSuffix(out, "Plan: 0 steps. Nothing is missing.\n") {
		t.Fatalf("the second setup: exit %d:\n%s", code, out)
	}
	if got := strings.Count(recorded(), "\nstdin "); got != 1 || s.requests() != requests {
		t.Fatalf("the second setup ran comfy (%d runs in all) or downloaded (%d requests, want %d)", got, s.requests(), requests)
	}
}

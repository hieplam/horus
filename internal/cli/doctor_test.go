package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/doctor"
	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// fakeDoctor returns rs and keeps the options and the place. setup is the
// setup service of the model check.
type fakeDoctor struct {
	rs    []doctor.Result
	opts  doctor.Options
	place DoctorPlace
	calls int
	setup *fakeSetup
}

func (f *fakeDoctor) at(p DoctorPlace) (Doctor, Setup) {
	f.place = p
	if f.setup == nil {
		f.setup = &fakeSetup{res: &provision.Result{}}
	}
	return f, f.setup
}
func (f *fakeDoctor) Check(_ context.Context, o doctor.Options) []doctor.Result {
	f.opts = o
	f.calls++
	return f.rs
}

func TestDoctorRejectsAnUnknownFlag(t *testing.T) {
	for _, name := range []string{"doctor", "check-setup"} {
		f := &fakeDoctor{}
		var out, errOut bytes.Buffer
		if code := RunDoctor(context.Background(), name, []string{"-nope"}, &out, &errOut, f.at); code != 2 || f.calls != 0 {
			t.Errorf("%s: exit %d, %d calls", name, code, f.calls)
		}
		if code := RunDoctor(context.Background(), name, []string{"-h"}, &out, &errOut, f.at); code != 0 ||
			!strings.Contains(errOut.String(), "horus "+name) {
			t.Errorf("%s -h: exit %d: %s", name, code, errOut.String())
		}
	}
}

// TestDoctorLinesAndExitCode: the lines of the two names are the lines of
// check-setup: one PASS or FAIL line for each check, then the count.
func TestDoctorLinesAndExitCode(t *testing.T) {
	f := &fakeDoctor{rs: []doctor.Result{{Name: "ComfyUI folder", Level: doctor.Pass, Detail: "/c"},
		{Name: "ComfyUI server", Detail: "no answer at http://h:1. Fix: horus start-comfyui"}}}
	for _, name := range []string{"doctor", "check-setup"} {
		var out, errOut bytes.Buffer
		code := RunDoctor(context.Background(), name, nil, &out, &errOut, f.at)
		want := "PASS  ComfyUI folder - /c\nFAIL  ComfyUI server - no answer at http://h:1. Fix: horus start-comfyui\n1 check(s) failed.\n"
		if code != 1 || out.String() != want {
			t.Errorf("%s: exit %d:\n%s\nwant:\n%s", name, code, out.String(), want)
		}
	}
	f.rs = f.rs[:1]
	var out, errOut bytes.Buffer
	if code := RunDoctor(context.Background(), "doctor", nil, &out, &errOut, f.at); code != 0 ||
		out.String() != "PASS  ComfyUI folder - /c\nAll checks passed.\n" {
		t.Errorf("exit %d:\n%s", code, out.String())
	}
}

// TestDoctorWarnLinesKeepExitCodeZero: a result of level Warn gives a WARN
// line with its detail as the service wrote it, and it does not count as
// failed: with only PASS and WARN lines the exit code is 0 and the last line
// is "All checks passed.". A FAIL next to a WARN counts once.
func TestDoctorWarnLinesKeepExitCodeZero(t *testing.T) {
	warn := doctor.Result{Name: "macOS privacy folder", Level: doctor.Warn,
		Detail: "the repo root /Users/you/Desktop/horus is inside /Users/you/Desktop, a folder that macOS protects."}
	f := &fakeDoctor{rs: []doctor.Result{{Name: "ComfyUI folder", Level: doctor.Pass, Detail: "/c"}, warn}}
	var out, errOut bytes.Buffer
	code := RunDoctor(context.Background(), "doctor", nil, &out, &errOut, f.at)
	want := "PASS  ComfyUI folder - /c\n" +
		"WARN  macOS privacy folder - the repo root /Users/you/Desktop/horus is inside /Users/you/Desktop, a folder that macOS protects.\n" +
		"All checks passed.\n"
	if code != 0 || out.String() != want {
		t.Errorf("exit %d, want 0:\n%s\nwant:\n%s", code, out.String(), want)
	}

	f.rs = append(f.rs, doctor.Result{Name: "ComfyUI server", Level: doctor.Fail, Detail: "no answer"})
	out.Reset()
	code = RunDoctor(context.Background(), "doctor", nil, &out, &errOut, f.at)
	if code != 1 || !strings.HasSuffix(out.String(), "\nFAIL  ComfyUI server - no answer\n1 check(s) failed.\n") {
		t.Errorf("with a FAIL: exit %d, want 1:\n%s", code, out.String())
	}
}

// TestDoctorModelCheckLinesAndExitCode: with -workflow, the handler asks the
// setup service for the check of horus workflow check. It writes the lines
// of that check with an indent, then the PASS or FAIL line of the model
// check, and a failed check gives exit code 1.
func TestDoctorModelCheckLinesAndExitCode(t *testing.T) {
	items := []provision.Item{
		{Status: provision.StatusOK, Kind: "file", Role: "vae", File: "v.st", Path: "/m/vae/v.st", Bytes: 5},
		{Status: provision.StatusMissing, Kind: "file", Role: "unet", File: "u.st", Detail: "not in /m/unet"},
	}
	setup := &fakeSetup{res: &provision.Result{Checked: true, CheckFiles: 2, Workflow: "w", Items: items},
		err: errors.New("1 check(s) failed. Fix: horus workflow install w")}
	f := &fakeDoctor{rs: []doctor.Result{{Name: "ComfyUI folder", Level: doctor.Pass, Detail: "/c"}}, setup: setup}
	var out, errOut bytes.Buffer
	code := RunDoctor(context.Background(), "check-setup", []string{"-workflow", "w"}, &out, &errOut, f.at)
	want := "PASS  ComfyUI folder - /c\n" +
		"      Checking workflow 'w': 2 model file(s).\n" +
		"      OK        vae /m/vae/v.st (5 bytes, SHA-256 verified)\n" +
		"      MISSING   unet u.st (not in /m/unet)\n" +
		"      FAIL: 1 check(s) failed. Fix: horus workflow install w\n" +
		"FAIL  models for 'w' - see lines above. Fix: horus workflow install w\n" +
		"1 check(s) failed.\n"
	if code != 1 || out.String() != want {
		t.Fatalf("exit %d:\n%s\nwant:\n%s", code, out.String(), want)
	}
	wantReq := provision.Request{Workflow: "w", WorkflowsDir: filepath.Join(settings.Root(), "workflows")}
	if setup.req.Workflow != wantReq.Workflow || setup.req.WorkflowsDir != wantReq.WorkflowsDir || setup.req.Quick {
		t.Fatalf("request %+v, want %+v", setup.req, wantReq)
	}

	// The quick check reads sizes only, so its line must not say "verified".
	items = items[:1]
	setup = &fakeSetup{res: &provision.Result{Checked: true, CheckFiles: 1, Quick: true, Workflow: "w", Items: items}}
	f = &fakeDoctor{rs: f.rs, setup: setup}
	out.Reset()
	code = RunDoctor(context.Background(), "doctor", []string{"-workflow", "w", "-quick"}, &out, &errOut, f.at)
	want = "PASS  ComfyUI folder - /c\n" +
		"      Checking workflow 'w': 1 model file(s), size only.\n" +
		"      OK        vae /m/vae/v.st (5 bytes)\n" +
		"      OK: workflow 'w' is ready.\n" +
		"PASS  models for 'w' - all files present, size checked (the SHA-256 is not checked; drop -quick)\n" +
		"All checks passed.\n"
	if code != 0 || out.String() != want || !setup.req.Quick {
		t.Fatalf("quick: exit %d, quick %v:\n%s\nwant:\n%s", code, setup.req.Quick, out.String(), want)
	}

	setup = &fakeSetup{res: &provision.Result{Checked: true, CheckFiles: 1, Workflow: "w", Items: items}}
	f = &fakeDoctor{rs: f.rs, setup: setup}
	out.Reset()
	code = RunDoctor(context.Background(), "doctor", []string{"-workflow", "w"}, &out, &errOut, f.at)
	if code != 0 || !strings.Contains(out.String(), "\n      OK: workflow 'w' is ready.\nPASS  models for 'w' - all files present and verified (SHA-256 checked)\n") {
		t.Fatalf("full: exit %d:\n%s", code, out.String())
	}

	// A check that cannot start (no such workflow) has no head line: only
	// its FAIL line, indented.
	setup = &fakeSetup{res: &provision.Result{Workflow: "x"}, err: errors.New("No workflow 'x'. Known: w")}
	f = &fakeDoctor{rs: f.rs, setup: setup}
	out.Reset()
	code = RunDoctor(context.Background(), "doctor", []string{"-workflow", "x", "-quick"}, &out, &errOut, f.at)
	want = "PASS  ComfyUI folder - /c\n" +
		"      FAIL: No workflow 'x'. Known: w\n" +
		"FAIL  models for 'x' - see lines above. Fix: horus workflow install x\n" +
		"1 check(s) failed.\n"
	if code != 1 || out.String() != want {
		t.Fatalf("no workflow: exit %d:\n%s\nwant:\n%s", code, out.String(), want)
	}
}

// TestDoctorWithoutWorkflowRunsNoModelCheck: without -workflow, the handler
// does not ask the setup service, and writes no model line.
func TestDoctorWithoutWorkflowRunsNoModelCheck(t *testing.T) {
	f := &fakeDoctor{rs: []doctor.Result{{Name: "ComfyUI folder", Level: doctor.Pass, Detail: "/c"}}}
	var out, errOut bytes.Buffer
	if code := RunDoctor(context.Background(), "doctor", []string{"-quick"}, &out, &errOut, f.at); code != 0 ||
		f.setup.req.WorkflowsDir != "" || strings.Contains(out.String(), "models for") {
		t.Fatalf("exit %d, request %+v:\n%s", code, f.setup.req, out.String())
	}
}

// TestDoctorModelCheckIsFullByDefault: the default model check reads the
// SHA-256, -quick reads the sizes only, and the flags pick the place.
func TestDoctorModelCheckIsFullByDefault(t *testing.T) {
	for _, c := range []struct {
		flags     []string
		wantQuick bool
	}{{nil, false}, {[]string{"-quick"}, true}} {
		f := &fakeDoctor{}
		args := append([]string{"-workflow", "it-illustration", "-comfyui-dir", "/c", "-server-url", "http://127.0.0.1:1",
			"-models-dir", "/m", "-text-encoder-dir", "/t", "-venv-path", "/v"}, c.flags...)
		var out, errOut bytes.Buffer
		RunDoctor(context.Background(), "check-setup", args, &out, &errOut, f.at)
		if f.setup.req.Quick != c.wantQuick || f.setup.req.Workflow != "it-illustration" || f.opts.VenvPath != "/v" || f.opts.ServerURL != "http://127.0.0.1:1" {
			t.Errorf("%v: options %+v, request %+v", c.flags, f.opts, f.setup.req)
		}
		if f.place != (DoctorPlace{ComfyUIDir: "/c", ServerURL: "http://127.0.0.1:1", ModelsDir: "/m", TextEncoderDir: "/t"}) {
			t.Errorf("place %+v", f.place)
		}
		if f.opts.ModelsDir != "/m" || f.opts.TextEncoderDir != "/t" || f.opts.OutputDir != settings.OutputDir() {
			t.Errorf("the folders of the TCC check: %+v", f.opts)
		}
	}
	// Without -server-url and HORUS_COMFYUI_URL, the address is the default
	// of check-setup.
	useRoot(t, t.TempDir())
	t.Setenv("HORUS_COMFYUI_URL", "")
	f := &fakeDoctor{}
	var out, errOut bytes.Buffer
	RunDoctor(context.Background(), "doctor", nil, &out, &errOut, f.at)
	if f.place.ServerURL != "http://127.0.0.1:8188" || f.opts.ServerURL != "http://127.0.0.1:8188" || f.setup.req.Workflow != "" {
		t.Errorf("defaults: %+v %+v %+v", f.place, f.opts, f.setup.req)
	}
}

// TestDoctorUsesHorusComfyUIURLWithoutTheFlag (#82): without -server-url,
// horus doctor asks the server of HORUS_COMFYUI_URL, the address that horus
// start-comfyui starts. The line of the check names that address. Without
// the variable, the address is http://127.0.0.1:8188. The flag wins over the
// variable.
func TestDoctorUsesHorusComfyUIURLWithoutTheFlag(t *testing.T) {
	useRoot(t, t.TempDir()) // no .env of a real checkout is read
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	run := func(args ...string) *fakeDoctor {
		t.Helper()
		f := &fakeDoctor{}
		var out, errOut bytes.Buffer
		if code := RunDoctor(context.Background(), "doctor", args, &out, &errOut, f.at); code != 0 {
			t.Fatalf("%v: exit %d:\n%s%s", args, code, out.String(), errOut.String())
		}
		return f
	}

	t.Setenv("HORUS_COMFYUI_URL", srv.URL+"/")
	settings.Reset()
	if f := run(); f.place.ServerURL != srv.URL || f.opts.ServerURL != srv.URL {
		t.Errorf("HORUS_COMFYUI_URL=%s/: the doctor asks %q, its line names %q; want %s", srv.URL, f.place.ServerURL, f.opts.ServerURL, srv.URL)
	}
	if f := run("-server-url", "http://127.0.0.1:1"); f.place.ServerURL != "http://127.0.0.1:1" || f.opts.ServerURL != "http://127.0.0.1:1" {
		t.Errorf("-server-url: the doctor asks %q, its line names %q", f.place.ServerURL, f.opts.ServerURL)
	}

	t.Setenv("HORUS_COMFYUI_URL", "")
	settings.Reset()
	if f := run(); f.place.ServerURL != "http://127.0.0.1:8188" || f.opts.ServerURL != "http://127.0.0.1:8188" {
		t.Errorf("no HORUS_COMFYUI_URL: the doctor asks %q, its line names %q; want http://127.0.0.1:8188", f.place.ServerURL, f.opts.ServerURL)
	}
}

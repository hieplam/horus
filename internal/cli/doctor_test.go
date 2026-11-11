package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/doctor"
)

// fakeDoctor returns rs and keeps the options and the place.
type fakeDoctor struct {
	rs    []doctor.Result
	opts  doctor.Options
	place DoctorPlace
	calls int
}

func (f *fakeDoctor) at(p DoctorPlace) Doctor { f.place = p; return f }
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
// check-setup: the lines of a check, indented, before its PASS or FAIL line,
// then the count.
func TestDoctorLinesAndExitCode(t *testing.T) {
	f := &fakeDoctor{rs: []doctor.Result{{Name: "ComfyUI folder", OK: true, Detail: "/c"},
		{Name: "models for 'w'", Detail: "see lines above.", Lines: []string{"OK        vae", "MISSING   unet"}}}}
	for _, name := range []string{"doctor", "check-setup"} {
		var out, errOut bytes.Buffer
		code := RunDoctor(context.Background(), name, nil, &out, &errOut, f.at)
		want := "PASS  ComfyUI folder - /c\n      OK        vae\n      MISSING   unet\nFAIL  models for 'w' - see lines above.\n1 check(s) failed.\n"
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
		if f.opts.Quick != c.wantQuick || f.opts.Workflow != "it-illustration" || f.opts.VenvPath != "/v" || f.opts.ServerURL != "http://127.0.0.1:1" {
			t.Errorf("%v: options %+v", c.flags, f.opts)
		}
		if f.place != (DoctorPlace{ComfyUIDir: "/c", ServerURL: "http://127.0.0.1:1", ModelsDir: "/m", TextEncoderDir: "/t"}) {
			t.Errorf("place %+v", f.place)
		}
	}
	// The default address is the fixed address of check-setup.
	f := &fakeDoctor{}
	var out, errOut bytes.Buffer
	RunDoctor(context.Background(), "doctor", nil, &out, &errOut, f.at)
	if f.place.ServerURL != "http://127.0.0.1:8188" || f.opts.ServerURL != "http://127.0.0.1:8188" || f.opts.Workflow != "" {
		t.Errorf("defaults: %+v %+v", f.place, f.opts)
	}
}

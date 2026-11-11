package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/platform"
)

// fakeBackend writes lines to out and returns err for each call, and keeps
// the place of the flags and the calls.
type fakeBackend struct {
	place BackendPlace
	calls []string
	lines string
	err   error
}

func (f *fakeBackend) at(p BackendPlace) Backend { f.place = p; return f }
func (f *fakeBackend) call(name string, out io.Writer) error {
	f.calls = append(f.calls, name)
	io.WriteString(out, f.lines)
	return f.err
}
func (f *fakeBackend) Install(_ context.Context, out io.Writer) error { return f.call("Install", out) }
func (f *fakeBackend) Start(_ context.Context, out io.Writer) error   { return f.call("Start", out) }
func (f *fakeBackend) Stop(_ context.Context, out io.Writer) error    { return f.call("Stop", out) }

type comfyCommand struct {
	name string
	run  func(context.Context, []string, io.Writer, io.Writer, func(BackendPlace) Backend) int
	call string
}

var comfyCommands = []comfyCommand{
	{"install-comfyui", InstallComfyUI, "Install"},
	{"start-comfyui", StartComfyUI, "Start"},
	{"stop-comfyui", StopComfyUI, "Stop"},
}

// TestComfyUINotSupportedOffWindows: the backend of another OS answers
// platform.ErrNotSupported. Each command exits 2 with the line of the script
// and prints nothing to stdout.
func TestComfyUINotSupportedOffWindows(t *testing.T) {
	for _, c := range comfyCommands {
		f := &fakeBackend{err: platform.ErrNotSupported}
		var out, errOut bytes.Buffer
		code := c.run(context.Background(), nil, &out, &errOut, f.at)
		if code != 2 || errOut.String() != "horus "+c.name+": not supported on this OS yet\n" || out.Len() != 0 {
			t.Errorf("%s: exit %d, stderr %q, stdout %q", c.name, code, errOut.String(), out.String())
		}
	}
}

// TestComfyUIBadFlagExits2: a wrong flag exits 2 and -h exits 0, and neither
// calls the backend.
func TestComfyUIBadFlagExits2(t *testing.T) {
	for _, c := range comfyCommands {
		f := &fakeBackend{}
		if code := c.run(context.Background(), []string{"-nope"}, io.Discard, io.Discard, f.at); code != 2 {
			t.Errorf("%s -nope: exit %d, want 2", c.name, code)
		}
		if code := c.run(context.Background(), []string{"-h"}, io.Discard, io.Discard, f.at); code != 0 {
			t.Errorf("%s -h: exit %d, want 0", c.name, code)
		}
		if len(f.calls) != 0 {
			t.Errorf("%s: calls %v", c.name, f.calls)
		}
	}
}

// TestComfyUIOutputAndExitCodes: the lines of the backend go to stdout. A
// failed step prints "horus <command>: <error>" to stderr and exits 1.
func TestComfyUIOutputAndExitCodes(t *testing.T) {
	for _, c := range comfyCommands {
		f := &fakeBackend{lines: "line 1\n"}
		var out, errOut bytes.Buffer
		if code := c.run(context.Background(), nil, &out, &errOut, f.at); code != 0 || out.String() != "line 1\n" || errOut.Len() != 0 {
			t.Errorf("%s ok: exit %d, stdout %q, stderr %q", c.name, code, out.String(), errOut.String())
		}
		f.err = errors.New("the step failed")
		out.Reset()
		code := c.run(context.Background(), nil, &out, &errOut, f.at)
		if code != 1 || out.String() != "line 1\n" || errOut.String() != "horus "+c.name+": the step failed\n" {
			t.Errorf("%s failed: exit %d, stdout %q, stderr %q", c.name, code, out.String(), errOut.String())
		}
		if len(f.calls) != 2 || f.calls[0] != c.call {
			t.Errorf("%s: calls %v, want %s", c.name, f.calls, c.call)
		}
	}
}

// TestComfyUIFlagsPickThePlace: the flags go to the backend. Without a flag,
// each field is empty: the backend reads the setting.
func TestComfyUIFlagsPickThePlace(t *testing.T) {
	f := &fakeBackend{}
	InstallComfyUI(context.Background(), []string{"-install-dir", "D1", "-archive-path", "A", "-seven-zip", "Z", "-keep-archive"}, io.Discard, io.Discard, f.at)
	if want := (BackendPlace{Dir: "D1", ArchivePath: "A", SevenZip: "Z", KeepArchive: true}); f.place != want {
		t.Errorf("install: %+v, want %+v", f.place, want)
	}
	StartComfyUI(context.Background(), []string{"-comfyui-dir", "D2", "-server-url", "http://127.0.0.1:8189", "-timeout-seconds", "7"}, io.Discard, io.Discard, f.at)
	if want := (BackendPlace{Dir: "D2", ServerURL: "http://127.0.0.1:8189", Timeout: 7 * time.Second}); f.place != want {
		t.Errorf("start: %+v, want %+v", f.place, want)
	}
	StartComfyUI(context.Background(), nil, io.Discard, io.Discard, f.at)
	if want := (BackendPlace{Timeout: 300 * time.Second}); f.place != want {
		t.Errorf("start without flags: %+v, want %+v", f.place, want)
	}
	StopComfyUI(context.Background(), []string{"-comfyui-dir", "D3", "-server-url", "http://127.0.0.1:8189"}, io.Discard, io.Discard, f.at)
	if want := (BackendPlace{Dir: "D3", ServerURL: "http://127.0.0.1:8189"}); f.place != want {
		t.Errorf("stop: %+v, want %+v", f.place, want)
	}
}

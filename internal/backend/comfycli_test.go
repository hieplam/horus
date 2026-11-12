package backend

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/platform"
)

// fakeComfy is the program comfy of the tests. It holds no code of
// comfy-cli. It appends one line with its arguments to HORUS_FAKE_RECORD,
// and one line that says if its standard input was closed. For "install" it
// writes a line to stdout and makes <workspace>/main.py. The mode "fail"
// exits 3, the mode "nomain" makes no main.py.
func fakeComfy(mode string, args []string) int {
	stdin := "stdin open"
	read := make(chan error, 1)
	go func() { _, err := io.ReadAll(os.Stdin); read <- err }()
	select {
	case err := <-read:
		if err == nil {
			stdin = "stdin closed"
		}
	case <-time.After(2 * time.Second):
	}
	if f, err := os.OpenFile(os.Getenv("HORUS_FAKE_RECORD"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		f.WriteString(strings.Join(args, " ") + "\n" + stdin + "\n")
		f.Close()
	}
	if mode == "fail" {
		return 3
	}
	workspace := ""
	for i, a := range args {
		if a == "--workspace" && i+1 < len(args) {
			workspace = args[i+1]
		}
	}
	os.Stdout.WriteString("fake comfy install line\n")
	if mode != "nomain" && workspace != "" {
		os.MkdirAll(workspace, 0o755)
		os.WriteFile(filepath.Join(workspace, "main.py"), []byte("# fake\n"), 0o644)
	}
	return 0
}

// comfyRig is a comfy-cli backend with an empty home folder, a PATH that
// holds only the folder bin, and a record file for the fake comfy.
type comfyRig struct {
	c      *comfyCLI
	dir    string // the install folder
	bin    string // the only folder of the PATH
	home   string // the home folder: the venv of install-comfy-mcp is in it
	record string
	out    *bytes.Buffer
}

func newComfyRig(t *testing.T) *comfyRig {
	t.Helper()
	tmp := t.TempDir()
	r := &comfyRig{dir: filepath.Join(tmp, "horus"), bin: filepath.Join(tmp, "bin"), home: filepath.Join(tmp, "home"),
		record: filepath.Join(tmp, "comfy.record"), out: &bytes.Buffer{}}
	os.MkdirAll(r.bin, 0o755)
	os.MkdirAll(r.home, 0o755)
	t.Setenv("PATH", r.bin)
	t.Setenv("HOME", r.home)
	t.Setenv("USERPROFILE", r.home)
	t.Setenv("HORUS_FAKE_COMFY", "1")
	t.Setenv("HORUS_FAKE_RECORD", r.record)
	r.c = &comfyCLI{Place: Place{Dir: r.dir, URL: "http://127.0.0.1:8190"}}
	return r
}

// putComfy copies the test binary as the program comfy into folder.
func (r *comfyRig) putComfy(t *testing.T, folder string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(folder, 0o755)
	copyFile(t, exe, filepath.Join(folder, platform.ExeName("comfy")))
}

func (r *comfyRig) recorded() string {
	b, _ := os.ReadFile(r.record)
	return string(b)
}

// TestComfyCLIIsTheBackendOfDarwin: New gives the comfy-cli backend on
// macOS, the portable build on Windows, and the "not supported" backend on
// Linux. The layout of the comfy-cli workspace is the one of the spec. Start
// and Stop answer "not supported" until T2.
func TestComfyCLIIsTheBackendOfDarwin(t *testing.T) {
	dir := t.TempDir()
	place := Place{Dir: dir, URL: "http://127.0.0.1:8190"}
	b, ok := newFor("darwin", place, Archive{}).(*comfyCLI)
	if !ok {
		t.Fatalf("darwin: New gives %T, want the comfy-cli backend", newFor("darwin", place, Archive{}))
	}
	if _, ok := newFor("windows", place, Archive{}).(*portable); !ok {
		t.Error("windows: New does not give the portable backend")
	}
	if _, ok := newFor("linux", place, Archive{}).(unsupported); !ok {
		t.Error("linux: New does not give the backend that answers not supported")
	}
	if takesArchive("darwin") || takesArchive("linux") || !takesArchive("windows") {
		t.Error("only the backend of Windows takes the archive")
	}
	ui := filepath.Join(dir, "ComfyUI")
	l := b.Layout()
	if l.Dir != dir || l.ComfyUI != ui || l.Python != filepath.Join(ui, ".venv", "bin", "python") ||
		!equal(l.Logs, []string{filepath.Join(ui, "user", "comfyui_8190.log")}) {
		t.Errorf("Layout = %+v", l)
	}
	if d, ok := b.Installed(); d != dir || ok {
		t.Errorf("without main.py: Installed = %s %v", d, ok)
	}
	os.MkdirAll(ui, 0o755)
	os.WriteFile(filepath.Join(ui, "main.py"), []byte("#"), 0o644)
	if d, ok := b.Installed(); d != dir || !ok {
		t.Errorf("with main.py: Installed = %s %v", d, ok)
	}
	for name, call := range map[string]func(context.Context, io.Writer) error{"Start": b.Start, "Stop": b.Stop} {
		if err := call(context.Background(), io.Discard); !errors.Is(err, platform.ErrNotSupported) {
			t.Errorf("%s = %v, want not supported until T2", name, err)
		}
	}
}

// TestComfyCLIInstallRunsTheCommandOfTheSpec: Install runs exactly
// `comfy --workspace <dir>/ComfyUI install --m-series --version 0.44.0`, one
// time, with a closed standard input, and gives the lines of comfy to out.
func TestComfyCLIInstallRunsTheCommandOfTheSpec(t *testing.T) {
	r := newComfyRig(t)
	r.putComfy(t, r.bin)
	if err := r.c.Install(context.Background(), r.out); err != nil {
		t.Fatalf("Install: %v\n%s", err, r.out)
	}
	want := "--workspace " + filepath.Join(r.dir, "ComfyUI") + " install --m-series --version 0.44.0\nstdin closed\n"
	if got := r.recorded(); got != want {
		t.Errorf("the fake comfy got %q, want %q", got, want)
	}
	for _, line := range []string{"fake comfy install line", "OK: ComfyUI v0.44.0 is installed in " + r.dir,
		"Next: horus start-comfyui -comfyui-dir '" + r.dir + "'"} {
		if !strings.Contains(r.out.String(), line) {
			t.Errorf("output has no line %q:\n%s", line, r.out)
		}
	}
	if _, ok := r.c.Installed(); !ok {
		t.Error("Installed is false after the install")
	}
}

// TestComfyCLIInstallFindsComfyInTheVenv: with no comfy on the PATH, Install
// runs the comfy of the venv of horus install-comfy-mcp.
func TestComfyCLIInstallFindsComfyInTheVenv(t *testing.T) {
	r := newComfyRig(t)
	venvBin := filepath.Dir(platform.VenvBin(filepath.Join(r.home, ".venvs", "comfy-mcp"), "comfy"))
	r.putComfy(t, venvBin)
	if err := r.c.Install(context.Background(), r.out); err != nil {
		t.Fatalf("Install: %v\n%s", err, r.out)
	}
	if !strings.HasPrefix(r.recorded(), "--workspace "+filepath.Join(r.dir, "ComfyUI")+" install ") {
		t.Errorf("the comfy of the venv did not run: %q", r.recorded())
	}
}

// TestComfyCLIInstallSkipsWhenComfyUIIsInstalled: with ComfyUI/main.py in
// the install folder, Install runs no comfy and says SKIP.
func TestComfyCLIInstallSkipsWhenComfyUIIsInstalled(t *testing.T) {
	r := newComfyRig(t)
	r.putComfy(t, r.bin)
	os.MkdirAll(filepath.Join(r.dir, "ComfyUI"), 0o755)
	os.WriteFile(filepath.Join(r.dir, "ComfyUI", "main.py"), []byte("#"), 0o644)
	if err := r.c.Install(context.Background(), r.out); err != nil {
		t.Fatal(err)
	}
	if got := r.recorded(); got != "" {
		t.Errorf("comfy ran: %q", got)
	}
	if got, want := r.out.String(), "SKIP: ComfyUI is already installed in "+r.dir+"\n"; got != want {
		t.Errorf("output %q, want %q", got, want)
	}
}

// TestComfyCLIInstallNamesBothFixesWhenComfyIsMissing: with comfy neither on
// the PATH nor in the venv, Install fails, names both fixes, and makes no
// folder.
func TestComfyCLIInstallNamesBothFixesWhenComfyIsMissing(t *testing.T) {
	r := newComfyRig(t)
	err := r.c.Install(context.Background(), r.out)
	if err == nil {
		t.Fatal("Install without comfy did not fail")
	}
	for _, fix := range []string{"horus install-comfy-mcp", "uv tool install comfy-cli==1.25.0",
		filepath.Join(r.home, ".venvs", "comfy-mcp")} {
		if !strings.Contains(err.Error(), fix) {
			t.Errorf("the error does not name %q: %v", fix, err)
		}
	}
	if _, e := os.Stat(r.dir); e == nil {
		t.Error("Install made the install folder")
	}
}

// TestComfyCLIInstallFailsWhenComfyFails: an exit code of comfy that is not
// 0, and an install that gives no main.py, are errors.
func TestComfyCLIInstallFailsWhenComfyFails(t *testing.T) {
	r := newComfyRig(t)
	r.putComfy(t, r.bin)
	t.Setenv("HORUS_FAKE_COMFY", "fail")
	if err := r.c.Install(context.Background(), r.out); err == nil || !strings.Contains(err.Error(), "exit code 3") {
		t.Errorf("comfy exit 3: %v", err)
	}
	t.Setenv("HORUS_FAKE_COMFY", "nomain")
	if err := r.c.Install(context.Background(), r.out); err == nil || !strings.Contains(err.Error(), "main.py is not a file") {
		t.Errorf("no main.py: %v", err)
	}
	if strings.Contains(r.out.String(), "OK:") {
		t.Errorf("a failed install says OK:\n%s", r.out)
	}
}

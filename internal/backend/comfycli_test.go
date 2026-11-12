package backend

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/backend/comfyclitest"
	"github.com/hieplam/horus/internal/platform"
)

// comfyRig is a comfy-cli backend with an empty home folder, a PATH that
// holds only the folder bin, and the fake comfy of comfyclitest. The
// address is a free port that the OS picks.
type comfyRig struct {
	c    *comfyCLI
	fake *comfyclitest.Fake
	dir  string // the install folder
	ui   string // the workspace of comfy: <dir>/ComfyUI
	url  string // the address of the server
	port string
	bin  string // the only folder of the PATH
	home string // the home folder: the venv of install-comfy-mcp is in it
	out  *bytes.Buffer
}

func newComfyRig(t *testing.T) *comfyRig {
	t.Helper()
	tmp := t.TempDir()
	r := &comfyRig{dir: filepath.Join(tmp, "horus"), bin: filepath.Join(tmp, "bin"), home: filepath.Join(tmp, "home"),
		url: freeURL(t), out: &bytes.Buffer{}}
	r.ui = filepath.Join(r.dir, "ComfyUI")
	r.port, _ = portOf(r.url)
	os.MkdirAll(r.bin, 0o755)
	os.MkdirAll(r.home, 0o755)
	t.Setenv("PATH", r.bin)
	t.Setenv("HOME", r.home)
	t.Setenv("USERPROFILE", r.home)
	r.fake = comfyclitest.New(t)
	r.c = &comfyCLI{Place: Place{Dir: r.dir, URL: r.url, PollInterval: 20 * time.Millisecond}}
	return r
}

// putComfy copies the fake comfy into folder.
func (r *comfyRig) putComfy(t *testing.T, folder string) string { return r.fake.Put(t, folder) }

func (r *comfyRig) recorded() string { return r.fake.Recorded() }

// install makes ComfyUI/main.py, as comfy install does.
func (r *comfyRig) install(t *testing.T) {
	t.Helper()
	os.MkdirAll(r.ui, 0o755)
	if err := os.WriteFile(filepath.Join(r.ui, "main.py"), []byte("#"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// launchLine is the record of the start: the arguments of the spec, with the
// global flag --json, and a closed standard input.
func (r *comfyRig) launchLine() string {
	return "--workspace " + r.ui + " --json launch --background -- --listen 127.0.0.1 --port " + r.port + "\nstdin closed\n"
}

// stopLine is the record of `comfy --workspace <dir>/ComfyUI stop`.
func (r *comfyRig) stopLine() string { return "--workspace " + r.ui + " stop\nstdin closed\n" }

// personLaunches starts a background ComfyUI with the fake comfy at the
// address of the rig, as a person does in a terminal: no state file of horus.
func (r *comfyRig) personLaunches(t *testing.T, comfy string) {
	t.Helper()
	cmd := exec.Command(comfy, "--workspace", r.ui, "launch", "--background", "--", "--listen", "127.0.0.1", "--port", r.port)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the person's comfy launch: %v\n%s", err, out)
	}
	if !answers(context.Background(), r.url) {
		t.Fatal("the person's ComfyUI does not answer")
	}
}

// TestComfyCLIIsTheBackendOfDarwin: New gives the comfy-cli backend on
// macOS, the portable build on Windows, and the "not supported" backend on
// Linux. The layout of the comfy-cli workspace is the one of the spec. Since
// T2, Start and Stop of the comfy-cli backend no longer answer "not
// supported".
func TestComfyCLIIsTheBackendOfDarwin(t *testing.T) {
	dir := t.TempDir()
	url := freeURL(t)
	port, _ := portOf(url)
	place := Place{Dir: dir, URL: url}
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
	l := b.layout()
	if l.Dir != dir || l.ComfyUI != ui || l.Python != filepath.Join(ui, ".venv", "bin", "python") ||
		!equal(l.Logs, []string{filepath.Join(ui, "user", "comfyui_"+port+".log")}) {
		t.Errorf("Layout = %+v", l)
	}
	if d, ok := b.Installed(); d != dir || ok {
		t.Errorf("without main.py: Installed = %s %v", d, ok)
	}
	var out bytes.Buffer
	if err := b.Start(context.Background(), &out); errors.Is(err, platform.ErrNotSupported) || err == nil {
		t.Errorf("Start of a folder without ComfyUI = %v, want the error of a missing ComfyUI", err)
	}
	if err := b.Stop(context.Background(), &out); err != nil || !strings.Contains(out.String(), "Nothing to stop.") {
		t.Errorf("Stop with nothing to stop = %v, output %q", err, out.String())
	}
	os.MkdirAll(ui, 0o755)
	os.WriteFile(filepath.Join(ui, "main.py"), []byte("#"), 0o644)
	if d, ok := b.Installed(); d != dir || !ok {
		t.Errorf("with main.py: Installed = %s %v", d, ok)
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
	r.install(t)
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
	r.fake.SetMode(t, comfyclitest.Fail)
	if err := r.c.Install(context.Background(), r.out); err == nil || !strings.Contains(err.Error(), "exit code 3") {
		t.Errorf("comfy exit 3: %v", err)
	}
	r.fake.SetMode(t, comfyclitest.NoMain)
	if err := r.c.Install(context.Background(), r.out); err == nil || !strings.Contains(err.Error(), "main.py is not a file") {
		t.Errorf("no main.py: %v", err)
	}
	if strings.Contains(r.out.String(), "OK:") {
		t.Errorf("a failed install says OK:\n%s", r.out)
	}
}

// TestComfyCLIStartRunsLaunchAndTheServerAnswers: Start runs exactly
// `comfy --workspace <dir>/ComfyUI --json launch --background -- --listen
// 127.0.0.1 --port <port>` one time, with a closed standard input. Then the
// server answers, and the state file names the workspace, the process id
// that comfy printed and the address. A second Start runs no comfy.
func TestComfyCLIStartRunsLaunchAndTheServerAnswers(t *testing.T) {
	r := newComfyRig(t)
	r.putComfy(t, r.bin)
	r.install(t)
	if err := r.c.Start(context.Background(), r.out); err != nil {
		t.Fatalf("Start: %v\n%s", err, r.out)
	}
	if got, want := r.recorded(), r.launchLine(); got != want {
		t.Errorf("the fake comfy got %q, want %q", got, want)
	}
	if !answers(context.Background(), r.url) {
		t.Fatal("the server does not answer after Start")
	}
	pid := r.fake.ServerPID()
	st, err := readState(filepath.Join(r.dir, stateName))
	if err != nil || st == nil {
		t.Fatalf("state file: %v %v", st, err)
	}
	if pid <= 0 || st.PID != pid || st.Tree != "comfy-cli:"+r.ui || st.URL != r.url || st.Started == "" {
		t.Errorf("state = %+v, want the process %d of comfy at %s", *st, pid, r.url)
	}
	for _, line := range []string{"Starting ComfyUI with comfy-cli in " + r.ui,
		"Started ComfyUI (process " + itoa(pid) + "). Waiting for " + r.url, "OK: ComfyUI answers at " + r.url} {
		if !strings.Contains(r.out.String(), line) {
			t.Errorf("output has no line %q:\n%s", line, r.out)
		}
	}
	r.out.Reset()
	if err := r.c.Start(context.Background(), r.out); err != nil {
		t.Fatal(err)
	}
	if got := r.recorded(); got != r.launchLine() {
		t.Errorf("a second Start ran comfy: %q", got)
	}
	if got, want := r.out.String(), "ComfyUI already answers at "+r.url+"\n"; got != want {
		t.Errorf("second Start: output %q, want %q", got, want)
	}
}

// TestComfyCLIStartFailsWhenComfyUIIsNotInstalled: Start on a folder without
// ComfyUI/main.py runs no comfy, and its error has the method NotInstalled.
func TestComfyCLIStartFailsWhenComfyUIIsNotInstalled(t *testing.T) {
	r := newComfyRig(t)
	r.putComfy(t, r.bin)
	err := r.c.Start(context.Background(), r.out)
	var ni interface{ NotInstalled() bool }
	if !errors.As(err, &ni) || !ni.NotInstalled() {
		t.Fatalf("Start = %v, want an error with NotInstalled", err)
	}
	if !strings.Contains(err.Error(), r.dir) || !strings.Contains(err.Error(), "horus install-comfyui") {
		t.Errorf("the error names no folder or fix: %v", err)
	}
	if got := r.recorded(); got != "" {
		t.Errorf("comfy ran: %q", got)
	}
	if _, e := os.Stat(filepath.Join(r.dir, stateName)); e == nil {
		t.Error("Start wrote a state file")
	}
}

// TestComfyCLIStartFailsWhenLaunchFails: an exit code of comfy launch that
// is not 0 is an error that names the log, and Start writes no state file.
func TestComfyCLIStartFailsWhenLaunchFails(t *testing.T) {
	r := newComfyRig(t)
	r.putComfy(t, r.bin)
	r.install(t)
	r.fake.SetMode(t, comfyclitest.Fail)
	err := r.c.Start(context.Background(), r.out)
	if err == nil || !strings.Contains(err.Error(), "exit code 3") || !strings.Contains(err.Error(), filepath.Join(r.ui, "user", "comfyui_"+r.port+".log")) {
		t.Errorf("Start = %v", err)
	}
	if _, e := os.Stat(filepath.Join(r.dir, stateName)); e == nil {
		t.Error("a failed Start wrote a state file")
	}
}

// TestComfyCLIStopRunsStopAndTheServerEnds: after Start, Stop runs exactly
// `comfy --workspace <dir>/ComfyUI stop` one time. Then the server does not
// answer, the process of ComfyUI is gone, and the state file is deleted.
func TestComfyCLIStopRunsStopAndTheServerEnds(t *testing.T) {
	r := newComfyRig(t)
	r.putComfy(t, r.bin)
	r.install(t)
	if err := r.c.Start(context.Background(), r.out); err != nil {
		t.Fatalf("Start: %v\n%s", err, r.out)
	}
	pid := r.fake.ServerPID()
	r.out.Reset()
	if err := r.c.Stop(context.Background(), r.out); err != nil {
		t.Fatalf("Stop: %v\n%s", err, r.out)
	}
	if got, want := r.recorded(), r.launchLine()+r.stopLine(); got != want {
		t.Errorf("the fake comfy got %q, want %q", got, want)
	}
	if answers(context.Background(), r.url) {
		t.Error("the server still answers after Stop")
	}
	if platform.Alive(pid) {
		t.Errorf("the process %d of ComfyUI still runs", pid)
	}
	if _, e := os.Stat(filepath.Join(r.dir, stateName)); e == nil {
		t.Error("Stop did not delete the state file")
	}
	if want := "OK: stopped the ComfyUI that horus started at " + r.url + " (process " + itoa(pid) + ").\n"; !strings.HasSuffix(r.out.String(), want) {
		t.Errorf("output %q, want the end %q", r.out.String(), want)
	}
}

// TestComfyCLIStopRefusesAComfyUIThatHorusDidNotStart: a person started the
// background ComfyUI of comfy-cli at the address. Without a state file, and
// with a state file of another address, Stop refuses, runs no comfy stop,
// and the server still answers.
func TestComfyCLIStopRefusesAComfyUIThatHorusDidNotStart(t *testing.T) {
	r := newComfyRig(t)
	comfy := r.putComfy(t, r.bin)
	r.install(t)
	r.personLaunches(t, comfy)
	before := r.recorded()
	err := r.c.Stop(context.Background(), r.out)
	if err == nil || !strings.Contains(err.Error(), "horus did not start the ComfyUI that answers at "+r.url) {
		t.Errorf("Stop without a state file = %v, want the refusal", err)
	}
	other := freeURL(t)
	writeState(filepath.Join(r.dir, stateName), state{Tree: "comfy-cli:" + r.ui, PID: r.fake.ServerPID(), URL: other})
	err = r.c.Stop(context.Background(), r.out)
	if err == nil || !strings.Contains(err.Error(), "horus started the ComfyUI at "+other+", not this one") {
		t.Errorf("Stop with the state file of %s = %v, want the refusal", other, err)
	}
	if got := r.recorded(); got != before || strings.Contains(got, " stop\n") {
		t.Errorf("the fake comfy got a stop: %q", got)
	}
	if !answers(context.Background(), r.url) {
		t.Error("the person's ComfyUI no longer answers")
	}
}

// TestComfyCLIStopWithAStaleStateRunsNoStop: the state file names a process
// that ended (ComfyUI ended, the Mac restarted). Stop runs no comfy stop and
// deletes the state file. With nothing at the address it says that ComfyUI
// was not running any more. When a person's ComfyUI answers there, Stop
// refuses it, and it still answers.
func TestComfyCLIStopWithAStaleStateRunsNoStop(t *testing.T) {
	r := newComfyRig(t)
	comfy := r.putComfy(t, r.bin)
	r.install(t)
	ended := exec.Command(comfy, "--version")
	ended.Env = append(os.Environ(), "HORUS_FAKE_COMFY="+comfyclitest.Fail)
	_ = ended.Run()
	stale := state{Tree: "comfy-cli:" + r.ui, PID: ended.Process.Pid, URL: r.url, Started: "2026-11-12T10:00:00Z"}
	path := filepath.Join(r.dir, stateName)

	writeState(path, stale)
	if err := r.c.Stop(context.Background(), r.out); err != nil {
		t.Fatalf("Stop with a stale state file: %v", err)
	}
	if want := "The ComfyUI that horus started at " + r.url + " (process " + itoa(stale.PID) + ") was not running any more.\n"; r.out.String() != want {
		t.Errorf("output %q, want %q", r.out.String(), want)
	}
	if _, e := os.Stat(path); e == nil {
		t.Error("Stop did not delete the stale state file")
	}

	r.personLaunches(t, comfy)
	writeState(path, stale)
	err := r.c.Stop(context.Background(), r.out)
	if err == nil || !strings.Contains(err.Error(), "is gone, but a server still answers at "+r.url) {
		t.Errorf("Stop with a stale state file and a person's ComfyUI = %v, want the refusal", err)
	}
	if strings.Contains(r.recorded(), " stop\n") {
		t.Errorf("the fake comfy got a stop: %q", r.recorded())
	}
	if !answers(context.Background(), r.url) {
		t.Error("the person's ComfyUI no longer answers")
	}
	if _, e := os.Stat(path); e == nil {
		t.Error("Stop did not delete the stale state file")
	}
}

// TestComfyCLIStartAndStopWithoutAProcessID: when comfy prints no process
// id, the state file holds 0, and Stop runs comfy stop for a server that
// answers.
func TestComfyCLIStartAndStopWithoutAProcessID(t *testing.T) {
	r := newComfyRig(t)
	r.putComfy(t, r.bin)
	r.install(t)
	r.fake.SetMode(t, comfyclitest.NoPID)
	if err := r.c.Start(context.Background(), r.out); err != nil {
		t.Fatalf("Start: %v\n%s", err, r.out)
	}
	if st, _ := readState(filepath.Join(r.dir, stateName)); st == nil || st.PID != 0 {
		t.Fatalf("state = %+v, want the process id 0", st)
	}
	if !strings.Contains(r.out.String(), "Started ComfyUI. Waiting for "+r.url) {
		t.Errorf("output %q", r.out.String())
	}
	if err := r.c.Stop(context.Background(), r.out); err != nil {
		t.Fatalf("Stop: %v\n%s", err, r.out)
	}
	if !strings.HasSuffix(r.recorded(), r.stopLine()) || answers(context.Background(), r.url) {
		t.Errorf("Stop did not stop: %q", r.recorded())
	}
}

// TestReadEnvelopeOfComfyJSON: the process id of a launch that succeeded,
// the message of a launch that failed, and nothing from other text.
func TestReadEnvelopeOfComfyJSON(t *testing.T) {
	cases := []struct {
		in      string
		pid     int
		message string
	}{
		{`{"schema":"envelope/1","type":"envelope","ok":true,"command":"launch","data":{"background":true,"pid":4242},"error":null}` + "\n", 4242, ""},
		{"a line of text\n" + `{"type":"envelope","ok":false,"data":null,"error":{"code":"port_in_use","message":"The 8190 port is already in use."}}`, 0, "The 8190 port is already in use."},
		{`{"type":"envelope","ok":true,"data":{"background":true}}`, 0, ""},
		{"ComfyUI is successfully launched in the background.\n", 0, ""},
		{"", 0, ""},
	}
	for _, c := range cases {
		if pid, msg := readEnvelope([]byte(c.in)); pid != c.pid || msg != c.message {
			t.Errorf("readEnvelope(%q) = %d %q, want %d %q", c.in, pid, msg, c.pid, c.message)
		}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

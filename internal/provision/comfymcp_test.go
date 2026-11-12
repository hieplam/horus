package provision

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/platform"
)

// fakePython is the fake Python launcher and the fake python of the venv. It
// writes every call to HORUS_FAKE_RECORD.
func fakePython(args []string) int {
	if rec := os.Getenv("HORUS_FAKE_RECORD"); rec != "" {
		if f, err := os.OpenFile(rec, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintln(f, strings.Join(args, " "))
			f.Close()
		}
	}
	joined := strings.Join(args, " ")
	exe, _ := os.Executable()
	binDir := filepath.Dir(exe) // Scripts or bin of the venv, when this is the venv python
	switch {
	case strings.Contains(joined, "-m venv"):
		venv := args[len(args)-1]
		b, _ := os.ReadFile(exe)
		py := platform.VenvBin(venv, "python")
		os.MkdirAll(filepath.Dir(py), 0o755)
		os.WriteFile(py, b, 0o755)
	case strings.Contains(joined, "-m pip install"):
		if os.Getenv("HORUS_FAKE_PIPFAIL") != "" {
			return 9
		}
		for _, p := range venvPrograms(filepath.Dir(binDir)) {
			os.WriteFile(p, nil, 0o755)
		}
		os.WriteFile(filepath.Join(binDir, "installed.marker"), nil, 0o644)
	case strings.Contains(joined, "importlib.metadata"):
		if _, err := os.Stat(filepath.Join(binDir, "installed.marker")); err != nil {
			return 1
		}
		fmt.Println(mcpVersion)
		fmt.Println(cliVersion)
	}
	return 0
}

type mcpRig struct {
	opts   ComfyMCPOptions
	record string
	out    *bytes.Buffer
}

func newMCPRig(t *testing.T) *mcpRig {
	t.Helper()
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := &mcpRig{record: filepath.Join(dir, "calls.record"), out: &bytes.Buffer{}}
	t.Setenv("HORUS_FAKE_PY", "1")
	t.Setenv("HORUS_FAKE_RECORD", r.record)
	r.opts = ComfyMCPOptions{VenvPath: filepath.Join(dir, "venvs", "comfy-mcp"), PythonVersion: "3.12",
		Launcher: []string{exe, "-3.12"}, Stdout: r.out, Stderr: r.out}
	return r
}

func (r *mcpRig) install() error {
	return InstallComfyMCP(context.Background(), platform.Programs{}, r.opts)
}

func (r *mcpRig) calls() []string {
	b, _ := os.ReadFile(r.record)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestMakesTheVenvAndInstallsThePinnedVersions(t *testing.T) {
	r := newMCPRig(t)
	if err := r.install(); err != nil {
		t.Fatal(err)
	}
	calls := r.calls()
	want := []string{
		"-3.12 -c import sys",
		"-3.12 -m venv " + r.opts.VenvPath,
		"-c import importlib.metadata as m; print(m.version('comfy-mcp')); print(m.version('comfy-cli'))",
		"-m pip install --disable-pip-version-check comfy-mcp==0.10.0 comfy-cli==1.25.0",
	}
	if len(calls) != len(want) {
		t.Fatalf("calls = %q, want %q", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call %d = %q, want %q", i, calls[i], want[i])
		}
	}
	for _, w := range []string{"Creating the venv at " + r.opts.VenvPath + " ...", "Installing comfy-mcp==0.10.0 and comfy-cli==1.25.0 ...",
		"OK: comfy-mcp is installed in " + r.opts.VenvPath, "Next: .mcp.json starts it as `horus comfy-mcp`. Connect your agent (docs/setup.md, step 5)."} {
		if !strings.Contains(r.out.String(), w) {
			t.Errorf("output lacks %q:\n%s", w, r.out)
		}
	}
	for _, p := range venvPrograms(r.opts.VenvPath) {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("program missing: %v", err)
		}
	}
}

func TestRunAgainChangesNothing(t *testing.T) {
	r := newMCPRig(t)
	if err := r.install(); err != nil {
		t.Fatal(err)
	}
	before := len(r.calls())
	r.out.Reset()
	if err := r.install(); err != nil {
		t.Fatal(err)
	}
	calls := r.calls()[before:]
	if len(calls) != 1 || !strings.Contains(calls[0], "importlib.metadata") {
		t.Errorf("second run calls = %q, want only the version query", calls)
	}
	if !strings.Contains(r.out.String(), "SKIP: comfy-mcp 0.10.0 and comfy-cli 1.25.0 are already installed.") {
		t.Errorf("output:\n%s", r.out)
	}
}

func TestPipFailureFails(t *testing.T) {
	r := newMCPRig(t)
	t.Setenv("HORUS_FAKE_PIPFAIL", "1")
	err := r.install()
	if err == nil || err.Error() != "pip install failed (exit code 9)." {
		t.Fatalf("error = %v", err)
	}
}

func TestFolderThatIsNoVenvFails(t *testing.T) {
	r := newMCPRig(t)
	os.MkdirAll(r.opts.VenvPath, 0o755)
	err := r.install()
	if err == nil || !strings.Contains(err.Error(), "exists but is not a usable venv") {
		t.Fatalf("error = %v", err)
	}
	if b, _ := os.ReadFile(r.record); len(b) != 0 {
		t.Errorf("a program ran: %q", b)
	}
}

// lookPrograms is platform.Programs with a fake LookPath: it finds each
// name of found, and no other name.
type lookPrograms struct {
	platform.Programs
	found  map[string]string
	looked *[]string
}

func (p lookPrograms) LookPath(name string) (string, error) {
	*p.looked = append(*p.looked, name)
	if path, ok := p.found[name]; ok {
		return path, nil
	}
	return "", fmt.Errorf("%s: not found", name)
}

// TestDefaultLauncherNotOnThePathFails: with no Launcher option, the
// service looks for the Python command of this OS through Programs, not on
// the real PATH. Not found, it fails with the text of platform.PythonCommand
// and runs no program.
func TestDefaultLauncherNotOnThePathFails(t *testing.T) {
	r := newMCPRig(t)
	r.opts.Launcher = nil
	var looked []string
	err := InstallComfyMCP(context.Background(), lookPrograms{looked: &looked}, r.opts)
	cmd, notFound := platform.PythonCommand("3.12")
	if err == nil || err.Error() != notFound {
		t.Fatalf("error = %v, want %q", err, notFound)
	}
	if len(looked) != 1 || looked[0] != cmd[0] {
		t.Errorf("looked up %q, want [%s]", looked, cmd[0])
	}
	if b, _ := os.ReadFile(r.record); len(b) != 0 {
		t.Errorf("a program ran: %q", b)
	}
}

// TestDefaultLauncherIsTheProgramThatLookPathFinds: the service runs the
// path that Programs.LookPath gives, with the first arguments of
// platform.PythonCommand.
func TestDefaultLauncherIsTheProgramThatLookPathFinds(t *testing.T) {
	r := newMCPRig(t)
	exe := r.opts.Launcher[0]
	r.opts.Launcher = nil
	cmd, _ := platform.PythonCommand("3.12")
	var looked []string
	if err := InstallComfyMCP(context.Background(), lookPrograms{found: map[string]string{cmd[0]: exe}, looked: &looked}, r.opts); err != nil {
		t.Fatal(err)
	}
	lead := strings.Join(cmd[1:], " ")
	first := strings.TrimSpace(lead + " -c import sys")
	if calls := r.calls(); len(calls) < 2 || calls[0] != first || calls[1] != strings.TrimSpace(lead+" -m venv "+r.opts.VenvPath) {
		t.Errorf("calls = %q, want %q first", calls, first)
	}
}

func TestMissingLauncherFails(t *testing.T) {
	r := newMCPRig(t)
	r.opts.Launcher = []string{filepath.Join(t.TempDir(), "no-such-python")}
	err := r.install()
	if err == nil || !strings.HasPrefix(err.Error(), "Python 3.12 was not found.") {
		t.Fatalf("error = %v", err)
	}
}

// runPrograms is a fake Programs for RunComfyMCP: LookPath finds the names
// of found, and Run records the command and gives code.
type runPrograms struct {
	found map[string]string
	code  int
	ran   *[]platform.Command
}

func (p runPrograms) LookPath(name string) (string, error) {
	if path, ok := p.found[name]; ok {
		return path, nil
	}
	return "", fmt.Errorf("%s: not found", name)
}

func (p runPrograms) Run(_ context.Context, c platform.Command) (int, error) {
	*p.ran = append(*p.ran, c)
	return p.code, nil
}

// touch makes an empty file and its folder.
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestRunComfyMCPFindsTheProgramAndSetsTheEnvironment: the comfy-mcp of the
// venv comes before the one of the PATH. COMFY_BIN names the comfy next to
// the program that runs, only when COMFY_BIN is not set and that comfy
// exists. The streams, the arguments and the exit code pass through.
func TestRunComfyMCPFindsTheProgramAndSetsTheEnvironment(t *testing.T) {
	venv := filepath.Join(t.TempDir(), "venv")
	pathDir := t.TempDir()
	onPath := filepath.Join(pathDir, platform.ExeName("comfy-mcp"))
	touch(t, onPath)
	run := func(set bool) (int, platform.Command, error) {
		var ran []platform.Command
		p := runPrograms{found: map[string]string{"comfy-mcp": onPath}, code: 7, ran: &ran}
		in, out, errOut := strings.NewReader("in"), &bytes.Buffer{}, &bytes.Buffer{}
		code, err := RunComfyMCP(context.Background(), p, ComfyMCPRun{VenvPath: venv, Args: []string{"-x"}, ComfyBinSet: set,
			Stdin: in, Stdout: out, Stderr: errOut})
		if len(ran) != 1 {
			t.Fatalf("%d runs, want 1", len(ran))
		}
		c := ran[0]
		if c.Stdin != in || c.Stdout != out || c.Stderr != errOut || strings.Join(c.Args, " ") != "-x" {
			t.Errorf("the streams or the arguments do not pass through: %+v", c)
		}
		return code, c, err
	}
	env := func(c platform.Command) string { return strings.Join(c.Env, " ") }

	// Only on the PATH, no comfy next to it: no COMFY_BIN.
	code, c, err := run(false)
	if err != nil || code != 7 || c.Path != onPath || env(c) != "PYTHONUTF8=1 PYTHONIOENCODING=utf-8" {
		t.Errorf("PATH: code %d, err %v, path %s, env %q", code, err, c.Path, env(c))
	}
	// On the PATH with comfy next to it.
	comfyOnPath := filepath.Join(pathDir, platform.ExeName("comfy"))
	touch(t, comfyOnPath)
	if _, c, _ := run(false); env(c) != "PYTHONUTF8=1 PYTHONIOENCODING=utf-8 COMFY_BIN="+comfyOnPath {
		t.Errorf("PATH with comfy: env %q", env(c))
	}
	// The venv comes first, with its comfy.
	inVenv, comfyInVenv := platform.VenvBin(venv, "comfy-mcp"), platform.VenvBin(venv, "comfy")
	touch(t, inVenv)
	touch(t, comfyInVenv)
	if _, c, _ := run(false); c.Path != inVenv || env(c) != "PYTHONUTF8=1 PYTHONIOENCODING=utf-8 COMFY_BIN="+comfyInVenv {
		t.Errorf("venv: path %s, env %q", c.Path, env(c))
	}
	// A COMFY_BIN of the caller is not replaced.
	if _, c, _ := run(true); env(c) != "PYTHONUTF8=1 PYTHONIOENCODING=utf-8" {
		t.Errorf("COMFY_BIN set: env %q", env(c))
	}
}

// TestRunComfyMCPNotFoundNamesTheFix: without comfy-mcp in the venv and on
// the PATH, nothing runs and the error names horus install-comfy-mcp.
func TestRunComfyMCPNotFoundNamesTheFix(t *testing.T) {
	var ran []platform.Command
	venv := filepath.Join(t.TempDir(), "venv")
	code, err := RunComfyMCP(context.Background(), runPrograms{ran: &ran}, ComfyMCPRun{VenvPath: venv})
	if code != 1 || err == nil || len(ran) != 0 ||
		err.Error() != "comfy-mcp was not found in "+filepath.Dir(platform.VenvBin(venv, "comfy-mcp"))+" or on the PATH. Fix: horus install-comfy-mcp" {
		t.Fatalf("code %d, err %v, %d runs", code, err, len(ran))
	}
}

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
		"OK: comfy-mcp is installed in " + r.opts.VenvPath, "Next: connect your agent (docs/setup.md, step 5)."} {
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

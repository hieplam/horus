package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

// fakePrograms is a PATH that holds claude when claude is not "", and a run
// that records each command. "claude mcp get" exits 1 (an unknown server),
// every other command exits 0.
type fakePrograms struct {
	claude string
	runs   []platform.Command
}

func (f *fakePrograms) LookPath(name string) (string, error) {
	if name == "claude" && f.claude != "" {
		return f.claude, nil
	}
	return "", errors.New("not found")
}

func (f *fakePrograms) Run(_ context.Context, c platform.Command) (int, error) {
	f.runs = append(f.runs, c)
	if len(c.Args) > 1 && c.Args[1] == "get" {
		return 1, nil
	}
	return 0, nil
}

func (f *fakePrograms) commands() string {
	var s []string
	for _, c := range f.runs {
		s = append(s, c.Path+" "+strings.Join(c.Args, " "))
	}
	return strings.Join(s, "\n")
}

// repoDir makes a folder that looks like the repo (it has workflows/).
func repoDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(d, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

func useRoot(t *testing.T, root string) {
	t.Helper()
	t.Setenv("HORUS_ROOT", root)
	settings.Reset()
	t.Cleanup(settings.Reset)
}

func TestInstallComfyMCPBadFlagExits2(t *testing.T) {
	var out, errOut bytes.Buffer
	f := &fakePrograms{}
	if code := InstallComfyMCP(context.Background(), []string{"-nope"}, &out, &errOut, f); code != 2 || len(f.runs) != 0 {
		t.Errorf("exit %d, %d runs", code, len(f.runs))
	}
}

// TestInstallComfyMCPReportsAFailure: a launcher that is not Python gives
// exit 1 and the error line of the command.
func TestInstallComfyMCPReportsAFailure(t *testing.T) {
	f := &failingRun{}
	var out, errOut bytes.Buffer
	venv := filepath.Join(t.TempDir(), "venv")
	code := InstallComfyMCP(context.Background(), []string{"-venv-path", venv, "-python-version", "3.12"}, &out, &errOut, f)
	if code != 1 || !strings.HasPrefix(errOut.String(), "horus install-comfy-mcp: ") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}

// failingRun is a machine where no program runs.
type failingRun struct{ fakePrograms }

func (*failingRun) Run(context.Context, platform.Command) (int, error) {
	return -1, errors.New("no program")
}

// TestRegisterFindsClaudeAndPassesTheFlags runs the whole command with claude
// on the fake PATH and a path for the horus program.
func TestRegisterFindsClaudeAndPassesTheFlags(t *testing.T) {
	f := &fakePrograms{claude: "/bin/claude"}
	root := repoDir(t)
	var out, errOut bytes.Buffer
	if code := Register(context.Background(), []string{"-horus-path", "/x/horus", "-root", root}, &out, &errOut, f); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	want := "/bin/claude mcp get horus\n/bin/claude mcp add --scope user horus -e HORUS_ROOT=" + root + " -- /x/horus mcp"
	if f.commands() != want || f.runs[0].Dir != os.TempDir() {
		t.Fatalf("commands:\n%s\nwant:\n%s\n(dir %q)", f.commands(), want, f.runs[0].Dir)
	}
	if !strings.HasPrefix(out.String(), "OK: registered for Claude Code in user scope.") {
		t.Fatalf("output %q", out.String())
	}
}

func TestRegisterWithoutClaude(t *testing.T) {
	useRoot(t, repoDir(t))
	f := &fakePrograms{}
	var out, errOut bytes.Buffer
	if code := Register(context.Background(), nil, &out, &errOut, f); code != 1 ||
		!strings.Contains(errOut.String(), "horus register: The claude command was not found.") || len(f.runs) != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}

func TestRegisterWithoutARepoFails(t *testing.T) {
	useRoot(t, t.TempDir()) // a folder without workflows/
	f := &fakePrograms{claude: "/bin/claude"}
	var out, errOut bytes.Buffer
	if code := Register(context.Background(), []string{"-horus-path", "/x/horus"}, &out, &errOut, f); code != 1 ||
		!strings.Contains(errOut.String(), "has no workflows folder") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if len(f.runs) != 0 {
		t.Fatalf("called claude: %s", f.commands())
	}
}

func TestRegisterTakesRootFromTheSettings(t *testing.T) {
	root := repoDir(t)
	useRoot(t, root)
	f := &fakePrograms{claude: "/bin/claude"}
	var out, errOut bytes.Buffer
	if code := Register(context.Background(), []string{"-horus-path", "/x/horus"}, &out, &errOut, f); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(f.commands(), "horus -e HORUS_ROOT="+root+" -- /x/horus mcp") {
		t.Fatalf("commands: %s", f.commands())
	}
	if code := Register(context.Background(), []string{"-nope"}, &out, &errOut, f); code != 2 {
		t.Fatalf("bad flag: exit %d", code)
	}
}

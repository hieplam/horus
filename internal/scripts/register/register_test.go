package register

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/settings"
)

// The test binary is also the fake claude program: when HORUS_FAKE_CLAUDE is
// set, TestMain acts as that program. It appends its folder and its arguments
// to the record file. "mcp get" exits 0 when HORUS_FAKE_KNOWN is set, and 1
// if not. "mcp add" exits with HORUS_FAKE_ADD_EXIT. It runs on every system.
func TestMain(m *testing.M) {
	if rec := os.Getenv("HORUS_FAKE_CLAUDE"); rec != "" {
		wd, _ := os.Getwd()
		f, err := os.OpenFile(rec, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			f.WriteString("dir " + wd + "\nargs " + strings.Join(os.Args[1:], " ") + "\n")
			f.Close()
		}
		if len(os.Args) > 2 && os.Args[2] == "get" {
			if os.Getenv("HORUS_FAKE_KNOWN") != "" {
				os.Exit(0)
			}
			os.Exit(1)
		}
		n, _ := strconv.Atoi(os.Getenv("HORUS_FAKE_ADD_EXIT"))
		os.Exit(n)
	}
	os.Exit(m.Run())
}

// fakeClaude copies the test binary to <dir>/claude and returns the dir and the path.
func fakeClaude(t *testing.T) (dir, path string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	dir = t.TempDir()
	name := "claude"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path = filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

func record(t *testing.T, env map[string]string) (string, func() string) {
	t.Helper()
	rec := filepath.Join(t.TempDir(), "record.txt")
	t.Setenv("HORUS_FAKE_CLAUDE", rec)
	for k, v := range env {
		t.Setenv(k, v)
	}
	return rec, func() string {
		data, _ := os.ReadFile(rec)
		return string(data)
	}
}

// repo makes a folder that looks like the repo (it has workflows/).
func repo(t *testing.T) string {
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

func TestRegisterAddsTheServerInUserScope(t *testing.T) {
	_, claude := fakeClaude(t)
	_, read := record(t, nil)
	root := repo(t)
	ask := t.TempDir()
	var out bytes.Buffer
	err := Register(context.Background(), Options{Claude: claude, HorusPath: "/the/path/horus", Root: root, AskDir: ask, Stdout: &out, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	got := read()
	ask, _ = filepath.EvalSymlinks(ask)
	if !strings.Contains(got, "args mcp get horus\n") {
		t.Fatalf("no question to claude:\n%s", got)
	}
	if !strings.Contains(got, "args mcp add --scope user horus -e HORUS_ROOT="+root+" -- /the/path/horus mcp\n") {
		t.Fatalf("no add command:\n%s", got)
	}
	if !strings.HasPrefix(got, "dir ") || !strings.Contains(strings.SplitN(got, "\n", 2)[0], filepath.Base(ask)) {
		t.Fatalf("the question did not run in the ask folder %s:\n%s", ask, got)
	}
	if !strings.HasPrefix(out.String(), "OK: registered for Claude Code in user scope.") {
		t.Fatalf("output: %q", out.String())
	}
}

func TestRegisterSkipsWhenClaudeKnowsTheServer(t *testing.T) {
	_, claude := fakeClaude(t)
	_, read := record(t, map[string]string{"HORUS_FAKE_KNOWN": "1"})
	var out bytes.Buffer
	if err := Register(context.Background(), Options{Claude: claude, HorusPath: "/p/horus", Root: repo(t), AskDir: t.TempDir(), Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(read(), "mcp add") {
		t.Fatalf("added a known server:\n%s", read())
	}
	if !strings.HasPrefix(out.String(), "SKIP: Claude Code already knows a server named horus") {
		t.Fatalf("output: %q", out.String())
	}
}

func TestRegisterReportsAFailedAdd(t *testing.T) {
	_, claude := fakeClaude(t)
	record(t, map[string]string{"HORUS_FAKE_ADD_EXIT": "3"})
	var out bytes.Buffer
	err := Register(context.Background(), Options{Claude: claude, HorusPath: "/p/horus", Root: repo(t), AskDir: t.TempDir(), Stdout: &out})
	if err == nil || err.Error() != "claude mcp add failed (exit code 3)." {
		t.Fatalf("err: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("printed OK after a failure: %q", out.String())
	}
}

// TestRunFindsClaudeOnThePath runs the whole command with the fake claude as
// the only program on the PATH, and with a path for the horus program.
func TestRunFindsClaudeOnThePath(t *testing.T) {
	dir, _ := fakeClaude(t)
	t.Setenv("PATH", dir)
	_, read := record(t, nil)
	root := repo(t)
	var out, errOut bytes.Buffer
	if code := Run([]string{"-horus-path", "/x/horus", "-root", root}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(read(), "args mcp add --scope user horus -e HORUS_ROOT="+root+" -- /x/horus mcp\n") {
		t.Fatalf("record:\n%s", read())
	}
}

func TestRunWithoutClaude(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var out, errOut bytes.Buffer
	if code := Run(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "The claude command was not found.") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}

func TestRegisterRefusesARootWithoutWorkflows(t *testing.T) {
	_, claude := fakeClaude(t)
	_, read := record(t, nil)
	err := Register(context.Background(), Options{Claude: claude, HorusPath: "/p/horus", Root: t.TempDir(), AskDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "has no workflows folder") {
		t.Fatalf("err: %v", err)
	}
	if read() != "" {
		t.Fatalf("called claude with a bad root: %s", read())
	}
}

func TestRunWithoutARepoFails(t *testing.T) {
	dir, _ := fakeClaude(t)
	t.Setenv("PATH", dir)
	_, read := record(t, nil)
	t.Setenv("HORUS_ROOT", t.TempDir()) // a folder without workflows/
	settings.Reset()
	defer settings.Reset()
	var out, errOut bytes.Buffer
	if code := Run([]string{"-horus-path", "/x/horus"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "has no workflows folder") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if read() != "" {
		t.Fatalf("called claude: %s", read())
	}
}

func TestRunTakesRootFromTheSettings(t *testing.T) {
	dir, _ := fakeClaude(t)
	t.Setenv("PATH", dir)
	_, read := record(t, nil)
	root := repo(t)
	t.Setenv("HORUS_ROOT", root)
	settings.Reset()
	defer settings.Reset()
	var out, errOut bytes.Buffer
	if code := Run([]string{"-horus-path", "/x/horus"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(read(), "horus -e HORUS_ROOT="+root+" -- /x/horus mcp\n") {
		t.Fatalf("record: %s", read())
	}
}

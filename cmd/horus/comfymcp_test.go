// Tests of horus comfy-mcp through the command table: the test binary runs
// as horus, and a copy of it in a venv is the fake comfy-mcp (TestMain,
// doctor_setup_test.go).
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/platform"
)

// fakeComfyMCP holds no code of comfy-mcp. It writes COMFY_BIN, the two
// Python variables and its arguments to stderr, copies its standard input to
// its standard output until the input closes, and exits with code.
func fakeComfyMCP(code string) int {
	for _, name := range []string{"COMFY_BIN", "PYTHONUTF8", "PYTHONIOENCODING"} {
		fmt.Fprintf(os.Stderr, "%s=%s\n", name, os.Getenv(name))
	}
	fmt.Fprintf(os.Stderr, "args=%s\n", strings.Join(os.Args[1:], " "))
	if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
		return 99
	}
	n, err := strconv.Atoi(code)
	if err != nil {
		return 98
	}
	return n
}

// runHorus runs the test binary as horus with args, the extra variables env
// and the standard input in. It gives the exit code and both outputs.
func runHorus(t *testing.T, in []byte, env []string, args ...string) (int, []byte, string) {
	t.Helper()
	exe, err := os.Executable()
	must(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = append(append(os.Environ(), "HORUS_TEST_AS_HORUS=1"), env...)
	var out, errOut bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewReader(in), &out, &errOut
	err = cmd.Run()
	var ee *exec.ExitError
	if err != nil && !errors.As(err, &ee) {
		t.Fatalf("horus %s: %v", strings.Join(args, " "), err)
	}
	return cmd.ProcessState.ExitCode(), out.Bytes(), errOut.String()
}

// TestComfyMCPPassesTheStreamsComfyBinAndTheExitCode: horus comfy-mcp runs
// the comfy-mcp of the venv. Every byte of its standard input reaches
// comfy-mcp, and the standard output of horus holds exactly the bytes of
// comfy-mcp. comfy-mcp ends when the input closes, and horus ends with it.
// COMFY_BIN names the comfy next to comfy-mcp when it is not set, a set
// COMFY_BIN stays, and the exit code is the exit code of comfy-mcp. Without
// comfy-mcp, horus exits 1, writes nothing to its standard output, and names
// the fix.
func TestComfyMCPPassesTheStreamsComfyBinAndTheExitCode(t *testing.T) {
	exe, err := os.Executable()
	must(t, err)
	b, err := os.ReadFile(exe)
	must(t, err)
	venv := filepath.Join(t.TempDir(), "venv")
	mcp, comfy := platform.VenvBin(venv, "comfy-mcp"), platform.VenvBin(venv, "comfy")
	must(t, os.MkdirAll(filepath.Dir(mcp), 0o755))
	must(t, os.WriteFile(mcp, b, 0o755))
	must(t, os.WriteFile(comfy, []byte("x"), 0o755))
	t.Setenv("COMFY_BIN", "")
	t.Setenv("PATH", t.TempDir()) // no other comfy-mcp

	// An MCP request, a line of other bytes, and 256 KiB more.
	in := []byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\"}\n\u00e9\u4e2d\r\n\x00\xff\n")
	in = append(in, bytes.Repeat([]byte("0123456789abcdef"), 1<<14)...)

	code, out, errOut := runHorus(t, in, []string{"HORUS_FAKE_COMFY_MCP=7"}, "comfy-mcp", "-venv-path", venv, "--", "-x", "y")
	if code != 7 {
		t.Errorf("exit %d, want 7 (the exit code of comfy-mcp); stderr:\n%s", code, errOut)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("stdout has %d bytes, want the %d bytes of the input; the first bytes: %q", len(out), len(in), out[:min(len(out), 80)])
	}
	for _, w := range []string{"COMFY_BIN=" + comfy + "\n", "PYTHONUTF8=1\n", "PYTHONIOENCODING=utf-8\n", "args=-x y\n"} {
		if !strings.Contains(errOut, w) {
			t.Errorf("stderr lacks %q:\n%s", w, errOut)
		}
	}

	other := filepath.Join(t.TempDir(), platform.ExeName("comfy"))
	code, out, errOut = runHorus(t, []byte("x"), []string{"HORUS_FAKE_COMFY_MCP=0", "COMFY_BIN=" + other}, "comfy-mcp", "-venv-path", venv)
	if code != 0 || string(out) != "x" || !strings.Contains(errOut, "COMFY_BIN="+other+"\n") {
		t.Errorf("COMFY_BIN set: exit %d, stdout %q, stderr:\n%s", code, out, errOut)
	}

	none := filepath.Join(t.TempDir(), "no-venv")
	code, out, errOut = runHorus(t, in, []string{"HORUS_FAKE_COMFY_MCP=7"}, "comfy-mcp", "-venv-path", none)
	want := "horus comfy-mcp: comfy-mcp was not found in " + filepath.Dir(platform.VenvBin(none, "comfy-mcp")) +
		" or on the PATH. Fix: horus install-comfy-mcp\n"
	if code != 1 || len(out) != 0 || errOut != want {
		t.Errorf("no comfy-mcp: exit %d, stdout %q, stderr %q, want exit 1 and %q", code, out, errOut, want)
	}
}

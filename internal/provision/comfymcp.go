// The comfy MCP server: install comfy-mcp and comfy-cli in their own Python
// virtual environment (horus install-comfy-mcp), and run comfy-mcp for the
// comfy-mcp entry of .mcp.json (horus comfy-mcp).
//
// Two packages with pinned versions: comfy-mcp (the MCP server that an agent
// talks to, AGPL) and comfy-cli (the command-line tool that comfy-mcp calls,
// GPL). horus only installs them with pip and starts them as separate
// programs. It never copies or imports their code.
package provision

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hieplam/horus/internal/platform"
)

// The pinned versions.
const (
	mcpVersion = "0.10.0"
	cliVersion = "1.25.0"
)

// Programs finds and runs the programs of this machine (Python, pip,
// claude). cmd/horus gives the service platform.Programs. A test gives it a
// fake.
type Programs interface {
	LookPath(name string) (string, error)
	// Run runs a program to its end and gives its exit code. The error is
	// not nil only when the program did not run to its end.
	Run(ctx context.Context, c platform.Command) (code int, err error)
}

// ComfyMCPOptions is everything that InstallComfyMCP needs.
type ComfyMCPOptions struct {
	VenvPath      string
	PythonVersion string
	// Launcher is the program that starts Python of the wanted version, with
	// its leading arguments. Empty: py.exe -<version> on Windows,
	// python<version> elsewhere, found with Programs.LookPath.
	Launcher []string
	// Stdout gets the lines of the steps and the output of python and pip.
	// Stderr gets the errors of python and pip. nil drops them.
	Stdout, Stderr io.Writer
}

// venvPrograms are the two programs that the install must leave in the venv.
func venvPrograms(venv string) []string {
	return []string{platform.VenvBin(venv, "comfy-mcp"), platform.VenvBin(venv, "comfy")}
}

// InstallComfyMCP makes the venv when it is missing and installs the pinned
// packages when they are not installed yet. Run again, it changes nothing.
// The lines of the steps go to o.Stdout, between the lines of python and
// pip, so the service writes them there itself.
func InstallComfyMCP(ctx context.Context, programs Programs, o ComfyMCPOptions) error {
	out, errOut := o.Stdout, o.Stderr
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	venv := o.VenvPath
	python := platform.VenvBin(venv, "python")
	run := func(prog string, w, ew io.Writer, args ...string) (int, error) {
		return programs.Run(ctx, platform.Command{Path: prog, Args: args, Stdout: w, Stderr: ew})
	}

	if !platform.IsFile(python) {
		if _, err := os.Stat(venv); err == nil {
			return fmt.Errorf("%s exists but is not a usable venv. Inspect it, then delete it or pass another -venv-path.", venv)
		}
		launcher := o.Launcher
		if len(launcher) == 0 {
			cmd, notFound := platform.PythonCommand(o.PythonVersion)
			p, err := programs.LookPath(cmd[0])
			if err != nil {
				return errors.New(notFound)
			}
			launcher = append([]string{p}, cmd[1:]...)
		}
		lead := launcher[1:len(launcher):len(launcher)]
		if code, err := run(launcher[0], nil, nil, append(lead, "-c", "import sys")...); err != nil || code != 0 {
			return fmt.Errorf("Python %s was not found. Install it from python.org (check 'py launcher').", o.PythonVersion)
		}
		if err := os.MkdirAll(filepath.Dir(venv), 0o755); err != nil {
			return err
		}
		fmt.Fprintf(out, "Creating the venv at %s ...\n", venv)
		if code, err := run(launcher[0], out, errOut, append(lead, "-m", "venv", venv)...); err != nil || code != 0 {
			return fmt.Errorf("Creating the venv failed (%s).", exitText(code, err))
		}
	}

	var versions bytes.Buffer
	code, err := run(python, &versions, nil, "-c", "import importlib.metadata as m; print(m.version('comfy-mcp')); print(m.version('comfy-cli'))")
	lines := strings.Fields(versions.String())
	if err == nil && code == 0 && len(lines) == 2 && lines[0] == mcpVersion && lines[1] == cliVersion {
		fmt.Fprintf(out, "SKIP: comfy-mcp %s and comfy-cli %s are already installed.\n", mcpVersion, cliVersion)
	} else {
		fmt.Fprintf(out, "Installing comfy-mcp==%s and comfy-cli==%s ...\n", mcpVersion, cliVersion)
		code, err := run(python, out, errOut, "-m", "pip", "install", "--disable-pip-version-check",
			"comfy-mcp=="+mcpVersion, "comfy-cli=="+cliVersion)
		if err != nil || code != 0 {
			return fmt.Errorf("pip install failed (%s).", exitText(code, err))
		}
	}

	for _, p := range venvPrograms(venv) {
		if !platform.IsFile(p) {
			return fmt.Errorf("Expected program is missing: %s", p)
		}
	}
	fmt.Fprintf(out, "OK: comfy-mcp is installed in %s\n", venv)
	fmt.Fprintln(out, "Next: .mcp.json starts it as `horus comfy-mcp`. Connect your agent (docs/setup.md, step 5).")
	return nil
}

// ComfyMCPRun is everything RunComfyMCP needs.
type ComfyMCPRun struct {
	// VenvPath is the venv of install-comfy-mcp.
	VenvPath string
	// Args are the arguments for comfy-mcp.
	Args []string
	// ComfyBinSet tells if COMFY_BIN is set (not empty) in the environment of horus.
	// A set COMFY_BIN goes to comfy-mcp as it is.
	ComfyBinSet bool
	// Stdin, Stdout and Stderr are the streams of comfy-mcp. cmd/horus gives
	// the streams of horus, so comfy-mcp reads and writes the pipes of the
	// MCP client itself.
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

// RunComfyMCP is horus comfy-mcp: it runs the comfy MCP server and returns
// its exit code. It finds the program comfy-mcp in the venv first, then on
// the PATH. It adds PYTHONUTF8=1 and PYTHONIOENCODING=utf-8 to the
// environment, and COMFY_BIN when COMFY_BIN is not set and the program comfy
// is next to comfy-mcp. Without COMFY_BIN, comfy-mcp runs the comfy of the
// PATH.
//
// The service writes nothing itself: every byte on r.Stdout comes from
// comfy-mcp. comfy-mcp ends when its standard input closes, so it ends with
// the MCP client. The end of ctx ends it too. The error is not nil when
// comfy-mcp is not found or does not start.
func RunComfyMCP(ctx context.Context, programs Programs, r ComfyMCPRun) (int, error) {
	prog := platform.VenvBin(r.VenvPath, "comfy-mcp")
	if !platform.IsFile(prog) {
		p, err := programs.LookPath("comfy-mcp")
		if err != nil {
			return 1, fmt.Errorf("comfy-mcp was not found in %s or on the PATH. Fix: horus install-comfy-mcp", filepath.Dir(prog))
		}
		prog = p
	}
	env := []string{"PYTHONUTF8=1", "PYTHONIOENCODING=utf-8"}
	if comfy := filepath.Join(filepath.Dir(prog), platform.ExeName("comfy")); !r.ComfyBinSet && platform.IsFile(comfy) {
		env = append(env, "COMFY_BIN="+comfy)
	}
	code, err := programs.Run(ctx, platform.Command{Path: prog, Args: r.Args, Env: env,
		Stdin: r.Stdin, Stdout: r.Stdout, Stderr: r.Stderr})
	if err != nil {
		return 1, fmt.Errorf("cannot run %s: %w", prog, err)
	}
	return code, nil
}

// exitText is the text of a run that failed: its exit code, or the error
// when it did not run to its end.
func exitText(code int, err error) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("exit code %d", code)
}

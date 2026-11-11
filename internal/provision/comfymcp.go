// The comfy MCP server: install comfy-mcp and comfy-cli in their own Python
// virtual environment (horus install-comfy-mcp). It ports
// scripts/install-comfy-mcp.ps1.
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
	MCPVersion = "0.10.0"
	CLIVersion = "1.25.0"
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

	if !isFile(python) {
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
	if err == nil && code == 0 && len(lines) == 2 && lines[0] == MCPVersion && lines[1] == CLIVersion {
		fmt.Fprintf(out, "SKIP: comfy-mcp %s and comfy-cli %s are already installed.\n", MCPVersion, CLIVersion)
	} else {
		fmt.Fprintf(out, "Installing comfy-mcp==%s and comfy-cli==%s ...\n", MCPVersion, CLIVersion)
		code, err := run(python, out, errOut, "-m", "pip", "install", "--disable-pip-version-check",
			"comfy-mcp=="+MCPVersion, "comfy-cli=="+CLIVersion)
		if err != nil || code != 0 {
			return fmt.Errorf("pip install failed (%s).", exitText(code, err))
		}
	}

	for _, p := range venvPrograms(venv) {
		if !isFile(p) {
			return fmt.Errorf("Expected program is missing: %s", p)
		}
	}
	fmt.Fprintf(out, "OK: comfy-mcp is installed in %s\n", venv)
	fmt.Fprintln(out, "Next: connect your agent (docs/setup.md, step 5).")
	return nil
}

// exitText is the text of a run that failed: its exit code, or the error
// when it did not run to its end.
func exitText(code int, err error) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("exit code %d", code)
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

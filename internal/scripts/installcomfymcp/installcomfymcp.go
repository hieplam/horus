// Package installcomfymcp installs the comfy MCP server and comfy-cli in their
// own Python virtual environment. It ports scripts/install-comfy-mcp.ps1.
//
// Two packages with pinned versions: comfy-mcp (the MCP server that an agent
// talks to, AGPL) and comfy-cli (the command-line tool that comfy-mcp calls,
// GPL). horus only installs them with pip and starts them as separate
// programs. It never copies or imports their code.
package installcomfymcp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
)

// The pinned versions.
const (
	MCPVersion = "0.10.0"
	CLIVersion = "1.25.0"
)

// Options is everything that Install needs.
type Options struct {
	VenvPath      string
	PythonVersion string
	// Launcher is the program that starts Python of the wanted version, with its
	// leading arguments. Empty: py.exe -<version> on Windows, python<version> elsewhere.
	Launcher []string
	Stdout   io.Writer
	Stderr   io.Writer
}

// Run is the command horus install-comfy-mcp. It returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("horus install-comfy-mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	venv := fs.String("venv-path", "", "folder of the virtual environment (default: <home>/.venvs/comfy-mcp)")
	pyVer := fs.String("python-version", "3.12", "Python version for the Python Launcher (py.exe), or python<version> elsewhere")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	o := Options{VenvPath: *venv, PythonVersion: *pyVer, Stdout: stdout, Stderr: stderr}
	if o.VenvPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(stderr, "horus install-comfy-mcp:", err)
			return 1
		}
		o.VenvPath = filepath.Join(home, ".venvs", "comfy-mcp")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := Install(ctx, o); err != nil {
		fmt.Fprintln(stderr, "horus install-comfy-mcp:", err)
		return 1
	}
	return 0
}

func venvPython(venv string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venv, "Scripts", "python.exe")
	}
	return filepath.Join(venv, "bin", "python")
}

func venvPrograms(venv string) []string {
	if runtime.GOOS == "windows" {
		return []string{filepath.Join(venv, "Scripts", "comfy-mcp.exe"), filepath.Join(venv, "Scripts", "comfy.exe")}
	}
	return []string{filepath.Join(venv, "bin", "comfy-mcp"), filepath.Join(venv, "bin", "comfy")}
}

// Install makes the venv when it is missing and installs the pinned packages
// when they are not installed yet.
func Install(ctx context.Context, o Options) error {
	out, errOut := o.Stdout, o.Stderr
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	venv := o.VenvPath
	python := venvPython(venv)

	if !isFile(python) {
		if _, err := os.Stat(venv); err == nil {
			return fmt.Errorf("%s exists but is not a usable venv. Inspect it, then delete it or pass another -venv-path.", venv)
		}
		launcher := o.Launcher
		if len(launcher) == 0 {
			if runtime.GOOS == "windows" {
				p, err := exec.LookPath("py.exe")
				if err != nil {
					return fmt.Errorf("The Python Launcher (py.exe) was not found. Install Python %s from python.org.", o.PythonVersion)
				}
				launcher = []string{p, "-" + o.PythonVersion}
			} else {
				p, err := exec.LookPath("python" + o.PythonVersion)
				if err != nil {
					return fmt.Errorf("python%s was not found. Install Python %s.", o.PythonVersion, o.PythonVersion)
				}
				launcher = []string{p}
			}
		}
		if err := exec.CommandContext(ctx, launcher[0], append(append([]string{}, launcher[1:]...), "-c", "import sys")...).Run(); err != nil {
			return fmt.Errorf("Python %s was not found. Install it from python.org (check 'py launcher').", o.PythonVersion)
		}
		if err := os.MkdirAll(filepath.Dir(venv), 0o755); err != nil {
			return err
		}
		fmt.Fprintf(out, "Creating the venv at %s ...\n", venv)
		cmd := exec.CommandContext(ctx, launcher[0], append(append([]string{}, launcher[1:]...), "-m", "venv", venv)...)
		cmd.Stdout, cmd.Stderr = out, errOut
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Creating the venv failed (%s).", exitText(err))
		}
	}

	query := exec.CommandContext(ctx, python, "-c", "import importlib.metadata as m; print(m.version('comfy-mcp')); print(m.version('comfy-cli'))")
	qout, qerr := query.Output()
	lines := strings.Fields(string(qout))
	if qerr == nil && len(lines) == 2 && lines[0] == MCPVersion && lines[1] == CLIVersion {
		fmt.Fprintf(out, "SKIP: comfy-mcp %s and comfy-cli %s are already installed.\n", MCPVersion, CLIVersion)
	} else {
		fmt.Fprintf(out, "Installing comfy-mcp==%s and comfy-cli==%s ...\n", MCPVersion, CLIVersion)
		pip := exec.CommandContext(ctx, python, "-m", "pip", "install", "--disable-pip-version-check",
			"comfy-mcp=="+MCPVersion, "comfy-cli=="+CLIVersion)
		pip.Stdout, pip.Stderr = out, errOut
		if err := pip.Run(); err != nil {
			return fmt.Errorf("pip install failed (%s).", exitText(err))
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

func exitText(err error) string {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return fmt.Sprintf("exit code %d", ee.ExitCode())
	}
	return err.Error()
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

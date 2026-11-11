// horus install-comfy-mcp and horus register: the handlers of the two MCP
// steps of a setup. The work is in the
// service internal/provision, which runs its programs through the
// provision.Programs that cmd/horus gives.
//
// Exit code 0 = done, 1 = the step failed, 2 = a flag that does not parse.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// InstallComfyMCP is horus install-comfy-mcp.
func InstallComfyMCP(ctx context.Context, args []string, out, errOut io.Writer, programs provision.Programs) int {
	var o provision.ComfyMCPOptions
	fs := newFlags("horus install-comfy-mcp", errOut)
	fs.StringVar(&o.VenvPath, "venv-path", "", "folder of the virtual environment (default: <home>/.venvs/comfy-mcp)")
	fs.StringVar(&o.PythonVersion, "python-version", "3.12", "Python version for the Python Launcher (py.exe), or python<version> elsewhere")
	if code, done := parse(fs, args); done {
		return code
	}
	o.Stdout, o.Stderr = out, errOut
	if o.VenvPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(errOut, "horus install-comfy-mcp:", err)
			return 1
		}
		o.VenvPath = filepath.Join(home, ".venvs", "comfy-mcp")
	}
	if err := provision.InstallComfyMCP(ctx, programs, o); err != nil {
		fmt.Fprintln(errOut, "horus install-comfy-mcp:", err)
		return 1
	}
	return 0
}

// Register is horus register.
func Register(ctx context.Context, args []string, out, errOut io.Writer, programs provision.Programs) int {
	fs := newFlags("horus register", errOut)
	rootFlag := fs.String("root", "", "the horus repo folder (with workflows/) to pass as HORUS_ROOT (default: the repo found from the current folder)")
	horus := fs.String("horus-path", "", "path of the horus program to register (default: this program)")
	if code, done := parse(fs, args); done {
		return code
	}
	root := *rootFlag
	if root == "" {
		root = settings.Root()
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	o := provision.RegisterOptions{HorusPath: *horus, Root: root, AskDir: os.TempDir(), Stdout: out, Stderr: errOut}
	if o.HorusPath == "" {
		self, err := os.Executable()
		if err != nil {
			fmt.Fprintln(errOut, "horus register:", err)
			return 1
		}
		o.HorusPath = self
	}
	if err := provision.Register(ctx, programs, o); err != nil {
		fmt.Fprintln(errOut, "horus register:", err)
		return 1
	}
	return 0
}

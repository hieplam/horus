// The comfy-cli backend: ComfyUI in a workspace of comfy-cli, the
// command-line installer of Comfy Org (the program comfy, version 1.25.0).
// horus runs comfy as a separate process only. It never copies or imports
// its code (GPL-3.0).
//
// Install: skip when ComfyUI is already in the install folder. Else find
// comfy (the PATH, then the venv of horus install-comfy-mcp) and run
// `comfy --workspace <dir>/ComfyUI install --m-series --version 0.44.0` with
// a closed standard input. comfy makes the ComfyUI folder, its Python venv
// (.venv) and the PyTorch of Apple Silicon (MPS).
//
// Start and Stop come with task T2 of Phase 3. Until then they answer
// platform.ErrNotSupported.
package backend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

// comfyCLIVersion is the version of comfy-cli that the fix text names.
const comfyCLIVersion = "1.25.0"

// comfyCLI is ComfyUI in the comfy-cli workspace <dir>/ComfyUI.
type comfyCLI struct {
	Place
}

// Layout gives the folders and files of the workspace. comfy writes the log
// of a background ComfyUI to user/comfyui_<port>.log.
func (c *comfyCLI) Layout() Layout {
	dir := c.dir()
	ui := comfyUIDir(dir)
	l := Layout{Dir: dir, ComfyUI: ui, Python: filepath.Join(ui, ".venv", "bin", "python")}
	if port, err := portOf(c.serverURL()); err == nil {
		l.Logs = []string{filepath.Join(ui, "user", "comfyui_"+port+".log")}
	}
	return l
}

// Install installs ComfyUI with comfy. It returns nil after a skip.
func (c *comfyCLI) Install(ctx context.Context, out io.Writer) error {
	dir, err := filepath.Abs(c.dir())
	if err != nil {
		return err
	}
	if isInstalled(dir) {
		saySkip(out, dir)
		return nil
	}
	comfy, err := findComfy()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	pin := strings.TrimPrefix(version, "v")
	fmt.Fprintf(out, "Installing ComfyUI %s with comfy-cli in %s (a few minutes) ...\n", version, dir)
	code, err := platform.Programs{}.Run(ctx, platform.Command{Path: comfy,
		Args:   []string{"--workspace", comfyUIDir(dir), "install", "--m-series", "--version", pin},
		Stdout: out, Stderr: out})
	if err != nil {
		return fmt.Errorf("cannot run %s: %w", comfy, err)
	}
	if code != 0 {
		return fmt.Errorf("comfy install failed with exit code %d. Read the lines of comfy above.", code)
	}
	if !isInstalled(dir) {
		return fmt.Errorf("comfy install ended, but %s is not a file.", filepath.Join(comfyUIDir(dir), "main.py"))
	}
	sayInstalled(out, dir)
	return nil
}

func (*comfyCLI) Start(context.Context, io.Writer) error { return platform.ErrNotSupported }
func (*comfyCLI) Stop(context.Context, io.Writer) error  { return platform.ErrNotSupported }

// findComfy gives the path of the program comfy: the PATH first, then the
// venv of horus install-comfy-mcp.
func findComfy() (string, error) {
	if p, err := (platform.Programs{}).LookPath("comfy"); err == nil {
		return p, nil
	}
	venv := platform.VenvBin(settings.ComfyMCPVenv(), "comfy")
	if platform.IsFile(venv) {
		return venv, nil
	}
	return "", errors.New("comfy-cli (the program comfy) was not found on the PATH or in " + filepath.Dir(venv) +
		". Fix: horus install-comfy-mcp, or uv tool install comfy-cli==" + comfyCLIVersion)
}

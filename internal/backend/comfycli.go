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
// Start: nothing when the server answers. Else run
// `comfy --workspace <dir>/ComfyUI --json launch --background -- --listen
// 127.0.0.1 --port <port>`. comfy starts ComfyUI as a background process and
// ends when ComfyUI reports its start. With --json, comfy prints an envelope
// on its standard output with the process id of the background process that
// runs ComfyUI. That process ends when ComfyUI ends. Start writes the state
// file (started.go) with that id, then waits until the server answers.
//
// Stop: comfy-cli keeps one background ComfyUI for each user, not for each
// workspace, and `comfy stop` stops that one, also when a person started it.
// So Stop runs `comfy --workspace <dir>/ComfyUI stop` only when the state file
// names the same address and its process still runs. A state file whose
// process is gone (ComfyUI ended, the Mac restarted) runs no stop: Stop
// deletes it and says that ComfyUI was not running any more.
package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
		l.Logs = []string{comfyLog(ui, port)}
	}
	return l
}

// comfyLog is the log file of a background ComfyUI of the workspace ui on
// the port.
func comfyLog(ui, port string) string { return filepath.Join(ui, "user", "comfyui_"+port+".log") }

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

// launchArgs are the arguments of comfy for the start of the workspace ui on
// the port. The global flag --json makes comfy print the process id.
func launchArgs(ui, port string) []string {
	return []string{"--workspace", ui, "--json", "launch", "--background", "--", "--listen", "127.0.0.1", "--port", port}
}

// stopArgs are the arguments of comfy for the stop of the background ComfyUI.
func stopArgs(ui string) []string { return []string{"--workspace", ui, "stop"} }

// Start starts ComfyUI with comfy when its server does not answer, then
// waits for it.
func (c *comfyCLI) Start(ctx context.Context, out io.Writer) error {
	serverURL := c.serverURL()
	if alreadyUp(ctx, out, serverURL) {
		return nil
	}
	dir, err := filepath.Abs(c.dir())
	if err != nil {
		return err
	}
	if !isInstalled(dir) {
		return &notInstalledError{fmt.Sprintf("ComfyUI was not found in %s. Run horus install-comfyui first, or pass -comfyui-dir.", dir)}
	}
	port, err := portOf(serverURL)
	if err != nil {
		return err
	}
	comfy, err := findComfy()
	if err != nil {
		return err
	}
	ui := comfyUIDir(dir)
	log := comfyLog(ui, port)
	fmt.Fprintf(out, "Starting ComfyUI with comfy-cli in %s ...\n", ui)
	var envelope bytes.Buffer
	code, err := platform.Programs{}.Run(ctx, platform.Command{Path: comfy, Args: launchArgs(ui, port), Stdout: &envelope, Stderr: out})
	if err != nil {
		return fmt.Errorf("cannot run %s: %w", comfy, err)
	}
	pid, why := readEnvelope(envelope.Bytes())
	if code != 0 {
		if why != "" {
			why = " (" + why + ")"
		}
		return fmt.Errorf("comfy launch failed with exit code %d%s. Read the lines of comfy above, and the log: %s", code, why, log)
	}
	path := statePath(dir)
	st := state{Tree: "comfy-cli:" + ui, PID: pid, URL: serverURL, Started: time.Now().Format(time.RFC3339)}
	if err := writeState(path, st); err != nil {
		// Without the state file, Stop cannot end this ComfyUI: end it now.
		_, _ = platform.Programs{}.Run(ctx, platform.Command{Path: comfy, Args: stopArgs(ui), Stdout: out, Stderr: out})
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return c.waitUp(ctx, out, st, []string{log}, func() error {
		if pid > 0 && !platform.Alive(pid) {
			removeState(path, st.Tree)
			return fmt.Errorf("ComfyUI%s stopped. Read the log: %s", process(pid), log)
		}
		return nil
	})
}

// Stop runs comfy stop for the ComfyUI that Start started at the same
// address (stopStarted). It runs no stop when that process is gone.
func (c *comfyCLI) Stop(ctx context.Context, out io.Writer) error {
	dir, err := filepath.Abs(c.dir())
	if err != nil {
		return err
	}
	return c.stopStarted(ctx, out, dir, func(st *state) (bool, error) {
		if st.PID > 0 && !platform.Alive(st.PID) {
			return false, nil
		}
		comfy, err := findComfy()
		if err != nil {
			return false, err
		}
		code, err := platform.Programs{}.Run(ctx, platform.Command{Path: comfy, Args: stopArgs(comfyUIDir(dir)), Stdout: out, Stderr: out})
		if err != nil {
			return false, fmt.Errorf("cannot run %s: %w", comfy, err)
		}
		if code != 0 {
			return false, fmt.Errorf("comfy stop failed with exit code %d. Read the lines of comfy above. The state file %s stays.", code, statePath(dir))
		}
		return true, nil
	})
}

// readEnvelope reads the standard output of comfy --json: one JSON envelope
// on a line. It gives the process id of a launch that succeeded (0 when the
// envelope has none), or the message of an envelope that failed.
func readEnvelope(stdout []byte) (pid int, message string) {
	lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		var env struct {
			Type string `json:"type"`
			OK   bool   `json:"ok"`
			Data struct {
				PID int `json:"pid"`
			} `json:"data"`
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(lines[i])), &env) != nil || env.Type != "envelope" {
			continue
		}
		if env.OK {
			return env.Data.PID, ""
		}
		return 0, env.Error.Message
	}
	return 0, ""
}

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

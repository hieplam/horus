// Package startcomfyui starts ComfyUI (the portable build) and waits until its
// server answers. It ports scripts/start-comfyui.ps1.
//
// It starts the embedded Python of the portable build with ComfyUI's main.py,
// detached, with the output in horus-comfyui.out.log and horus-comfyui.err.log
// in the ComfyUI folder. The server listens on 127.0.0.1 only.
package startcomfyui

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/hieplam/horus/internal/settings"
)

// Options is everything that Start needs.
type Options struct {
	ComfyUIDir string
	ServerURL  string
	Timeout    time.Duration
	// PollInterval is the time between two checks of the server (2 s in the script).
	PollInterval time.Duration
	Stdout       io.Writer
}

var goos = runtime.GOOS

// Run is the command horus start-comfyui. It returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("horus start-comfyui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("comfyui-dir", "", "the portable folder, with ComfyUI/ and python_embeded/ (default: HORUS_COMFYUI_DIR, then <home>/ComfyUI_windows_portable)")
	serverURL := fs.String("server-url", "http://127.0.0.1:8188", "address that the server must answer on; the port is taken from it")
	timeout := fs.Int("timeout-seconds", 300, "how long to wait for the server; the first start can take a few minutes")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if goos != "windows" {
		fmt.Fprintln(stderr, "horus start-comfyui: not supported on this OS yet")
		return 2
	}
	o := Options{ComfyUIDir: *dir, ServerURL: *serverURL, Timeout: time.Duration(*timeout) * time.Second,
		PollInterval: 2 * time.Second, Stdout: stdout}
	if o.ComfyUIDir == "" {
		o.ComfyUIDir = settings.ComfyDir()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := Start(ctx, o); err != nil {
		fmt.Fprintln(stderr, "horus start-comfyui:", err)
		return 1
	}
	return 0
}

// Start starts ComfyUI when its server does not answer, then waits for it.
func Start(ctx context.Context, o Options) error {
	out := o.Stdout
	if out == nil {
		out = io.Discard
	}
	python := filepath.Join(o.ComfyUIDir, "python_embeded", "python.exe")
	mainPy := filepath.Join(o.ComfyUIDir, "ComfyUI", "main.py")
	if !isFile(python) || !isFile(mainPy) {
		return fmt.Errorf("ComfyUI was not found in %s. Run horus install-comfyui first, or pass -comfyui-dir.", o.ComfyUIDir)
	}
	if answers(ctx, o.ServerURL) {
		fmt.Fprintf(out, "ComfyUI already answers at %s\n", o.ServerURL)
		return nil
	}
	u, err := url.Parse(o.ServerURL)
	if err != nil {
		return fmt.Errorf("bad -server-url: %w", err)
	}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf("bad port in -server-url: %q", port)
	}
	outLog := filepath.Join(o.ComfyUIDir, "horus-comfyui.out.log")
	errLog := filepath.Join(o.ComfyUIDir, "horus-comfyui.err.log")
	outF, err := os.Create(outLog)
	if err != nil {
		return err
	}
	defer outF.Close()
	errF, err := os.Create(errLog)
	if err != nil {
		return err
	}
	defer errF.Close()

	// No context here: the process must outlive this command.
	cmd := exec.Command(python, "-s", filepath.Join("ComfyUI", "main.py"),
		"--windows-standalone-build", "--listen", "127.0.0.1", "--port", port)
	cmd.Dir = o.ComfyUIDir
	cmd.Stdout, cmd.Stderr = outF, errF
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot start %s: %w", python, err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	fmt.Fprintf(out, "Started ComfyUI (process %d). Waiting for %s ...\n", cmd.Process.Pid, o.ServerURL)

	deadline := time.NewTimer(o.Timeout)
	defer deadline.Stop()
	tick := time.NewTicker(o.PollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("ComfyUI did not answer within %d seconds. Read the logs: %s and %s", int(o.Timeout.Seconds()), outLog, errLog)
		case <-tick.C:
		}
		if answers(ctx, o.ServerURL) {
			fmt.Fprintf(out, "OK: ComfyUI answers at %s\n", o.ServerURL)
			return nil
		}
		select {
		case werr := <-exited:
			code := 0
			var ee *exec.ExitError
			if errors.As(werr, &ee) {
				code = ee.ExitCode()
			}
			return fmt.Errorf("ComfyUI stopped with exit code %d. Read the log: %s", code, errLog)
		default:
		}
	}
}

// answers reports whether GET <serverURL>/system_stats gives a 2xx answer.
func answers(ctx context.Context, serverURL string) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/system_stats", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

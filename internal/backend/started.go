// What Start and Stop of every backend share: the state file, the wait until
// the server answers, the wait until it does not answer, and the rule of
// Stop.
//
// The state file horus-comfyui.state.json in the install folder is the record
// of the ComfyUI that horus started. Start writes it after the start. Stop
// reads it, and it ends only the ComfyUI that the file names, at the same
// address. Without a state file for that address, Stop ends nothing, and it
// refuses when a server answers there: horus did not start that ComfyUI.
package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The defaults of Start and Stop.
const (
	startTimeout = 300 * time.Second // the wait for the server; the first start can take a few minutes
	stopTimeout  = 15 * time.Second  // the wait until the server no longer answers after the stop
	pollInterval = 2 * time.Second   // the time between two checks of the server (2 s in the script)
)

// stateName is the file name of the state file in the install folder.
const stateName = "horus-comfyui.state.json"

func orDuration(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

// state is the content of horus-comfyui.state.json: the ComfyUI that Start
// started.
type state struct {
	// Tree names what Stop ends. The portable build: the id of
	// platform.StartTree. comfy-cli: "comfy-cli:<workspace>".
	Tree    string `json:"tree"`
	PID     int    `json:"pid"`     // the process of ComfyUI, for the text and the check of Stop; 0 = not known
	URL     string `json:"url"`     // the address that this ComfyUI answers on
	Started string `json:"started"` // the time of the start (RFC 3339)
}

func statePath(dir string) string { return filepath.Join(dir, stateName) }

func writeState(path string, st state) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// readState reads the state file. No file gives nil and no error.
func readState(path string) (*state, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var st state
	if err := json.Unmarshal(data, &st); err != nil || st.Tree == "" {
		return nil, fmt.Errorf("%s is not a state file of horus. Delete it, then stop ComfyUI where it was started.", path)
	}
	return &st, nil
}

// removeState deletes the state file when it still names the tree.
func removeState(path, tree string) {
	if st, err := readState(path); err == nil && st != nil && st.Tree == tree {
		os.Remove(path)
	}
}

// process gives " (process <pid>)" for the texts of Start and Stop, or ""
// when the process is not known.
func process(pid int) string {
	if pid <= 0 {
		return ""
	}
	return fmt.Sprintf(" (process %d)", pid)
}

// readLogs gives "Read the log: <file>" or "Read the logs: <a> and <b>".
func readLogs(logs []string) string {
	if len(logs) == 1 {
		return "Read the log: " + logs[0]
	}
	return "Read the logs: " + strings.Join(logs, " and ")
}

// alreadyUp reports if the server answers, and then says so: Start has
// nothing to do.
func alreadyUp(ctx context.Context, out io.Writer, serverURL string) bool {
	if !answers(ctx, serverURL) {
		return false
	}
	fmt.Fprintf(out, "ComfyUI already answers at %s\n", serverURL)
	return true
}

// waitUp waits until the server of st answers, after Start wrote the state
// file st. ended reports an error when the ComfyUI of st ended before its
// server answered; the error ends the wait. logs are the log files that the
// text of a timeout names.
func (p Place) waitUp(ctx context.Context, out io.Writer, st state, logs []string, ended func() error) error {
	fmt.Fprintf(out, "Started ComfyUI%s. Waiting for %s ...\n", process(st.PID), st.URL)
	timeout := orDuration(p.Timeout, startTimeout)
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(orDuration(p.PollInterval, pollInterval))
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("ComfyUI did not answer within %d seconds. %s", int(timeout.Seconds()), readLogs(logs))
		case <-tick.C:
		}
		if answers(ctx, st.URL) {
			fmt.Fprintf(out, "OK: ComfyUI answers at %s\n", st.URL)
			return nil
		}
		if err := ended(); err != nil {
			return err
		}
	}
}

// stopStarted is Stop of every backend. It reads the state file of dir.
// Without a state file for the address of p, it ends nothing: when a server
// answers there, horus did not start it, and stopStarted refuses with an
// error. With one, it calls end, deletes the state file, and waits until the
// server does not answer. end ends what the state file names. It reports
// false when it ran nothing, because that ComfyUI was gone.
func (p Place) stopStarted(ctx context.Context, out io.Writer, dir string, end func(st *state) (bool, error)) error {
	serverURL := p.serverURL()
	path := statePath(dir)
	st, err := readState(path)
	if err != nil {
		return err
	}
	if st == nil || st.URL != serverURL {
		if !answers(ctx, serverURL) {
			fmt.Fprintf(out, "No ComfyUI that horus started runs at %s. Nothing to stop.\n", serverURL)
			return nil
		}
		why := fmt.Sprintf("%s holds no ComfyUI that horus started", path)
		if st != nil {
			why = fmt.Sprintf("horus started the ComfyUI at %s, not this one", st.URL)
		}
		return fmt.Errorf("horus did not start the ComfyUI that answers at %s (%s), so it does not stop it. Stop it where it was started.", serverURL, why)
	}
	upBefore := answers(ctx, serverURL)
	ended, err := end(st)
	if err != nil {
		return err
	}
	removeState(path, st.Tree)
	if !upBefore {
		fmt.Fprintf(out, "The ComfyUI that horus started at %s%s was not running any more.\n", serverURL, process(st.PID))
		return nil
	}
	if !ended || !p.waitDown(ctx, serverURL) {
		return fmt.Errorf("the ComfyUI that horus started%s is gone, but a server still answers at %s. horus did not start it, so it does not stop it.", process(st.PID), serverURL)
	}
	fmt.Fprintf(out, "OK: stopped the ComfyUI that horus started at %s%s.\n", serverURL, process(st.PID))
	return nil
}

// waitDown waits until the server no longer answers. It reports false when
// it still answers after the wait of Stop.
func (p Place) waitDown(ctx context.Context, serverURL string) bool {
	deadline := time.Now().Add(orDuration(p.StopTimeout, stopTimeout))
	every := orDuration(p.PollInterval, pollInterval)
	if every > 500*time.Millisecond {
		every = 500 * time.Millisecond
	}
	for {
		if !answers(ctx, serverURL) {
			return true
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return false
		}
		time.Sleep(every)
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

// portOf gives the port of the address, as a decimal number.
func portOf(serverURL string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", fmt.Errorf("bad -server-url: %w", err)
	}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("bad port in -server-url: %q", port)
	}
	return port, nil
}

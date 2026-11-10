// Package models installs, checks and removes the model files of a workflow.
// It ports horus/models.py.
//
// It runs `horus workflow <action> <name> -json -workflows-dir <dir>` of the
// same binary as a child process and reads its last line "HORUS_JSON {...}".
// `horus workflow` stays the one implementation of the file rules: where a
// model file lives, the SHA-256 check, the token handling (the token never
// passes through horus mcp), the refusal to overwrite or delete a file that
// differs from models.json.
//
// Only an install stops on a cancel, as in models.py: the end of its context
// kills the process tree of the child, and the .part file stays, so the next
// install resumes it. ListStatus, Check and Remove run to the end, as the
// threads of models.py did; a remove must not stop in the middle.
package models

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
)

// Mark starts the result line of `horus workflow -json`.
const Mark = "HORUS_JSON "

// Exe gives the path of the program that runs `workflow <action>`: the
// running binary. Tests replace it.
var Exe = os.Executable

// WaitDelay is the time that a run waits for the pipes to close after the
// child stopped or the context ended.
var WaitDelay = 10 * time.Second

// Item is one check or one file of the result.
type Item struct {
	Status string `json:"status"`
	Kind   string `json:"kind"`
	Role   string `json:"role"`
	File   string `json:"file"`
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Detail string `json:"detail"`
}

// Result is the HORUS_JSON line.
type Result struct {
	OK       bool   `json:"ok"`
	Action   string `json:"action"`
	Workflow string `json:"workflow"`
	Message  string `json:"message"`
	Items    []Item `json:"items"`
}

// Run is one run of `horus workflow`. The output readers fill it while the
// child runs; read it with the methods, which lock it.
type Run struct {
	mu          sync.Mutex
	lines       []string
	stderr      []string
	result      *Result
	parseErr    error
	exitCode    int
	currentDest string // the file that install downloads now
}

// Lines returns the stdout lines without the HORUS_JSON line.
func (r *Run) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.lines...)
}

// Stderr returns the stderr lines.
func (r *Run) Stderr() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.stderr...)
}

// Result returns the parsed HORUS_JSON line, or nil.
func (r *Run) Result() *Result { r.mu.Lock(); defer r.mu.Unlock(); return r.result }

// ExitCode returns the exit code of the child (-1 while it runs or when it was killed).
func (r *Run) ExitCode() int { r.mu.Lock(); defer r.mu.Unlock(); return r.exitCode }

// CurrentDest returns the file that install downloads now, or "".
func (r *Run) CurrentDest() string { r.mu.Lock(); defer r.mu.Unlock(); return r.currentDest }

func (r *Run) stdoutLine(line string, onLine func(string)) {
	line = strings.TrimRight(line, "\r\n")
	r.mu.Lock()
	if strings.HasPrefix(line, Mark) {
		var res Result
		if err := json.Unmarshal([]byte(line[len(Mark):]), &res); err != nil {
			r.parseErr = fmt.Errorf("the HORUS_JSON line does not parse: %w", err)
		} else {
			r.result = &res
		}
		r.mu.Unlock()
		return
	}
	r.lines = append(r.lines, line)
	if strings.HasPrefix(line, "-- ") && strings.Contains(line, " -> ") {
		r.currentDest = strings.TrimSpace(strings.SplitN(line, " -> ", 2)[1])
	}
	r.mu.Unlock()
	if onLine != nil {
		onLine(line)
	}
}

func (r *Run) stderrLine(line string) {
	r.mu.Lock()
	r.stderr = append(r.stderr, strings.TrimRight(line, " \t\r\n"))
	r.mu.Unlock()
}

// lineWriter splits what the child writes into lines.
type lineWriter struct {
	buf  []byte
	line func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			return len(p), nil
		}
		w.line(strings.ToValidUTF8(string(w.buf[:i]), "�"))
		w.buf = w.buf[i+1:]
	}
}

func (w *lineWriter) flush() {
	if len(w.buf) > 0 {
		w.line(strings.ToValidUTF8(string(w.buf), "�"))
		w.buf = nil
	}
}

func command(ctx context.Context, action, name string, extra []string) (*exec.Cmd, error) {
	exe, err := Exe()
	if err != nil {
		return nil, err
	}
	args := []string{"workflow", action}
	if name != "" {
		args = append(args, name)
	}
	args = append(args, "-json", "-workflows-dir", settings.WorkflowsDir())
	args = append(args, extra...)
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = settings.Root()
	cmd.Env = os.Environ()
	prepare(cmd)
	cmd.Cancel = func() error { return killTree(cmd.Process) }
	cmd.WaitDelay = WaitDelay
	return cmd, nil
}

// RunScript runs `horus workflow <action> [<name>] -json -workflows-dir <dir>
// <extra...>` and waits. It fills and returns run. When ctx ends, it kills
// the process tree of the child and returns the error of ctx.
func RunScript(ctx context.Context, action, name string, extra []string, run *Run, onLine func(string)) (*Run, error) {
	if run == nil {
		run = &Run{}
	}
	run.mu.Lock()
	run.exitCode = -1
	run.mu.Unlock()
	cmd, err := command(ctx, action, name, extra)
	if err != nil {
		return run, err
	}
	out := &lineWriter{line: func(l string) { run.stdoutLine(l, onLine) }}
	errw := &lineWriter{line: run.stderrLine}
	cmd.Stdout, cmd.Stderr = out, errw
	err = cmd.Run()
	out.flush()
	errw.flush()
	if ctx.Err() != nil {
		return run, ctx.Err()
	}
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return run, err
	}
	run.mu.Lock()
	run.exitCode = cmd.ProcessState.ExitCode()
	parseErr := run.parseErr
	run.mu.Unlock()
	if parseErr != nil {
		return run, parseErr
	}
	return run, nil
}

func tail(run *Run) []string {
	all := append(run.Lines(), run.Stderr()...)
	if len(all) > 15 {
		all = all[len(all)-15:]
	}
	return all
}

func result(run *Run, action string) (*Result, error) {
	res := run.Result()
	if res == nil {
		return nil, herr.New("INTERNAL", fmt.Sprintf("horus workflow %s printed no result", action),
			map[string]any{"exit_code": run.ExitCode(), "output_tail": tail(run)})
	}
	if res.Items == nil {
		res.Items = []Item{}
	}
	return res, nil
}

// Row is one row of `horus workflow list -json`.
type Row struct {
	Name    string `json:"name"`
	Files   int    `json:"files"`
	Present int    `json:"present"`
	Bytes   int64  `json:"bytes"`
}

// ListStatus returns name -> row. A size check only, so it is fast.
func ListStatus(ctx context.Context) (map[string]Row, error) {
	run, err := RunScript(context.WithoutCancel(ctx), "list", "", nil, nil, nil)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(strings.Join(run.Lines(), "\n"))
	if run.ExitCode() != 0 || !strings.HasPrefix(text, "[") {
		return nil, herr.New("INTERNAL", "horus workflow list failed", map[string]any{"output_tail": tail(run)})
	}
	var rows []Row
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		return nil, fmt.Errorf("horus workflow list: %w", err)
	}
	out := make(map[string]Row, len(rows))
	for _, r := range rows {
		out[r.Name] = r
	}
	return out, nil
}

// InstallState is install_state of models.py. row nil means no row.
func InstallState(row *Row) string {
	if row == nil || row.Files == 0 {
		return "not_installed"
	}
	if row.Present == row.Files {
		return "installed"
	}
	if row.Present > 0 {
		return "partially_installed"
	}
	return "not_installed"
}

// Check runs `horus workflow check` (with -quick when quick is true).
func Check(ctx context.Context, w *catalog.Workflow, quick bool) (*Result, error) {
	var extra []string
	if quick {
		extra = []string{"-quick"}
	}
	run, err := RunScript(context.WithoutCancel(ctx), "check", w.Name, extra, nil, nil)
	if err != nil {
		return nil, err
	}
	return result(run, "check")
}

func byStatus(items []Item, statuses ...string) []Item {
	var out []Item
	for _, i := range items {
		for _, s := range statuses {
			if i.Status == s {
				out = append(out, i)
			}
		}
	}
	return out
}

func files(items []Item) []string {
	out := make([]string, 0, len(items))
	for _, i := range items {
		out = append(out, i.File)
	}
	return out
}

// downloadError maps a FAILED download item to its error code.
func downloadError(item Item, stderr string, w *catalog.Workflow) error {
	text := item.Detail + "\n" + stderr
	facts := func() map[string]any {
		return map[string]any{"file": item.File, "path": item.Path, "detail": item.Detail}
	}
	// The downloader prints "the server returned error: 401". Its hint line
	// names both codes, so it is no evidence of either.
	switch {
	case strings.Contains(text, "error: 401"):
		return herr.New("HF_TOKEN_INVALID", "Hugging Face refused the token (HTTP 401)", facts())
	case strings.Contains(text, "error: 403"):
		lic, err := w.License()
		if err != nil {
			return err
		}
		d := facts()
		d["license"], d["license_url"] = lic.Name, lic.URL
		return herr.New("LICENSE_NOT_ACCEPTED", "Hugging Face refused the download (HTTP 403)", d)
	case strings.Contains(text, "Not enough free space"):
		return herr.New("DISK_FULL", "not enough free disk space for the download", facts())
	case strings.Contains(text, "Refusing to overwrite"):
		return herr.New("FILE_CONFLICT", "a different file with the same name is on disk", facts())
	}
	return herr.New("DOWNLOAD_FAILED", "the download did not finish", facts())
}

// checkError maps the items of a failed check to its error code, or nil.
func checkError(items []Item, message string) error {
	if bad := byStatus(items, "NO_NODE"); len(bad) > 0 {
		return herr.New("NODE_PACK_MISSING", "the backend lacks node classes of this workflow", map[string]any{"missing": files(bad)})
	}
	if bad := append(byStatus(items, "BAD_SIZE"), byStatus(items, "BAD_HASH")...); len(bad) > 0 {
		return herr.New("FILE_CONFLICT", "a model file on disk differs from models.json", map[string]any{"path": bad[0].Path, "files": bad})
	}
	if bad := byStatus(items, "NOT_SEEN"); len(bad) > 0 {
		return herr.New("BACKEND_RESTART_REQUIRED", "the backend does not list the new model files yet", map[string]any{"files": files(bad)})
	}
	if bad := byStatus(items, "MISSING"); len(bad) > 0 {
		return herr.New("WORKFLOW_NOT_INSTALLED", "model files are missing", map[string]any{"missing": files(bad)})
	}
	if bad := byStatus(items, "UNLISTED"); len(bad) > 0 {
		return herr.New("INTERNAL", "the workflow loads a model file that models.json does not list", map[string]any{"files": files(bad)})
	}
	if strings.Contains(message, "ComfyUI was not found") {
		return herr.New("BACKEND_NOT_INSTALLED", message, nil)
	}
	return nil
}

// Install downloads and verifies every model file, then checks. It blocks;
// run shows the progress. When ctx ends, the child is killed and the .part
// file stays.
func Install(ctx context.Context, w *catalog.Workflow, run *Run) (*Result, error) {
	if _, err := RunScript(ctx, "install", w.Name, nil, run, nil); err != nil {
		return nil, err
	}
	res, err := result(run, "install")
	if err != nil {
		return nil, err
	}
	if res.OK {
		return res, nil
	}
	if failed := byStatus(res.Items, "FAILED"); len(failed) > 0 {
		return nil, downloadError(failed[0], strings.Join(run.Stderr(), "\n"), w)
	}
	if err := checkError(res.Items, res.Message); err != nil {
		return nil, err
	}
	return nil, herr.New("INTERNAL", res.Message, map[string]any{"items": res.Items})
}

// Remove deletes the model files (confirm) or returns the plan.
func Remove(ctx context.Context, w *catalog.Workflow, confirm, forceShared bool) (*Result, error) {
	var extra []string
	if confirm {
		extra = append(extra, "-yes")
	}
	if forceShared {
		extra = append(extra, "-force")
	}
	run, err := RunScript(context.WithoutCancel(ctx), "remove", w.Name, extra, nil, nil)
	if err != nil {
		return nil, err
	}
	res, err := result(run, "remove")
	if err != nil {
		return nil, err
	}
	if res.OK {
		return res, nil
	}
	if bad := byStatus(res.Items, "DELETE_FAILED"); len(bad) > 0 {
		return nil, herr.New("FILE_IN_USE", "some model files could not be deleted", map[string]any{"files": bad})
	}
	if refused := byStatus(res.Items, "REFUSED"); len(refused) > 0 {
		return nil, herr.New("FILE_CONFLICT", "a file on disk differs from models.json, so horus does not delete it",
			map[string]any{"path": refused[0].Path, "files": refused})
	}
	return nil, herr.New("INTERNAL", res.Message, map[string]any{"items": res.Items})
}

// PartSize is the size of dest.part, else of dest, else 0.
func PartSize(dest string) int64 {
	if dest == "" {
		return 0
	}
	for _, p := range []string{dest + ".part", dest} {
		if st, err := os.Stat(p); err == nil {
			return st.Size()
		}
	}
	return 0
}

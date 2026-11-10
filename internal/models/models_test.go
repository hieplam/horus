// Tests of the port of horus/models.py. The test binary is also the fake
// `horus workflow` child, so the tests start a real child process on every
// system and never touch a real ComfyUI folder or model file.
package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
)

// When HORUS_FAKE_WORKFLOW is set, TestMain acts as `horus workflow`:
//
//	canned  write its arguments and its folder to HORUS_FAKE_ARGS, print the
//	        file HORUS_FAKE_OUT to stdout and HORUS_FAKE_ERR to stderr, exit
//	        with HORUS_FAKE_EXIT
//	sleep   start a grandchild ("leaf") that shares stdout, print the line of
//	        the file that install downloads, write its .part file, print
//	        "ready", then sleep 60 s
//	leaf    sleep 60 s
func TestMain(m *testing.M) {
	switch os.Getenv("HORUS_FAKE_WORKFLOW") {
	case "":
		os.Exit(m.Run())
	case "canned":
		wd, _ := os.Getwd()
		os.WriteFile(os.Getenv("HORUS_FAKE_ARGS"), []byte(strings.Join(os.Args[1:], " ")+"\n"+wd), 0o644)
		out, _ := os.ReadFile(os.Getenv("HORUS_FAKE_OUT"))
		os.Stdout.Write(out)
		if p := os.Getenv("HORUS_FAKE_ERR"); p != "" {
			e, _ := os.ReadFile(p)
			os.Stderr.Write(e)
		}
		n, _ := strconv.Atoi(os.Getenv("HORUS_FAKE_EXIT"))
		os.Exit(n)
	case "sleep":
		leaf := exec.Command(os.Args[0])
		leaf.Env = append(os.Environ(), "HORUS_FAKE_WORKFLOW=leaf")
		leaf.Stdout, leaf.Stderr = os.Stdout, os.Stderr
		if err := leaf.Start(); err != nil {
			fmt.Println("cannot start the leaf:", err)
			os.Exit(1)
		}
		dest := os.Getenv("HORUS_FAKE_DEST")
		fmt.Println("-- diffusion -> " + dest)
		os.WriteFile(dest+".part", make([]byte, 1234), 0o644)
		fmt.Println("ready")
		time.Sleep(60 * time.Second)
		os.Exit(0)
	case "leaf":
		time.Sleep(60 * time.Second)
		os.Exit(0)
	}
	os.Exit(3)
}

type fake struct {
	t    *testing.T
	dir  string
	args string
}

// newFake makes the test binary the child and points the settings at a
// temporary folder.
func newFake(t *testing.T) *fake {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HORUS_ROOT", dir)
	t.Setenv("HORUS_WORKFLOWS_DIR", filepath.Join(dir, "workflows"))
	settings.Reset()
	t.Cleanup(settings.Reset)
	old := Exe
	Exe = func() (string, error) { return os.Args[0], nil }
	t.Cleanup(func() { Exe = old })
	f := &fake{t: t, dir: dir, args: filepath.Join(dir, "args.txt")}
	t.Setenv("HORUS_FAKE_WORKFLOW", "canned")
	t.Setenv("HORUS_FAKE_ARGS", f.args)
	return f
}

// answer sets what the next child prints: text lines, then the HORUS_JSON
// line of res (none when res is nil), the stderr text and the exit code.
func (f *fake) answer(lines []string, res any, stderr string, exit int) {
	f.t.Helper()
	text := strings.Join(lines, "\r\n")
	if len(lines) > 0 {
		text += "\r\n"
	}
	if res != nil {
		b, err := json.Marshal(res)
		if err != nil {
			f.t.Fatal(err)
		}
		text += Mark + string(b) + "\r\n"
	}
	out := filepath.Join(f.dir, "out.txt")
	errf := filepath.Join(f.dir, "err.txt")
	os.WriteFile(out, []byte(text), 0o644)
	os.WriteFile(errf, []byte(stderr), 0o644)
	f.t.Setenv("HORUS_FAKE_OUT", out)
	f.t.Setenv("HORUS_FAKE_ERR", errf)
	f.t.Setenv("HORUS_FAKE_EXIT", strconv.Itoa(exit))
}

func (f *fake) gotArgs() (args, wd string) {
	f.t.Helper()
	b, err := os.ReadFile(f.args)
	if err != nil {
		f.t.Fatal(err)
	}
	parts := strings.SplitN(string(b), "\n", 2)
	return parts[0], parts[1]
}

// workflow makes a workflow folder with a models.json.
func (f *fake) workflow() *catalog.Workflow {
	f.t.Helper()
	dir := filepath.Join(f.dir, "workflows", "wf")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "models.json"), []byte(`{"repo": "example/model", "license": "test-license",
		"files": [{"role": "diffusion", "repo_path": "d/x.safetensors", "folder": "diffusion_models", "bytes": 10, "sha256": "00"}]}`), 0o644)
	return &catalog.Workflow{Name: "wf", Dir: dir}
}

func codeOf(t *testing.T, err error) *herr.Error {
	t.Helper()
	var he *herr.Error
	if !errors.As(err, &he) {
		t.Fatalf("want a *herr.Error, got %v", err)
	}
	return he
}

func TestRunScriptReadsTheLinesAndTheResult(t *testing.T) {
	f := newFake(t)
	f.answer([]string{"License: x", "-- vae -> /m/vae/v.safetensors", "OK        vae"},
		Result{OK: true, Action: "check", Workflow: "wf", Message: "ready", Items: []Item{{Status: "OK", Kind: "file", File: "v"}}},
		"a warning  \r\n", 0)
	var seen []string
	run, err := RunScript(context.Background(), "check", "wf", []string{"-quick"}, nil, func(l string) { seen = append(seen, l) })
	if err != nil {
		t.Fatal(err)
	}
	if got := run.Lines(); strings.Join(got, "|") != "License: x|-- vae -> /m/vae/v.safetensors|OK        vae" {
		t.Errorf("lines: %q", got)
	}
	if strings.Join(seen, "|") != strings.Join(run.Lines(), "|") {
		t.Errorf("onLine saw %q", seen)
	}
	if run.CurrentDest() != "/m/vae/v.safetensors" {
		t.Errorf("current dest %q", run.CurrentDest())
	}
	if got := run.Stderr(); len(got) != 1 || got[0] != "a warning" {
		t.Errorf("stderr %q", got)
	}
	if res := run.Result(); res == nil || !res.OK || res.Message != "ready" || len(res.Items) != 1 {
		t.Errorf("result %+v", res)
	}
	if run.ExitCode() != 0 {
		t.Errorf("exit %d", run.ExitCode())
	}
	args, wd := f.gotArgs()
	want := "workflow check wf -json -workflows-dir " + filepath.Join(f.dir, "workflows") + " -quick"
	if args != want {
		t.Errorf("args\n got %s\nwant %s", args, want)
	}
	if a, _ := filepath.EvalSymlinks(wd); a != mustEval(t, f.dir) {
		t.Errorf("the child runs in %s, want the repo root %s", wd, f.dir)
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestNoResultLineIsInternal(t *testing.T) {
	f := newFake(t)
	f.answer([]string{"FAIL: something"}, nil, "boom", 1)
	_, err := Check(context.Background(), f.workflow(), true)
	e := codeOf(t, err)
	tail, _ := e.Details["output_tail"].([]string)
	if e.Code != "INTERNAL" || e.Message != "horus workflow check printed no result" || e.Details["exit_code"] != 1 ||
		strings.Join(tail, "|") != "FAIL: something|boom" {
		t.Fatalf("%v %v", e, e.Details)
	}
}

func TestListStatusAndInstallState(t *testing.T) {
	f := newFake(t)
	f.answer([]string{`[{"name":"a","files":3,"present":3,"bytes":172,"gigabytes":0},`,
		`{"name":"b","files":3,"present":2,"bytes":5000000100,"gigabytes":5},`,
		`{"name":"c","files":2,"present":0,"bytes":9,"gigabytes":0},{"name":"d","files":0,"present":0,"bytes":0}]`}, nil, "", 0)
	rows, err := ListStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	args, _ := f.gotArgs()
	if args != "workflow list -json -workflows-dir "+filepath.Join(f.dir, "workflows") {
		t.Errorf("args %s", args)
	}
	want := map[string]string{"a": "installed", "b": "partially_installed", "c": "not_installed", "d": "not_installed"}
	for name, state := range want {
		r := rows[name]
		if got := InstallState(&r); got != state {
			t.Errorf("%s: %s, want %s", name, got, state)
		}
	}
	if rows["b"].Bytes != 5000000100 {
		t.Errorf("bytes %d", rows["b"].Bytes)
	}
	if InstallState(nil) != "not_installed" {
		t.Error("no row must be not_installed")
	}

	f.answer([]string{"FAIL: No workflows folder"}, nil, "", 1)
	if e := codeOf(t, func() error { _, err := ListStatus(context.Background()); return err }()); e.Code != "INTERNAL" || e.Message != "horus workflow list failed" {
		t.Errorf("a failed list: %v", e)
	}
}

func item(status, kind, detail string) Item {
	return Item{Status: status, Kind: kind, Role: "diffusion", File: "x.safetensors", Path: "/m/x.safetensors", Bytes: 10, Detail: detail}
}

// TestInstallMapsEachStatusToItsCode runs the child once for each outcome of
// install that models.py maps to an error code.
func TestInstallMapsEachStatusToItsCode(t *testing.T) {
	f := newFake(t)
	w := f.workflow()
	cases := []struct {
		name    string
		items   []Item
		message string
		stderr  string
		code    string // "" = ok
		check   func(e *herr.Error) bool
	}{
		{"ok", []Item{item("DOWNLOADED", "download", "")}, "installed", "", "", nil},
		{"401", []Item{item("OK", "download", ""), item("FAILED", "download", "Attempt 1 of 4 did not finish (the server returned error: 401, ...) | FAIL: x")}, "", "", "HF_TOKEN_INVALID",
			func(e *herr.Error) bool {
				return e.Details["file"] == "x.safetensors" && e.Details["path"] == "/m/x.safetensors"
			}},
		{"401 on stderr", []Item{item("FAILED", "download", "")}, "", "the server returned error: 401", "HF_TOKEN_INVALID", nil},
		{"403", []Item{item("FAILED", "download", "the server returned error: 403")}, "", "", "LICENSE_NOT_ACCEPTED",
			func(e *herr.Error) bool {
				u, _ := e.Details["license_url"].(*string)
				return e.Details["license"] == "test-license" && u != nil && *u == "https://huggingface.co/example/model"
			}},
		{"disk full", []Item{item("FAILED", "download", "FAIL: Not enough free space on the drive")}, "", "", "DISK_FULL", nil},
		{"conflict", []Item{item("FAILED", "download", "FAIL: Refusing to overwrite /m/x.safetensors")}, "", "", "FILE_CONFLICT", nil},
		{"other download", []Item{item("FAILED", "download", "FAIL: connection reset")}, "", "", "DOWNLOAD_FAILED",
			func(e *herr.Error) bool { return e.Details["detail"] == "FAIL: connection reset" }},
		{"no node", []Item{item("MISSING", "file", ""), {Status: "NO_NODE", Kind: "node", File: "VAELoader"}}, "", "", "NODE_PACK_MISSING",
			func(e *herr.Error) bool { return fmt.Sprint(e.Details["missing"]) == "[VAELoader]" }},
		{"bad size", []Item{item("BAD_SIZE", "file", ""), {Status: "BAD_HASH", Kind: "file", File: "y", Path: "/m/y"}}, "", "", "FILE_CONFLICT",
			func(e *herr.Error) bool {
				return e.Details["path"] == "/m/x.safetensors" && len(e.Details["files"].([]Item)) == 2
			}},
		{"bad hash", []Item{{Status: "BAD_HASH", Kind: "file", File: "y", Path: "/m/y"}}, "", "", "FILE_CONFLICT",
			func(e *herr.Error) bool { return e.Details["path"] == "/m/y" }},
		{"not seen", []Item{item("NOT_SEEN", "server", "")}, "", "", "BACKEND_RESTART_REQUIRED",
			func(e *herr.Error) bool { return fmt.Sprint(e.Details["files"]) == "[x.safetensors]" }},
		{"missing", []Item{item("MISSING", "file", "")}, "", "", "WORKFLOW_NOT_INSTALLED",
			func(e *herr.Error) bool { return fmt.Sprint(e.Details["missing"]) == "[x.safetensors]" }},
		{"unlisted", []Item{item("UNLISTED", "graph", "")}, "", "", "INTERNAL",
			func(e *herr.Error) bool {
				return e.Message == "the workflow loads a model file that models.json does not list"
			}},
		{"no ComfyUI", nil, "ComfyUI was not found in /c. Run horus install-comfyui first, or pass -comfyui-dir.", "", "BACKEND_NOT_INSTALLED",
			func(e *herr.Error) bool { return strings.HasPrefix(e.Message, "ComfyUI was not found in /c") }},
		{"something else", []Item{item("OK", "file", "")}, "the yaml file is bad", "", "INTERNAL",
			func(e *herr.Error) bool { return e.Message == "the yaml file is bad" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exit := 1
			if c.code == "" {
				exit = 0
			}
			f.answer([]string{"-- diffusion -> /m/x.safetensors"},
				Result{OK: c.code == "", Action: "install", Workflow: "wf", Message: c.message, Items: c.items}, c.stderr, exit)
			run := &Run{}
			res, err := Install(context.Background(), w, run)
			if c.code == "" {
				if err != nil || res == nil || !res.OK || res.Items[0].Status != "DOWNLOADED" {
					t.Fatalf("%v %+v", err, res)
				}
				return
			}
			e := codeOf(t, err)
			if e.Code != c.code || (c.check != nil && !c.check(e)) {
				t.Fatalf("got %s %q %v, want %s", e.Code, e.Message, e.Details, c.code)
			}
		})
	}
	args, _ := f.gotArgs()
	if args != "workflow install wf -json -workflows-dir "+filepath.Join(f.dir, "workflows") {
		t.Errorf("args %s", args)
	}
}

func TestRemoveMapsEachStatusToItsCode(t *testing.T) {
	f := newFake(t)
	w := f.workflow()
	cases := []struct {
		name  string
		items []Item
		code  string
	}{
		{"ok", []Item{item("DELETED", "file", "")}, ""},
		{"in use", []Item{item("DELETED", "file", ""), item("DELETE_FAILED", "file", "in use")}, "FILE_IN_USE"},
		{"refused", []Item{item("REFUSED", "file", "size differs")}, "FILE_CONFLICT"},
		{"something else", nil, "INTERNAL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exit := 1
			if c.code == "" {
				exit = 0
			}
			f.answer(nil, Result{OK: c.code == "", Action: "remove", Workflow: "wf", Message: "m", Items: c.items}, "", exit)
			res, err := Remove(context.Background(), w, true, true)
			if c.code == "" {
				if err != nil || !res.OK {
					t.Fatalf("%v %+v", err, res)
				}
				return
			}
			if e := codeOf(t, err); e.Code != c.code {
				t.Fatalf("got %s, want %s", e.Code, c.code)
			}
		})
	}
	args, _ := f.gotArgs()
	if args != "workflow remove wf -json -workflows-dir "+filepath.Join(f.dir, "workflows")+" -yes -force" {
		t.Errorf("args %s", args)
	}
	f.answer(nil, Result{OK: true, Action: "remove", Workflow: "wf", Items: []Item{}}, "", 0)
	if _, err := Remove(context.Background(), w, false, false); err != nil {
		t.Fatal(err)
	}
	if args, _ := f.gotArgs(); strings.Contains(args, "-yes") || strings.Contains(args, "-force") {
		t.Errorf("a plan must not pass -yes or -force: %s", args)
	}
}

// TestCancelKillsTheProcessTree cancels an install whose child sleeps and
// whose grandchild holds the same stdout pipe. The run returns only when
// every holder of the pipe is gone, so a kill of the child alone waits for
// WaitDelay (30 s here) and fails the 10 s limit.
func TestCancelKillsTheProcessTree(t *testing.T) {
	f := newFake(t)
	t.Setenv("HORUS_FAKE_WORKFLOW", "sleep")
	dest := filepath.Join(f.dir, "x.safetensors")
	t.Setenv("HORUS_FAKE_DEST", dest)
	old := WaitDelay
	WaitDelay = 30 * time.Second
	t.Cleanup(func() { WaitDelay = old })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	var once bool
	start := time.Now()
	done := make(chan error, 1)
	run := &Run{}
	go func() {
		_, err := RunScript(ctx, "install", "wf", nil, run, func(l string) {
			if l == "ready" && !once {
				once = true
				close(ready)
			}
		})
		done <- err
	}()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("the child stopped before it was ready: %v %q %q", err, run.Lines(), run.Stderr())
	case <-time.After(30 * time.Second):
		t.Fatal("the child did not get ready")
	}
	if got := PartSize(run.CurrentDest()); got != 1234 {
		t.Errorf("PartSize while it runs: %d", got)
	}
	cancelled := time.Now()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err %v, want context.Canceled", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the run did not return 20 s after the cancel")
	}
	if took := time.Since(cancelled); took > 10*time.Second {
		t.Fatalf("the run returned %v after the cancel: a process of the tree kept the pipe open", took)
	}
	if _, err := os.Stat(dest + ".part"); err != nil {
		t.Errorf("the .part file must stay: %v", err)
	}
	t.Logf("the child started %v before the cancel; the run returned %v after it", cancelled.Sub(start), time.Since(cancelled))
}

func TestPartSize(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "f.bin")
	if PartSize("") != 0 || PartSize(dest) != 0 {
		t.Error("no file must be 0")
	}
	os.WriteFile(dest, make([]byte, 7), 0o644)
	if PartSize(dest) != 7 {
		t.Error("the file itself")
	}
	os.WriteFile(dest+".part", make([]byte, 3), 0o644)
	if PartSize(dest) != 3 {
		t.Error("the .part file comes first")
	}
}

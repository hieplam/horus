package startcomfyui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The test binary is also the fake python.exe of the portable build: when
// HORUS_FAKE_PY is set, TestMain acts as that program. The same fake runs on
// Windows, macOS and Linux.
func TestMain(m *testing.M) {
	if os.Getenv("HORUS_FAKE_PY") != "" {
		os.Exit(fakePython(os.Args[1:]))
	}
	os.Exit(m.Run())
}

// fakePython records its process id, its folder and its arguments, writes one
// line to each output, then exits with HORUS_FAKE_EXIT, or waits (the test
// kills it).
func fakePython(args []string) int {
	wd, _ := os.Getwd()
	rec := fmt.Sprintf("pid %d\ndir %s\nargs %s\n", os.Getpid(), wd, strings.Join(args, " "))
	if err := os.WriteFile(os.Getenv("HORUS_FAKE_RECORD")+".tmp", []byte(rec), 0o644); err == nil {
		os.Rename(os.Getenv("HORUS_FAKE_RECORD")+".tmp", os.Getenv("HORUS_FAKE_RECORD"))
	}
	fmt.Fprintln(os.Stdout, "fake comfyui stdout line")
	fmt.Fprintln(os.Stderr, "fake comfyui stderr line")
	if v := os.Getenv("HORUS_FAKE_EXIT"); v != "" {
		n, _ := strconv.Atoi(v)
		return n
	}
	time.Sleep(60 * time.Second)
	return 0
}

type rig struct {
	dir    string
	record string
	out    *bytes.Buffer
	opts   Options
	srv    *httptest.Server
}

// newRig has a fake portable folder (the test binary as python.exe) and a fake
// ComfyUI server. The server answers /system_stats with 200 only after the
// fake python has recorded its start, and with 503 before that, unless
// alwaysUp is true. It never answers 200 when neverUp is true.
func newRig(t *testing.T, alwaysUp, neverUp bool) *rig {
	t.Helper()
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pyDir := filepath.Join(dir, "python_embeded")
	os.MkdirAll(pyDir, 0o755)
	copyFile(t, exe, filepath.Join(pyDir, "python.exe"))
	os.MkdirAll(filepath.Join(dir, "ComfyUI"), 0o755)
	os.WriteFile(filepath.Join(dir, "ComfyUI", "main.py"), []byte("x"), 0o644)

	r := &rig{dir: dir, record: filepath.Join(t.TempDir(), "py.record"), out: &bytes.Buffer{}}
	t.Setenv("HORUS_FAKE_PY", "1")
	t.Setenv("HORUS_FAKE_RECORD", r.record)
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/system_stats" {
			http.NotFound(w, req)
			return
		}
		if neverUp {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		if _, err := os.Stat(r.record); err != nil && !alwaysUp {
			http.Error(w, "not started", http.StatusServiceUnavailable)
			return
		}
		io.WriteString(w, `{"system":{}}`)
	}))
	t.Cleanup(r.srv.Close)
	// Kill the fake python before the temporary folders go: Windows cannot
	// delete a folder that a running program holds open.
	t.Cleanup(func() {
		if pid := r.field("pid"); pid != "" {
			n, _ := strconv.Atoi(pid)
			if p, err := os.FindProcess(n); err == nil {
				p.Kill()
				time.Sleep(300 * time.Millisecond)
			}
		}
	})
	r.opts = Options{ComfyUIDir: dir, ServerURL: r.srv.URL, Timeout: 20 * time.Second,
		PollInterval: 20 * time.Millisecond, Stdout: r.out}
	return r
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, b, 0o755); err != nil {
		t.Fatal(err)
	}
}

// field reads one line of the record of the fake python: "pid", "dir" or "args".
func (r *rig) field(name string) string {
	b, err := os.ReadFile(r.record)
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, name+" ") {
			return strings.TrimRight(strings.TrimPrefix(l, name+" "), "\r")
		}
	}
	return ""
}

func TestStartsThePythonOfThePortableBuildAndWaitsForTheServer(t *testing.T) {
	r := newRig(t, false, false)
	port := strings.TrimPrefix(r.srv.URL, "http://127.0.0.1:")
	if err := Start(context.Background(), r.opts); err != nil {
		t.Fatal(err)
	}
	wantArgs := "-s " + filepath.Join("ComfyUI", "main.py") + " --windows-standalone-build --listen 127.0.0.1 --port " + port
	if got := r.field("args"); got != wantArgs {
		t.Errorf("fake python args = %q, want %q", got, wantArgs)
	}
	if got, _ := filepath.EvalSymlinks(r.field("dir")); got != mustEval(t, r.dir) {
		t.Errorf("fake python ran in %q, want %q", got, r.dir)
	}
	for _, w := range []string{"Started ComfyUI (process " + r.field("pid") + "). Waiting for " + r.srv.URL + " ...", "OK: ComfyUI answers at " + r.srv.URL} {
		if !strings.Contains(r.out.String(), w) {
			t.Errorf("output lacks %q:\n%s", w, r.out)
		}
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	e, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestWritesTheTwoLogFiles(t *testing.T) {
	r := newRig(t, false, false)
	if err := Start(context.Background(), r.opts); err != nil {
		t.Fatal(err)
	}
	// The fake python writes the lines before it waits, but the server can answer
	// as soon as the record exists, so wait a little for the lines.
	waitFor(t, func() bool {
		a, _ := os.ReadFile(filepath.Join(r.dir, "horus-comfyui.out.log"))
		b, _ := os.ReadFile(filepath.Join(r.dir, "horus-comfyui.err.log"))
		return strings.Contains(string(a), "fake comfyui stdout line") && strings.Contains(string(b), "fake comfyui stderr line")
	}, "the two log files hold the output of the fake python")
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timeout: %s", what)
}

func TestAlreadyAnsweringStartsNothing(t *testing.T) {
	r := newRig(t, true, false)
	if err := Start(context.Background(), r.opts); err != nil {
		t.Fatal(err)
	}
	if got, want := r.out.String(), "ComfyUI already answers at "+r.srv.URL+"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if _, err := os.Stat(r.record); err == nil {
		t.Error("the fake python was started")
	}
	if _, err := os.Stat(filepath.Join(r.dir, "horus-comfyui.out.log")); err == nil {
		t.Error("a log file was written")
	}
}

func TestFailsAfterTheTimeLimitWhenNothingAnswers(t *testing.T) {
	r := newRig(t, false, true)
	r.opts.Timeout = time.Second
	start := time.Now()
	err := Start(context.Background(), r.opts)
	if err == nil || !strings.Contains(err.Error(), "ComfyUI did not answer within 1 seconds. Read the logs: ") ||
		!strings.Contains(err.Error(), "horus-comfyui.out.log and ") {
		t.Fatalf("error = %v", err)
	}
	if d := time.Since(start); d < time.Second || d > 10*time.Second {
		t.Errorf("returned after %v, want about 1s", d)
	}
	if r.field("args") == "" {
		t.Error("the fake python was not started")
	}
}

func TestFailsWhenThePythonStops(t *testing.T) {
	r := newRig(t, false, true)
	t.Setenv("HORUS_FAKE_EXIT", "3")
	err := Start(context.Background(), r.opts)
	if err == nil || !strings.Contains(err.Error(), "ComfyUI stopped with exit code 3. Read the log: ") ||
		!strings.HasSuffix(err.Error(), "horus-comfyui.err.log") {
		t.Fatalf("error = %v", err)
	}
}

func TestFailsWhenComfyUIIsNotInstalled(t *testing.T) {
	r := newRig(t, false, true)
	r.opts.ComfyUIDir = filepath.Join(r.dir, "empty")
	err := Start(context.Background(), r.opts)
	if err == nil || !strings.HasPrefix(err.Error(), "ComfyUI was not found in "+r.opts.ComfyUIDir+".") {
		t.Fatalf("error = %v", err)
	}
	if r.field("args") != "" {
		t.Error("the fake python was started")
	}
}

func TestNotSupportedOffWindows(t *testing.T) {
	old := goos
	defer func() { goos = old }()
	for _, name := range []string{"linux", "darwin"} {
		goos = name
		var out, errOut bytes.Buffer
		code := Run(nil, &out, &errOut)
		if code != 2 || !strings.Contains(errOut.String(), "not supported on this OS yet") || out.Len() != 0 {
			t.Errorf("%s: exit %d, stderr %q", name, code, errOut.String())
		}
	}
}

func TestBadFlagExits2(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"-nope"}, &out, &errOut); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}

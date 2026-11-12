package backend

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/platform"
)

type rig struct {
	dir    string
	record string
	out    *bytes.Buffer
	p      *portable
	srv    *httptest.Server
}

// newRig has a fake portable folder (the test binary as python.exe) and a fake
// ComfyUI server. The server answers /system_stats with 200 only after the
// fake python has recorded its start, and with 503 before that, unless
// alwaysUp is true. It never answers 200 when neverUp is true.
func newRig(t *testing.T, alwaysUp, neverUp bool) *rig {
	t.Helper()
	dir := fakeInstall(t)
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
	// End the fake python before the temporary folders go: Windows cannot
	// delete a folder that a running program holds open.
	t.Cleanup(func() {
		if st, _ := readState(filepath.Join(dir, stateName)); st != nil {
			platform.EndTree(st.Tree)
		}
		if pid, _ := strconv.Atoi(r.field("pid")); pid > 0 {
			if p, err := os.FindProcess(pid); err == nil {
				p.Kill()
				time.Sleep(300 * time.Millisecond)
			}
		}
	})
	r.p = &portable{Place: Place{Dir: dir, URL: r.srv.URL, Timeout: 20 * time.Second, PollInterval: 20 * time.Millisecond}}
	return r
}

// fakeInstall makes a portable folder with the test binary as python.exe.
func fakeInstall(t *testing.T) string {
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
	return dir
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

// field reads one line of the record of the fake python: "pid", "dir", "args" or "child".
func (r *rig) field(name string) string { return recordField(r.record, name) }

func recordField(record, name string) string {
	b, err := os.ReadFile(record)
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
	if err := r.p.Start(context.Background(), r.out); err != nil {
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
	st, err := readState(filepath.Join(r.dir, stateName))
	if err != nil || st == nil || st.URL != r.srv.URL || strconv.Itoa(st.PID) != r.field("pid") || st.Tree == "" {
		t.Errorf("state file = %+v, %v; want the tree of process %s at %s", st, err, r.field("pid"), r.srv.URL)
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
	if err := r.p.Start(context.Background(), r.out); err != nil {
		t.Fatal(err)
	}
	if got, want := r.p.Layout().Logs, []string{filepath.Join(r.dir, "horus-comfyui.out.log"), filepath.Join(r.dir, "horus-comfyui.err.log")}; !equal(got, want) {
		t.Errorf("Layout().Logs = %v, want %v", got, want)
	}
	// The fake python writes the lines before it waits, but the server can answer
	// as soon as the record exists, so wait a little for the lines.
	waitFor(t, func() bool {
		a, _ := os.ReadFile(filepath.Join(r.dir, "horus-comfyui.out.log"))
		b, _ := os.ReadFile(filepath.Join(r.dir, "horus-comfyui.err.log"))
		return strings.Contains(string(a), "fake comfyui stdout line") && strings.Contains(string(b), "fake comfyui stderr line")
	}, "the two log files hold the output of the fake python")
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
	if err := r.p.Start(context.Background(), r.out); err != nil {
		t.Fatal(err)
	}
	if got, want := r.out.String(), "ComfyUI already answers at "+r.srv.URL+"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if _, err := os.Stat(r.record); err == nil {
		t.Error("the fake python was started")
	}
	for _, name := range []string{"horus-comfyui.out.log", stateName} {
		if _, err := os.Stat(filepath.Join(r.dir, name)); err == nil {
			t.Errorf("%s was written", name)
		}
	}
}

func TestFailsAfterTheTimeLimitWhenNothingAnswers(t *testing.T) {
	r := newRig(t, false, true)
	r.p.Timeout = time.Second
	start := time.Now()
	err := r.p.Start(context.Background(), r.out)
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
	err := r.p.Start(context.Background(), r.out)
	if err == nil || !strings.Contains(err.Error(), "ComfyUI stopped with exit code 3. Read the log: ") ||
		!strings.HasSuffix(err.Error(), "horus-comfyui.err.log") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.dir, stateName)); err == nil {
		t.Error("the state file of an ended ComfyUI stays")
	}
}

func TestFailsWhenComfyUIIsNotInstalled(t *testing.T) {
	r := newRig(t, false, true)
	r.p.Dir = filepath.Join(r.dir, "empty")
	err := r.p.Start(context.Background(), r.out)
	if err == nil || !strings.HasPrefix(err.Error(), "ComfyUI was not found in "+r.p.Dir+".") {
		t.Fatalf("error = %v", err)
	}
	if ni, ok := err.(interface{ NotInstalled() bool }); !ok || !ni.NotInstalled() {
		t.Errorf("the error %T has no method NotInstalled", err)
	}
	if r.field("args") != "" {
		t.Error("the fake python was not to start")
	}
}

// TestPortFollowsTheSetting: without a URL, the port of the start is the
// port of HORUS_COMFYUI_URL, and the folder is HORUS_COMFYUI_DIR.
func TestPortFollowsTheSetting(t *testing.T) {
	r := newRig(t, false, false)
	useSettings(t, map[string]string{"HORUS_COMFYUI_URL": r.srv.URL + "/", "HORUS_COMFYUI_DIR": r.dir})
	p := &portable{Place: Place{Timeout: 20 * time.Second, PollInterval: 20 * time.Millisecond}}
	if err := p.Start(context.Background(), r.out); err != nil {
		t.Fatal(err)
	}
	port := strings.TrimPrefix(r.srv.URL, "http://127.0.0.1:")
	if got := r.field("args"); !strings.HasSuffix(got, "--port "+port) {
		t.Errorf("fake python args = %q, want the port %s", got, port)
	}
}

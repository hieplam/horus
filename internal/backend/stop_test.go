package backend

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/platform"
)

// standIn is a fake ComfyUI that answers on its port and has started one
// grandchild.
type standIn struct {
	dir, url, record string
}

func newStandIn(t *testing.T) *standIn {
	t.Helper()
	s := &standIn{dir: fakeInstall(t), url: freeURL(t), record: filepath.Join(t.TempDir(), "py.record")}
	// End what is left when the test ends, also when it fails.
	t.Cleanup(func() {
		if st, _ := readState(filepath.Join(s.dir, stateName)); st != nil {
			platform.EndTree(st.Tree)
		}
	})
	return s
}

// pids gives the process of the fake ComfyUI and its grandchild.
func (s *standIn) pids(t *testing.T) []int {
	t.Helper()
	waitFor(t, func() bool { return recordField(s.record, "child") != "" }, "the record of the fake ComfyUI")
	var out []int
	for _, name := range []string{"pid", "child"} {
		n, err := strconv.Atoi(recordField(s.record, name))
		if err != nil {
			t.Fatalf("record %s: %v", name, err)
		}
		killAtEnd(t, n)
		out = append(out, n)
	}
	return out
}

func countAlive(pids []int) int {
	n := 0
	for _, p := range pids {
		if alive(p) {
			n++
		}
	}
	return n
}

// TestStopEndsTheTreeThatStartStarted: a starter process calls Start and
// ends. The fake ComfyUI and its grandchild keep running. Then Stop, in this
// process, ends both through the state file.
func TestStopEndsTheTreeThatStartStarted(t *testing.T) {
	s := newStandIn(t)
	starter := exec.Command(os.Args[0])
	starter.Env = append(os.Environ(), "HORUS_BACKEND_HELPER=start", "HORUS_HELPER_PY=serve",
		"HORUS_COMFYUI_DIR="+s.dir, "HORUS_COMFYUI_URL="+s.url, "HORUS_FAKE_RECORD="+s.record)
	if out, err := starter.CombinedOutput(); err != nil {
		t.Fatalf("the starter failed: %v\n%s", err, out)
	}
	pids := s.pids(t)
	if n := countAlive(pids); n != 2 || !answers(context.Background(), s.url) {
		t.Fatalf("after the starter ended: %d of 2 processes alive, answers %v", n, answers(context.Background(), s.url))
	}

	var out bytes.Buffer
	p := &portable{Place: Place{Dir: s.dir, URL: s.url, PollInterval: 20 * time.Millisecond}}
	if err := p.Stop(context.Background(), &out); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if want := "OK: stopped the ComfyUI that horus started at " + s.url + " (process " + strconv.Itoa(pids[0]) + ").\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
	for end := time.Now().Add(5 * time.Second); countAlive(pids) > 0 && time.Now().Before(end); {
		time.Sleep(20 * time.Millisecond)
	}
	if n := countAlive(pids); n != 0 {
		t.Errorf("%d of 2 processes alive 5 s after Stop", n)
	}
	if answers(context.Background(), s.url) {
		t.Error("the server still answers after Stop")
	}
	if _, err := os.Stat(filepath.Join(s.dir, stateName)); err == nil {
		t.Error("the state file stays after Stop")
	}
}

// TestStopRefusesAComfyUIThatHorusDidNotStart: a fake ComfyUI that the test
// starts itself (not through Start) answers. Stop refuses it, with no state
// file and with a state file for another address, and every process lives.
func TestStopRefusesAComfyUIThatHorusDidNotStart(t *testing.T) {
	s := newStandIn(t)
	port := strings.TrimPrefix(s.url, "http://127.0.0.1:")
	own := exec.Command(filepath.Join(s.dir, "python_embeded", "python.exe"), "--port", port)
	own.Env = append(os.Environ(), "HORUS_FAKE_PY=serve", "HORUS_FAKE_RECORD="+s.record)
	if err := own.Start(); err != nil {
		t.Fatal(err)
	}
	go own.Wait()
	pids := s.pids(t)
	waitFor(t, func() bool { return answers(context.Background(), s.url) }, "the fake ComfyUI answers")

	// A tree that horus started at another address: Stop must not end it either.
	other := exec.Command(os.Args[0])
	other.Env = append(os.Environ(), "HORUS_FAKE_PY=grandchild")
	tree, err := platform.StartTree(other)
	if err != nil {
		t.Fatal(err)
	}
	go other.Wait()
	killAtEnd(t, other.Process.Pid)
	t.Cleanup(func() { platform.EndTree(tree) })
	otherURL := freeURL(t)

	p := &portable{Place: Place{Dir: s.dir, URL: s.url, PollInterval: 20 * time.Millisecond}}
	cases := []struct {
		name  string
		state *state
		why   string
	}{
		{"no state file", nil, "holds no ComfyUI that horus started"},
		{"a state file for another address", &state{Tree: tree, PID: other.Process.Pid, URL: otherURL}, "horus started the ComfyUI at " + otherURL + ", not this one"},
	}
	for _, c := range cases {
		if c.state != nil {
			if err := writeState(filepath.Join(s.dir, stateName), *c.state); err != nil {
				t.Fatal(err)
			}
		}
		var out bytes.Buffer
		err := p.Stop(context.Background(), &out)
		want := "horus did not start the ComfyUI that answers at " + s.url + " ("
		if err == nil || !strings.HasPrefix(err.Error(), want) || !strings.Contains(err.Error(), c.why) ||
			!strings.HasSuffix(err.Error(), "so it does not stop it. Stop it where it was started.") {
			t.Errorf("%s: error = %v", c.name, err)
		}
		if out.Len() != 0 {
			t.Errorf("%s: output = %q", c.name, out.String())
		}
	}
	time.Sleep(500 * time.Millisecond) // a wrong end of the tree has time to act
	if n := countAlive(pids); n != 2 || !answers(context.Background(), s.url) {
		t.Errorf("after the refusals: %d of 2 processes alive, answers %v", n, answers(context.Background(), s.url))
	}
	if !alive(other.Process.Pid) {
		t.Error("Stop ended the tree of another address")
	}
	if _, err := os.Stat(filepath.Join(s.dir, stateName)); err != nil {
		t.Error("Stop deleted the state file of another address")
	}
}

// TestStopWithAStaleState: the state file names a tree that has ended (ComfyUI
// ended by itself, or the PC restarted). Stop ends nothing, deletes the old
// file, and says so. When another server answers at the address, Stop says
// that horus did not start it.
func TestStopWithAStaleState(t *testing.T) {
	dir := fakeInstall(t)
	ended := exec.Command(os.Args[0])
	ended.Env = append(os.Environ(), "HORUS_FAKE_PY=1", "HORUS_FAKE_EXIT=0", "HORUS_FAKE_RECORD="+filepath.Join(t.TempDir(), "r"))
	tree, err := platform.StartTree(ended)
	if err != nil {
		t.Fatal(err)
	}
	ended.Wait()
	statePath := filepath.Join(dir, stateName)

	down := freeURL(t)
	writeState(statePath, state{Tree: tree, PID: ended.Process.Pid, URL: down})
	var out bytes.Buffer
	p := &portable{Place: Place{Dir: dir, URL: down, PollInterval: 20 * time.Millisecond, StopTimeout: 300 * time.Millisecond}}
	if err := p.Stop(context.Background(), &out); err != nil {
		t.Fatalf("nothing answers: %v", err)
	}
	if want := "The ComfyUI that horus started at " + down + " (process " + strconv.Itoa(ended.Process.Pid) + ") was not running any more.\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
	if _, err := os.Stat(statePath); err == nil {
		t.Error("the old state file stays")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "{}") }))
	defer srv.Close()
	writeState(statePath, state{Tree: tree, PID: ended.Process.Pid, URL: srv.URL})
	p.URL = srv.URL
	out.Reset()
	err = p.Stop(context.Background(), &out)
	if err == nil || !strings.Contains(err.Error(), "is gone, but a server still answers at "+srv.URL+". horus did not start it") {
		t.Fatalf("another server answers: error = %v", err)
	}
	if !answers(context.Background(), srv.URL) {
		t.Error("the other server stopped")
	}

	// No state file and nothing answers: nothing to stop.
	out.Reset()
	p.URL = down
	if err := p.Stop(context.Background(), &out); err != nil || out.String() != "No ComfyUI that horus started runs at "+down+". Nothing to stop.\n" {
		t.Errorf("nothing to stop: %v %q", err, out.String())
	}
}

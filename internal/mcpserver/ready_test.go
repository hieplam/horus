package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/jobdata"
)

// fakeQueue gives the results of horus-db and horus-worker, and records the
// deadline of its context.
type fakeQueue struct {
	db, worker bool
	calls      atomic.Int32
	deadline   atomic.Int64 // the time left until the deadline of the last call, in ns
}

func (q *fakeQueue) Checks(ctx context.Context) []jobdata.CheckResult {
	q.calls.Add(1)
	if d, ok := ctx.Deadline(); ok {
		q.deadline.Store(int64(time.Until(d)))
	}
	return []jobdata.CheckResult{
		{Dependency: jobdata.DependencyDB, Up: q.db, Detail: "db detail"},
		{Dependency: jobdata.DependencyWorker, Up: q.worker, Detail: "worker detail"},
	}
}

// liveServer is a horus api that answers LivePath with the status code.
func liveServer(t *testing.T, code int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != LivePath {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(code)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// TestStackReady: the report is ready only when the 4 checks pass, and each
// dependency that is down is named in Down, with its detail.
func TestStackReady(t *testing.T) {
	up := liveServer(t, http.StatusOK)
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close()
	for _, c := range []struct {
		name   string
		stack  Stack
		down   []string
		detail string
	}{
		{"all up", Stack{&fakeQueue{db: true, worker: true}, &fakeImages{running: true}, up}, nil, ""},
		{"this process is horus api", Stack{&fakeQueue{db: true, worker: true}, &fakeImages{running: true}, ""}, nil, ""},
		{"horus-db down", Stack{&fakeQueue{worker: true}, &fakeImages{running: true}, up}, []string{jobdata.DependencyDB}, "db detail"},
		{"horus-worker down", Stack{&fakeQueue{db: true}, &fakeImages{running: true}, up}, []string{jobdata.DependencyWorker}, "worker detail"},
		{"ComfyUI down", Stack{&fakeQueue{db: true, worker: true}, &fakeImages{}, up}, []string{jobdata.DependencyComfyUI}, "/system_stats"},
		{"horus api does not answer", Stack{&fakeQueue{db: true, worker: true}, &fakeImages{running: true}, closedURL}, []string{jobdata.DependencyAPI}, "does not answer"},
		{"horus api answers 500", Stack{&fakeQueue{db: true, worker: true}, &fakeImages{running: true}, liveServer(t, 500)}, []string{jobdata.DependencyAPI}, "500"},
	} {
		r := c.stack.Ready(context.Background())
		if r.Ready != (len(c.down) == 0) || !slices.Equal(r.Down, append([]string{}, c.down...)) {
			t.Errorf("%s: ready %v, down %v, want down %v", c.name, r.Ready, r.Down, c.down)
			continue
		}
		if len(r.Checks) != 4 {
			t.Errorf("%s: %d checks, want 4", c.name, len(r.Checks))
		}
		if c.detail != "" && !strings.Contains(r.Text(), c.detail) {
			t.Errorf("%s: the report does not say %q:\n%s", c.name, c.detail, r.Text())
		}
	}
}

// TestStackReadyOnly: with only, the stack runs only the checks of those
// dependencies, and gives a time limit of CheckTimeout to each check.
func TestStackReadyOnly(t *testing.T) {
	q := &fakeQueue{db: true, worker: true}
	images := &fakeImages{}
	r := Stack{q, images, "http://127.0.0.1:1"}.Ready(context.Background(), jobdata.DependencyWorker)
	if q.calls.Load() != 1 {
		t.Fatalf("the queue checks ran %d times, want 1", q.calls.Load())
	}
	if left := time.Duration(q.deadline.Load()); left <= 0 || left > CheckTimeout {
		t.Errorf("the time left for the check: %s, want at most %s", left, CheckTimeout)
	}
	var lines []string
	for _, c := range r.Checks {
		if c.Detail != "no check result" {
			lines = append(lines, c.Dependency)
		}
	}
	if !slices.Equal(lines, []string{jobdata.DependencyWorker}) || !r.Checks[2].Up {
		t.Errorf("only horus-worker: the results %+v", r.Checks)
	}
}

// TestNotReadyGate: in the stack, a call while a dependency is down gets
// NOT_READY that names it in details.down, and the tool does not run. When
// the dependency is up again, the same session serves again.
func TestNotReadyGate(t *testing.T) {
	useRepoWorkflows(t)
	images := &fakeImages{running: true, result: &generate.Result{Workflow: "it-illustration", Path: "/x/a.png", Bytes: 8, Width: 1024, Height: 768, JobID: "j1"}}
	var down atomic.Bool
	down.Store(true)
	ready := func(ctx context.Context, only ...string) jobdata.Readiness {
		comfy := jobdata.CheckResult{Dependency: jobdata.DependencyComfyUI, Up: !down.Load(), Detail: "no answer in 2 s"}
		return jobdata.ReadinessOf([]jobdata.CheckResult{{Dependency: jobdata.DependencyDB, Up: true},
			comfy, {Dependency: jobdata.DependencyWorker, Up: true}, {Dependency: jobdata.DependencyAPI, Up: true}})
	}
	cs := connect(t, NewWithReady(images, nil, nil, ready), nil)
	call := map[string]any{"workflow": "it-illustration", "subject": "a falcon over a server rack, flat colours"}
	res := callTool(t, cs, "generate_image", call)
	env := envelopeOf(t, res)
	e, _ := env["error"].(map[string]any)
	if !res.IsError || e["code"] != "NOT_READY" || e["action_by"] != "human" || e["retryable"] != true {
		t.Fatalf("a call while ComfyUI is down: %v", env)
	}
	d, _ := e["details"].(map[string]any)
	if got, _ := d["down"].([]any); len(got) != 1 || got[0] != jobdata.DependencyComfyUI {
		t.Errorf("details.down = %v, want [comfyui]", d["down"])
	}
	if checks, _ := d["checks"].([]any); len(checks) != 4 {
		t.Errorf("details.checks = %v, want the 4 results", d["checks"])
	}
	if msg, _ := e["message"].(string); !strings.Contains(msg, "comfyui (no answer in 2 s)") {
		t.Errorf("the message does not name ComfyUI and its cause: %q", msg)
	}
	if n := len(images.requests()); n != 0 {
		t.Errorf("the tool ran %d times while horus was not ready", n)
	}
	down.Store(false)
	env = envelopeOf(t, callTool(t, cs, "generate_image", call))
	if env["ok"] != true || len(images.requests()) != 1 {
		t.Errorf("a call after ComfyUI is up again: %v, %d runs", env, len(images.requests()))
	}
}

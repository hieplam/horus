// A fake of the interface generate.ComfyUI for the tests of the worker. It
// answers like the fake ComfyUI of internal/comfy/comfytest, without HTTP: a
// handler test does not import an adapter (go test ./internal/archtest). The
// same worker through the real adapter and comfytest over HTTP, on the
// Postgres job store and broker, is tested in cmd/horus (worker_test.go).
package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/graph"
	"github.com/hieplam/horus/internal/herr"
)

// The modes of a job.
const (
	modeOK   = "ok"   // done on the second poll, with one image
	modeHold = "hold" // runs until release, then done on the next poll
)

// fakeJob is one submitted graph.
type fakeJob struct {
	mode     string
	graph    map[string]any
	polls    int
	released bool
	// ended: a poll reported the job as done, or a cancel removed it. The
	// fake no longer holds it.
	ended bool
}

// fakeComfy counts each submit, and the most jobs that it held at one time.
type fakeComfy struct {
	mu      sync.Mutex
	down    bool
	mode    string
	info    graph.ObjectInfo
	jobs    map[string]*fakeJob
	order   []string
	cancels []string
	refused int // the submits while down
	most    int // the most jobs held at one time
}

func newFake(t *testing.T) *fakeComfy {
	t.Helper()
	data, err := os.ReadFile("../../tests/fixtures/object_info.json")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeComfy{mode: modeOK, jobs: map[string]*fakeJob{}}
	if err := json.Unmarshal(data, &f.info); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fakeComfy) setMode(m string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mode = m
}

// setDown makes the fake stop answering (true) or answer again (false).
func (f *fakeComfy) setDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
}

// submits gives the ids of the submitted jobs, in order.
func (f *fakeComfy) submits() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.order...)
}

// seed gives the seed of the KSampler node of the submitted job id.
func (f *fakeComfy) seed(id string) uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, n := range f.jobs[id].graph {
		node, _ := n.(map[string]any)
		if node["class_type"] == "KSampler" {
			v, _ := node["inputs"].(map[string]any)["seed"].(float64)
			return uint64(v)
		}
	}
	return 0
}

// active gives the count of the jobs that the fake holds. Hold mu.
func (f *fakeComfy) active() int {
	n := 0
	for _, j := range f.jobs {
		if !j.ended {
			n++
		}
	}
	return n
}

// Active is the count of the jobs that the fake holds now.
func (f *fakeComfy) Active() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active()
}

// mostActive is the most jobs that the fake held at one time.
func (f *fakeComfy) mostActive() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.most
}

// held gives the ids of the jobs in the mode hold that wait for release.
func (f *fakeComfy) held() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, id := range f.order {
		if j := f.jobs[id]; j.mode == modeHold && !j.released && !j.ended {
			out = append(out, id)
		}
	}
	return out
}

// release lets each held job end at its next poll.
func (f *fakeComfy) release() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, j := range f.jobs {
		j.released = true
	}
}

// refusedSubmits is the count of the submits while the fake was down.
func (f *fakeComfy) refusedSubmits() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refused
}

func (f *fakeComfy) unavailable() error {
	return herr.New("BACKEND_UNAVAILABLE", "ComfyUI does not answer at the fake", nil)
}

func (f *fakeComfy) IsUp(context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.down, nil
}

func (f *fakeComfy) Address() string { return "http://fake" }

func (f *fakeComfy) ObjectInfo(context.Context) (graph.ObjectInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return nil, f.unavailable()
	}
	return f.info, nil
}

func (f *fakeComfy) Submit(_ context.Context, api *graph.API) (string, error) {
	raw, err := json.Marshal(api)
	if err != nil {
		return "", err
	}
	var g map[string]any
	if err := json.Unmarshal(raw, &g); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		f.refused++
		return "", f.unavailable()
	}
	id := fmt.Sprintf("job-%d", len(f.order)+1)
	f.jobs[id] = &fakeJob{mode: f.mode, graph: g}
	f.order = append(f.order, id)
	f.most = max(f.most, f.active())
	return id, nil
}

func (f *fakeComfy) History(_ context.Context, id string) (*graph.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return nil, f.unavailable()
	}
	j, ok := f.jobs[id]
	if !ok || j.ended {
		return nil, nil
	}
	j.polls++
	if j.mode == modeHold && !j.released || j.mode == modeOK && j.polls < 2 {
		return nil, nil
	}
	j.ended = true
	secs := 3.5
	return &graph.Job{Done: true, Images: []graph.Image{{Filename: id + ".png", Type: "output"}}, BackendSeconds: &secs}, nil
}

func (f *fakeComfy) QueueState(_ context.Context, id string) (graph.Queue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return graph.Queue{}, f.unavailable()
	}
	if j, ok := f.jobs[id]; ok && !j.ended {
		return graph.Queue{Running: true}, nil
	}
	return graph.Queue{}, nil
}

func (f *fakeComfy) Cancel(_ context.Context, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancels = append(f.cancels, id)
	if j, ok := f.jobs[id]; ok {
		j.ended = true
	}
}

// View gives the 8 bytes of the PNG signature and the name of the image.
func (f *fakeComfy) View(_ context.Context, img graph.Image) ([]byte, error) {
	return []byte("\x89PNG\r\n\x1a\n" + img.Filename), nil
}

func (f *fakeComfy) Free(context.Context) {}

// The fake implements the interface of the service.
var _ generate.ComfyUI = (*fakeComfy)(nil)

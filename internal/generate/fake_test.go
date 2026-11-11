// A fake of the interface ComfyUI for the tests of the service. It answers
// like the fake ComfyUI of internal/comfy/comfytest, without HTTP: the wire
// format is the work of the adapter internal/comfy and of its tests.
package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/hieplam/horus/internal/graph"
	"github.com/hieplam/horus/internal/herr"
)

// Modes of a job, as in comfytest.
const (
	OK      = "ok"      // done on the second poll, one image
	OOM     = "oom"     // fails on the second poll with OUT_OF_MEMORY
	Slow    = "slow"    // runs for ever
	Pending = "pending" // waits in the queue for ever, at position 2
	Absent  = "absent"  // is neither in the history nor in the queue
	NoImage = "noimage" // done, without an image
)

// fakeJob is one submitted graph.
type fakeJob struct {
	Mode  string
	Graph map[string]any
	Polls int
}

type fakeComfy struct {
	mu      sync.Mutex
	down    bool
	mode    string
	info    graph.ObjectInfo
	jobs    map[string]*fakeJob
	order   []string
	cancels []string
	frees   int
}

func newFake(t *testing.T, objectInfo string) *fakeComfy {
	t.Helper()
	data, err := os.ReadFile(objectInfo)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeComfy{mode: OK, jobs: map[string]*fakeJob{}}
	if err := json.Unmarshal(data, &f.info); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fakeComfy) SetMode(m string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mode = m
}

// SetDown makes the server not answer.
func (f *fakeComfy) SetDown() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = true
}

func (f *fakeComfy) Jobs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.order...)
}

func (f *fakeComfy) Job(id string) *fakeJob {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.jobs[id]
}

func (f *fakeComfy) Polls(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if j, ok := f.jobs[id]; ok {
		return j.Polls
	}
	return 0
}

// Cancels are the job ids of the calls of Cancel.
func (f *fakeComfy) Cancels() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.cancels...)
}

func (f *fakeComfy) Frees() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.frees
}

func (f *fakeComfy) unavailable() error {
	return herr.New("BACKEND_UNAVAILABLE", "ComfyUI does not answer at the fake", nil)
}

func (f *fakeComfy) IsUp(ctx context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.down, nil
}

func (f *fakeComfy) Address() string { return "http://fake" }

// SetUp makes the server answer again.
func (f *fakeComfy) SetUp() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = false
}

// fakeBackend is a fake of the interface Backend. Each Start counts, writes
// the lines, then calls start (nil: the start brings the fake server up).
type fakeBackend struct {
	mu     sync.Mutex
	comfy  *fakeComfy
	starts int
	lines  string
	start  func() error
}

func (b *fakeBackend) Start(ctx context.Context, out io.Writer) error {
	b.mu.Lock()
	b.starts++
	lines, start := b.lines, b.start
	b.mu.Unlock()
	io.WriteString(out, lines)
	if start != nil {
		return start()
	}
	b.comfy.SetUp()
	return nil
}

func (b *fakeBackend) Starts() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.starts
}

// notInstalled is an error of a start of a ComfyUI that is not installed,
// as internal/backend gives it.
type notInstalled struct{ msg string }

func (e notInstalled) Error() string      { return e.msg }
func (e notInstalled) NotInstalled() bool { return true }

func (f *fakeComfy) ObjectInfo(ctx context.Context) (graph.ObjectInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return nil, f.unavailable()
	}
	return f.info, nil
}

// Submit keeps the graph as the JSON value that the server receives.
func (f *fakeComfy) Submit(ctx context.Context, api *graph.API) (string, error) {
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
	id := fmt.Sprintf("job-%d", len(f.order)+1)
	f.jobs[id] = &fakeJob{Mode: f.mode, Graph: g}
	f.order = append(f.order, id)
	return id, nil
}

func (f *fakeComfy) History(ctx context.Context, id string) (*graph.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job, ok := f.jobs[id]
	if !ok {
		return nil, nil
	}
	job.Polls++
	switch {
	case job.Mode == Slow || job.Mode == Pending || job.Mode == Absent || job.Polls < 2:
		return nil, nil
	case job.Mode == OOM:
		return nil, herr.New("OUT_OF_MEMORY", "the GPU ran out of memory", map[string]any{"job_id": id})
	case job.Mode == NoImage:
		return &graph.Job{Done: true, BackendSeconds: ptr(3.5)}, nil
	}
	return &graph.Job{Done: true, Images: []graph.Image{{Filename: id + ".png", Type: "output"}}, BackendSeconds: ptr(3.5)}, nil
}

func (f *fakeComfy) QueueState(ctx context.Context, id string) (graph.Queue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job, ok := f.jobs[id]
	if !ok || contains(f.cancels, id) {
		return graph.Queue{}, nil
	}
	switch job.Mode {
	case Slow:
		return graph.Queue{Running: true}, nil
	case Pending:
		return graph.Queue{Position: 2}, nil
	}
	return graph.Queue{}, nil
}

func (f *fakeComfy) Cancel(ctx context.Context, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancels = append(f.cancels, id)
}

// View returns the 8 bytes of the PNG signature and the name of the image.
func (f *fakeComfy) View(ctx context.Context, img graph.Image) ([]byte, error) {
	return []byte("\x89PNG\r\n\x1a\n" + img.Filename), nil
}

func (f *fakeComfy) Free(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.frees++
}

func ptr[T any](v T) *T { return &v }

// The fake implements the interface of the service.
var _ ComfyUI = (*fakeComfy)(nil)

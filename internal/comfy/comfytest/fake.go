// Package comfytest is a fake ComfyUI for the Go tests, on an ephemeral port.
// It answers like the fake ComfyUI of tests/mcp_contract.py.
package comfytest

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

// Modes of a job.
const (
	OK      = "ok"      // done on the second poll, one image
	OOM     = "oom"     // fails with a CUDA out-of-memory error
	Slow    = "slow"    // runs for ever
	Pending = "pending" // waits in the queue for ever
	Absent  = "absent"  // is neither in /history nor in /queue
	NoImage = "noimage" // done, without an image
	Reject  = "reject"  // /prompt answers HTTP 400 with node_errors for a model file
)

// Job is one submitted graph.
type Job struct {
	Mode  string
	Graph map[string]any
	Polls int
}

// Fake is the state of the fake server.
type Fake struct {
	Server *httptest.Server

	mu         sync.Mutex
	mode       string
	info       []byte
	jobs       map[string]*Job
	order      []string
	interrupts []string
	deleted    []string
	frees      int
}

// New starts a fake ComfyUI that serves objectInfo (a path) at /object_info.
// The test closes it at the end.
func New(t *testing.T, objectInfo string) *Fake {
	t.Helper()
	info, err := os.ReadFile(objectInfo)
	if err != nil {
		t.Fatal(err)
	}
	f := &Fake{mode: OK, info: info, jobs: map[string]*Job{}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Server.Close)
	return f
}

// URL is the address of the server.
func (f *Fake) URL() string { return f.Server.URL }

// SetMode sets the mode of the next jobs.
func (f *Fake) SetMode(m string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mode = m
}

// Jobs returns the ids of the submitted jobs, in order.
func (f *Fake) Jobs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.order...)
}

// Job returns one submitted job.
func (f *Fake) Job(id string) *Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.jobs[id]
}

// Polls is the count of GET /history for one job.
func (f *Fake) Polls(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if j, ok := f.jobs[id]; ok {
		return j.Polls
	}
	return 0
}

// Interrupts are the prompt ids of POST /interrupt.
func (f *Fake) Interrupts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.interrupts...)
}

// Deleted are the prompt ids of POST /queue {"delete"}.
func (f *Fake) Deleted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deleted...)
}

// Frees is the count of POST /free.
func (f *Fake) Frees() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.frees
}

// TinyPNG is a 1x1 PNG, the same as tiny_png of the contract test.
func TinyPNG() []byte {
	chunk := func(kind string, data []byte) []byte {
		var b bytes.Buffer
		_ = binary.Write(&b, binary.BigEndian, uint32(len(data)))
		b.WriteString(kind)
		b.Write(data)
		_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(kind), data...)))
		return b.Bytes()
	}
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	_, _ = w.Write([]byte{0, 0xff, 0xff, 0xff})
	_ = w.Close()
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	b.Write(chunk("IHDR", []byte{0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0}))
	b.Write(chunk("IDAT", z.Bytes()))
	b.Write(chunk("IEND", nil))
	return b.Bytes()
}

func send(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if r.Method == http.MethodGet {
		switch {
		case p == "/system_stats":
			send(w, 200, map[string]any{"system": map[string]any{"comfyui_version": "fake"}})
		case p == "/object_info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(f.info)
		case strings.HasPrefix(p, "/history/"):
			f.history(w, strings.TrimPrefix(p, "/history/"))
		case p == "/queue":
			f.mu.Lock()
			running, pending := [][]any{}, [][]any{}
			for i, id := range f.order {
				if contains(f.interrupts, id) || contains(f.deleted, id) {
					continue
				}
				switch f.jobs[id].Mode {
				case Slow:
					running = append(running, []any{i, id})
				case Pending:
					pending = append(pending, []any{i + 100, id})
				}
			}
			f.mu.Unlock()
			// A second, older pending job puts ours at position 2.
			if len(pending) > 0 {
				pending = append([][]any{{1, "other-job"}}, pending...)
			}
			send(w, 200, map[string]any{"queue_running": running, "queue_pending": pending})
		case p == "/view":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(TinyPNG())
		default:
			send(w, 404, map[string]any{"error": "not found"})
		}
		return
	}
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	f.mu.Lock()
	defer f.mu.Unlock()
	switch p {
	case "/prompt":
		if f.mode == Reject {
			send(w, 400, map[string]any{"error": map[string]any{"type": "prompt_outputs_failed_validation"},
				"node_errors": map[string]any{"1": map[string]any{"errors": []any{map[string]any{
					"type": "value_not_in_list", "extra_info": map[string]any{"received_value": "gone.safetensors"}}}}}})
			return
		}
		id := fmt.Sprintf("job-%d", len(f.order)+1)
		g, _ := body["prompt"].(map[string]any)
		f.jobs[id] = &Job{Mode: f.mode, Graph: g}
		f.order = append(f.order, id)
		send(w, 200, map[string]any{"prompt_id": id, "number": len(f.order), "node_errors": map[string]any{}})
	case "/interrupt":
		id, _ := body["prompt_id"].(string)
		f.interrupts = append(f.interrupts, id)
		send(w, 200, map[string]any{})
	case "/queue":
		if d, ok := body["delete"].([]any); ok {
			for _, x := range d {
				s, _ := x.(string)
				f.deleted = append(f.deleted, s)
			}
		}
		send(w, 200, map[string]any{})
	case "/free":
		f.frees++
		w.WriteHeader(200)
	default:
		send(w, 404, map[string]any{"error": "not found"})
	}
}

func (f *Fake) history(w http.ResponseWriter, id string) {
	f.mu.Lock()
	job, ok := f.jobs[id]
	if ok {
		job.Polls++
	}
	f.mu.Unlock()
	if !ok {
		send(w, 200, map[string]any{})
		return
	}
	switch {
	case job.Mode == Slow || job.Mode == Pending || job.Mode == Absent || job.Polls < 2:
		send(w, 200, map[string]any{})
	case job.Mode == OOM:
		send(w, 200, map[string]any{id: map[string]any{"status": map[string]any{"status_str": "error", "completed": false,
			"messages": []any{
				[]any{"execution_start", map[string]any{"timestamp": 1}},
				[]any{"execution_error", map[string]any{"node_type": "KSampler", "exception_type": "torch.OutOfMemoryError",
					"exception_message": "CUDA out of memory. Tried to allocate 2.00 GiB"}}}},
			"outputs": map[string]any{}}})
	default:
		outputs := map[string]any{"9": map[string]any{"images": []any{map[string]any{"filename": id + ".png", "subfolder": "", "type": "output"}}}}
		if job.Mode == NoImage {
			outputs = map[string]any{}
		}
		send(w, 200, map[string]any{id: map[string]any{"status": map[string]any{"status_str": "success", "completed": true,
			"messages": []any{[]any{"execution_start", map[string]any{"timestamp": 1000}},
				[]any{"execution_success", map[string]any{"timestamp": 4500}}}},
			"outputs": outputs}})
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

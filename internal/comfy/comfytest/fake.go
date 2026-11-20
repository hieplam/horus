// Package comfytest is a fake ComfyUI for the Go tests, on an ephemeral port.
// It answers like the fake ComfyUI of tests/mcp_contract.py.
//
// As ComfyUI 0.44.0, it gives the step of a job only on its WebSocket
// /ws?clientId=<id>, and only to the client id of the job: no REST route has
// a step or a total. A test sends a step with Step.
package comfytest

import (
	"bytes"
	"compress/zlib"
	"context"
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

	"github.com/coder/websocket"
)

// Modes of a job.
const (
	OK      = "ok"      // done on the second poll, one image
	OOM     = "oom"     // fails with a CUDA out-of-memory error
	OOMMPS  = "oommps"  // fails with the out-of-memory error of PyTorch on MPS (a Mac)
	Slow    = "slow"    // runs for ever
	Pending = "pending" // waits in the queue for ever
	Absent  = "absent"  // is neither in /history nor in /queue
	NoImage = "noimage" // done, without an image
	Reject  = "reject"  // /prompt answers HTTP 400 with node_errors for a model file
)

// MPSOutOfMemoryText is the error text of PyTorch when a Mac runs out of memory on MPS.
const MPSOutOfMemoryText = "MPS backend out of memory (MPS allocated: 9.07 GB, other allocations: 6.50 GB, max allowed: 18.13 GB). Tried to allocate 256.00 MB on private pool. Use PYTORCH_MPS_HIGH_WATERMARK_RATIO=0.0 to disable upper limit for memory allocations (may cause system failure)."

// Job is one submitted graph.
type Job struct {
	Mode  string
	Graph map[string]any
	Polls int
	// ClientID is the client_id of the POST /prompt of the job.
	ClientID string
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
	down       bool
	refused    int                        // the POST /prompt requests while down
	sockets    map[string]*websocket.Conn // the open WebSockets, by client id
	noSockets  bool                       // GET /ws answers 404
}

// New starts a fake ComfyUI that serves objectInfo (a path) at /object_info.
// The test closes it at the end.
func New(t *testing.T, objectInfo string) *Fake {
	t.Helper()
	info, err := os.ReadFile(objectInfo)
	if err != nil {
		t.Fatal(err)
	}
	f := &Fake{mode: OK, info: info, jobs: map[string]*Job{}, sockets: map[string]*websocket.Conn{}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Server.Close)
	// Close waits for each request, and a WebSocket is a request that does
	// not end: close the sockets first.
	t.Cleanup(f.DropSockets)
	return f
}

// RefuseSockets makes GET /ws answer 404, as a server without a WebSocket.
func (f *Fake) RefuseSockets() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.noSockets = true
}

// DropSockets closes each open WebSocket, as a ComfyUI whose connection
// fails while a job runs.
func (f *Fake) DropSockets() {
	f.mu.Lock()
	open := f.sockets
	f.sockets = map[string]*websocket.Conn{}
	f.mu.Unlock()
	for _, c := range open {
		_ = c.CloseNow()
	}
}

// Sockets is the count of the open WebSockets.
func (f *Fake) Sockets() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sockets)
}

// SamplerNode gives the id of the KSampler node in the graph of the job, or
// "" when the graph has none.
func (f *Fake) SamplerNode(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	job, ok := f.jobs[id]
	if !ok {
		return ""
	}
	for node, v := range job.Graph {
		if n, _ := v.(map[string]any); n["class_type"] == "KSampler" {
			return node
		}
	}
	return ""
}

// Step reports step value of max for the KSampler node of the job, exactly
// as ComfyUI 0.44.0 does: one "progress" message on the WebSocket of the
// client id of the job. It gives false when that WebSocket is not open, and
// then nobody gets the step.
func (f *Fake) Step(id string, value, max int) bool {
	return f.Send(id, map[string]any{"type": "progress",
		"data": map[string]any{"value": value, "max": max, "prompt_id": id, "node": f.SamplerNode(id)}})
}

// End reports the end of the job on its WebSocket: "execution_success".
func (f *Fake) End(id string) bool {
	return f.Send(id, map[string]any{"type": "execution_success", "data": map[string]any{"prompt_id": id, "timestamp": 4500}})
}

// Send sends one JSON message on the WebSocket of the client id of the job.
// It gives false when that WebSocket is not open.
func (f *Fake) Send(id string, message any) bool {
	f.mu.Lock()
	var conn *websocket.Conn
	if job, ok := f.jobs[id]; ok {
		conn = f.sockets[job.ClientID]
	}
	f.mu.Unlock()
	if conn == nil {
		return false
	}
	raw, err := json.Marshal(message)
	if err != nil {
		return false
	}
	return conn.Write(context.Background(), websocket.MessageText, raw) == nil
}

// socket serves GET /ws?clientId=<id>: it keeps the WebSocket of the client
// id until it closes. As ComfyUI, it sends one "status" message first.
func (f *Fake) socket(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	refuse := f.noSockets
	f.mu.Unlock()
	if refuse {
		send(w, 404, map[string]any{"error": "not found"})
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	sid := r.URL.Query().Get("clientId")
	f.mu.Lock()
	f.sockets[sid] = conn
	f.mu.Unlock()
	status, _ := json.Marshal(map[string]any{"type": "status",
		"data": map[string]any{"status": map[string]any{"exec_info": map[string]any{"queue_remaining": 0}}, "sid": sid}})
	_ = conn.Write(context.Background(), websocket.MessageText, status)
	for {
		if _, _, err := conn.Read(context.Background()); err != nil {
			break
		}
	}
	_ = conn.CloseNow()
	f.mu.Lock()
	if f.sockets[sid] == conn {
		delete(f.sockets, sid)
	}
	f.mu.Unlock()
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

// SetDown makes the fake stop answering (true) or answer again (false).
// While it is down, it closes each connection without an answer, as a
// ComfyUI that does not run. It also closes each open WebSocket.
func (f *Fake) SetDown(down bool) {
	f.mu.Lock()
	f.down = down
	f.mu.Unlock()
	if down {
		f.DropSockets()
	}
}

// RefusedPrompts is the count of POST /prompt that came while the fake was
// down.
func (f *Fake) RefusedPrompts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refused
}

// Frees is the count of POST /free.
func (f *Fake) Frees() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.frees
}

// tinyPNG is a 1x1 PNG, the same as tiny_png of the contract test.
func tinyPNG() []byte {
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
	f.mu.Lock()
	down := f.down
	if down && r.Method == http.MethodPost && p == "/prompt" {
		f.refused++
	}
	f.mu.Unlock()
	if down {
		if h, ok := w.(http.Hijacker); ok {
			if conn, _, err := h.Hijack(); err == nil {
				_ = conn.Close()
				return
			}
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		switch {
		case p == "/ws":
			f.socket(w, r)
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
			_, _ = w.Write(tinyPNG())
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
		client, _ := body["client_id"].(string)
		f.jobs[id] = &Job{Mode: f.mode, Graph: g, ClientID: client}
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
	case job.Mode == OOM || job.Mode == OOMMPS:
		etype, emsg := "torch.OutOfMemoryError", "CUDA out of memory. Tried to allocate 2.00 GiB"
		if job.Mode == OOMMPS {
			etype, emsg = "RuntimeError", MPSOutOfMemoryText
		}
		send(w, 200, map[string]any{id: map[string]any{"status": map[string]any{"status_str": "error", "completed": false,
			"messages": []any{
				[]any{"execution_start", map[string]any{"timestamp": 1}},
				[]any{"execution_error", map[string]any{"node_type": "KSampler", "exception_type": etype,
					"exception_message": emsg}}}},
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

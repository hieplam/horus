// Package comfy is the adapter for the local ComfyUI: its HTTP client. It
// ports horus/comfy.py. *Client implements the interface generate.ComfyUI.
// The pure graph conversion is in internal/graph.
//
// The wire format stays in this package: the JSON of /history and /queue, the
// client_id of a job, and the mapping of a rejected graph or a failed job to a
// horus error (jobs.go).
//
// ComfyUI endpoints used:
//
//	GET  /system_stats        is the server up
//	GET  /object_info         node classes, their inputs, and the model files the server lists
//	POST /prompt              submit a graph in API format, returns prompt_id
//	GET  /history/<id>        status and outputs of a job
//	GET  /queue               running and pending jobs
//	POST /queue {"delete"}    remove a pending job
//	POST /interrupt           stop the running job
//	POST /free                unload models (before horus deletes model files)
//	GET  /view                download an output image
package comfy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/graph"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
)

// Client talks to one ComfyUI server. An empty URL means the
// HORUS_COMFYUI_URL setting, read at each request.
type Client struct {
	URL string
}

// New makes a client for url. An empty url makes a client for the
// HORUS_COMFYUI_URL setting: it reads the setting at each request, so a
// client that cmd/horus makes at the start sees the setting of each call.
func New(u string) *Client {
	return &Client{URL: strings.TrimRight(u, "/")}
}

// base is the address of the server for this request.
func (c *Client) base() string {
	if c.URL != "" {
		return c.URL
	}
	return settings.ComfyURL()
}

// HTTPError is an answer with a status of 400 or more. comfy.py lets
// urllib's HTTPError pass through; so does this client.
type HTTPError struct {
	Code int
	Body []byte
}

func (e *HTTPError) Error() string { return fmt.Sprintf("HTTP Error %d", e.Code) }

// open sends one request. A transport failure is BACKEND_UNAVAILABLE.
// A cancelled call (not a timeout) gives the error of ctx, as a cancelled
// asyncio task in Python raises CancelledError and no horus error.
func (c *Client) open(parent context.Context, method, path string, body []byte, timeout time.Duration) ([]byte, error) {
	data, err := c.send(parent, method, path, body, timeout)
	if err != nil && parent.Err() != nil {
		return nil, parent.Err()
	}
	return data, err
}

func (c *Client) send(ctx context.Context, method, path string, body []byte, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base()+path, rd)
	if err != nil {
		return nil, c.unavailable(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, c.unavailable(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, c.unavailable(err)
	}
	if resp.StatusCode >= 400 {
		return nil, &HTTPError{Code: resp.StatusCode, Body: data}
	}
	return data, nil
}

func (c *Client) unavailable(err error) error {
	return herr.New("BACKEND_UNAVAILABLE", "ComfyUI does not answer at "+c.base(), map[string]any{"reason": err.Error()})
}

// GetJSON reads a JSON answer into v.
func (c *Client) GetJSON(ctx context.Context, path string, timeout time.Duration, v any) error {
	data, err := c.open(ctx, http.MethodGet, path, nil, timeout)
	if err != nil {
		return err
	}
	return decode(data, v)
}

// PostJSON sends body as JSON and reads the answer into v. An empty answer is {}.
func (c *Client) PostJSON(ctx context.Context, path string, body any, timeout time.Duration, v any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	data, err := c.open(ctx, http.MethodPost, path, raw, timeout)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}
	if v == nil {
		return nil
	}
	return decode(data, v)
}

func decode(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(v)
}

func isHorus(err error) bool {
	var he *herr.Error
	return errors.As(err, &he)
}

// -- server -------------------------------------------------------------------

// IsUp reports if /system_stats answers. Only a BACKEND_UNAVAILABLE error
// means "not up"; any other error passes through, as in comfy.py.
func (c *Client) IsUp(ctx context.Context) (bool, error) {
	var v any
	err := c.GetJSON(ctx, "/system_stats", 3*time.Second, &v)
	if err == nil {
		return true, nil
	}
	if isHorus(err) {
		return false, nil
	}
	return false, err
}

// Address is the address of the server for this request, for the text of
// an error. The adapter starts no program: internal/backend starts ComfyUI.
func (c *Client) Address() string { return c.base() }

// Free unloads the models. Errors are ignored.
func (c *Client) Free(ctx context.Context) {
	_ = c.PostJSON(ctx, "/free", map[string]any{"unload_models": true, "free_memory": true}, 30*time.Second, nil)
}

// ObjectInfo reads /object_info.
func (c *Client) ObjectInfo(ctx context.Context) (graph.ObjectInfo, error) {
	var info graph.ObjectInfo
	data, err := c.open(ctx, http.MethodGet, "/object_info", nil, 60*time.Second)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return info, nil
}

// -- jobs ---------------------------------------------------------------------

// uuid4 is a random UUID (version 4): the client_id of a job.
func uuid4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Submit posts the graph with a new client_id and returns the prompt_id.
func (c *Client) Submit(ctx context.Context, api *graph.API) (string, error) {
	var reply catalog.Object
	err := c.PostJSON(ctx, "/prompt", map[string]any{"prompt": api, "client_id": uuid4()}, 60*time.Second, &reply)
	var he *HTTPError
	if errors.As(err, &he) {
		if json.Unmarshal(he.Body, &reply) != nil {
			return "", herr.New("GENERATION_FAILED", fmt.Sprintf("ComfyUI rejected the job with HTTP %d", he.Code),
				map[string]any{"backend_message": headRunes(string(he.Body), 1500)})
		}
	} else if err != nil {
		return "", err
	}
	if graph.Truthy(reply.Raw("node_errors")) || graph.Truthy(reply.Raw("error")) {
		return "", RejectedGraphError(&reply)
	}
	var id any
	_ = reply.Decode("prompt_id", &id)
	if s, ok := id.(string); ok && s != "" {
		return s, nil
	}
	return "", herr.New("INTERNAL", "ComfyUI returned no prompt_id", map[string]any{"reply": headRunes(graph.PyDumps(&reply), 500)})
}

// image is one output image of a job in /history.
type image struct {
	Filename  string `json:"filename"`
	Subfolder string `json:"subfolder"`
	Type      string `json:"type"`
}

// Entry is one job in /history.
type Entry struct {
	Status  Status          `json:"status"`
	Outputs *catalog.Object `json:"outputs"`
}

// Status is the status of a job in /history.
type Status struct {
	StatusStr string            `json:"status_str"`
	Completed bool              `json:"completed"`
	Messages  []json.RawMessage `json:"messages"`
}

// Images returns the images of all output nodes, in the order of the nodes.
func (e *Entry) Images() []graph.Image {
	var out []graph.Image
	if e.Outputs == nil {
		return out
	}
	for _, k := range e.Outputs.Keys() {
		var node struct {
			Images []image `json:"images"`
		}
		if json.Unmarshal(e.Outputs.Raw(k), &node) == nil {
			for _, img := range node.Images {
				out = append(out, graph.Image{Filename: img.Filename, Subfolder: img.Subfolder, Type: img.Type})
			}
		}
	}
	return out
}

// entry reads the job from /history, or nil when /history does not know it yet.
func (c *Client) entry(ctx context.Context, promptID string) (*Entry, error) {
	var all map[string]json.RawMessage
	if err := c.GetJSON(ctx, "/history/"+promptID, 30*time.Second, &all); err != nil {
		return nil, err
	}
	raw, ok := all[promptID]
	if !ok || string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil
	}
	var e Entry
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// History polls the job one time. It returns nil when /history does not know
// the job yet, and the horus error of JobError when the job failed. A job that
// is done has its images and its backend time.
func (c *Client) History(ctx context.Context, promptID string) (*graph.Job, error) {
	e, err := c.entry(ctx, promptID)
	if err != nil || e == nil {
		return nil, err
	}
	if e.Status.StatusStr == "error" {
		return nil, JobError(promptID, e.Status)
	}
	if !e.Status.Completed {
		return &graph.Job{}, nil
	}
	return &graph.Job{Done: true, Images: e.Images(), BackendSeconds: ExecutionSeconds(e)}, nil
}

// QueueState is "running", "pending:<position>" or "absent".
func (c *Client) QueueState(ctx context.Context, promptID string) (string, error) {
	var q struct {
		Running [][]any `json:"queue_running"`
		Pending [][]any `json:"queue_pending"`
	}
	if err := c.GetJSON(ctx, "/queue", 10*time.Second, &q); err != nil {
		return "", err
	}
	for _, item := range q.Running {
		if len(item) > 1 && item[1] == promptID {
			return "running", nil
		}
	}
	pending := append([][]any(nil), q.Pending...)
	sort.SliceStable(pending, func(i, j int) bool { return number(pending[i][0]) < number(pending[j][0]) })
	for i, item := range pending {
		if len(item) > 1 && item[1] == promptID {
			return fmt.Sprintf("pending:%d", i+1), nil
		}
	}
	return "absent", nil
}

func number(v any) float64 {
	if n, ok := v.(json.Number); ok {
		f, _ := n.Float64()
		return f
	}
	return 0
}

// Cancel stops the running job or removes the pending job. Errors are ignored.
func (c *Client) Cancel(ctx context.Context, promptID string) {
	state, err := c.QueueState(ctx, promptID)
	if err != nil {
		return
	}
	if state == "running" {
		_ = c.PostJSON(ctx, "/interrupt", map[string]any{"prompt_id": promptID}, 10*time.Second, nil)
	} else if strings.HasPrefix(state, "pending") {
		_ = c.PostJSON(ctx, "/queue", map[string]any{"delete": []string{promptID}}, 10*time.Second, nil)
	}
}

// View downloads an output image.
func (c *Client) View(ctx context.Context, img graph.Image) ([]byte, error) {
	t := img.Type
	if t == "" {
		t = "output"
	}
	q := url.Values{"filename": {img.Filename}, "subfolder": {img.Subfolder}, "type": {t}}
	return c.open(ctx, http.MethodGet, "/view?"+q.Encode(), nil, 120*time.Second)
}

// headRunes is s[:n] of Python: the first n characters.
func headRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

// tailRunes is s[-n:] of Python: the last n characters.
func tailRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[len(r)-n:])
}

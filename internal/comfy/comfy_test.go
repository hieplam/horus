// Tests of the ComfyUI client and the job error mapping.
package comfy

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/comfy/comfytest"
	"github.com/hieplam/horus/internal/graph"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
)

func fixturePath() string { return filepath.Join("..", "..", "tests", "fixtures", "object_info.json") }

func readFile(t *testing.T, parts ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func codeOf(err error) string {
	var he *herr.Error
	if errors.As(err, &he) {
		return he.Code
	}
	return ""
}

func status(t *testing.T, s string) jobStatus {
	t.Helper()
	var st jobStatus
	if err := json.Unmarshal([]byte(s), &st); err != nil {
		t.Fatal(err)
	}
	return st
}

func execError(etype, emsg string) string {
	b, _ := json.Marshal(map[string]any{"status_str": "error", "messages": []any{
		[]any{"execution_start", map[string]any{"timestamp": 1}},
		[]any{"execution_error", map[string]any{"node_type": "KSampler", "exception_type": etype, "exception_message": emsg}}}})
	return string(b)
}

// TestJobErrors has one case for each mapped job error of job_error.
func TestJobErrors(t *testing.T) {
	for _, c := range []struct {
		name, status, code string
		retryable          bool
		actionBy           string
	}{
		{"interrupted", `{"status_str": "error", "messages": [["execution_interrupted", {}]]}`, "CANCELLED", true, "retry"},
		{"torch OutOfMemoryError", execError("torch.OutOfMemoryError", "CUDA error"), "OUT_OF_MEMORY", true, "caller"},
		{"out of memory text", execError("RuntimeError", "CUDA out of memory. Tried to allocate 2.00 GiB"), "OUT_OF_MEMORY", true, "caller"},
		{"Out Of Memory spaced", execError("Out Of Memory Error", ""), "OUT_OF_MEMORY", true, "caller"},
		{"allocation on device", execError("RuntimeError", "Allocation on device 0 would exceed allowed memory"), "OUT_OF_MEMORY", true, "caller"},
		{"old driver", execError("RuntimeError", "CUDA driver version is insufficient for CUDA runtime version"), "GENERATION_FAILED", false, "retry"},
		{"other failure", execError("ValueError", "bad latent"), "GENERATION_FAILED", true, "retry"},
		{"no message", `{"status_str": "error", "messages": []}`, "GENERATION_FAILED", true, "retry"},
	} {
		t.Run(c.name, func(t *testing.T) {
			info := jobError("job-1", status(t, c.status)).Info()
			if info.Code != c.code || info.Retryable != c.retryable || info.ActionBy != c.actionBy {
				t.Fatalf("got %s retryable=%v action_by=%s", info.Code, info.Retryable, info.ActionBy)
			}
			if info.Details["job_id"] != "job-1" {
				t.Fatalf("details: %v", info.Details)
			}
		})
	}
	info := jobError("j", status(t, execError("RuntimeError", "CUDA driver version is insufficient"))).Info()
	if !strings.Contains(info.NextStep, "update the NVIDIA driver") {
		t.Fatalf("next_step: %s", info.NextStep)
	}
	long := strings.Repeat("a", 1000) + strings.Repeat("b", 1500)
	info = jobError("j", status(t, execError("ValueError", long))).Info()
	if info.Details["backend_message"] != strings.Repeat("b", 1500) {
		t.Fatal("backend_message must be the last 1500 characters")
	}
	if info.Details["node_type"] != "KSampler" || info.Details["exception_type"] != "ValueError" {
		t.Fatalf("details: %v", info.Details)
	}
}

func TestRejectedGraphError(t *testing.T) {
	var reply catalog.Object
	_ = json.Unmarshal([]byte(`{"error": {"type": "x"}, "node_errors": {"1": {"errors": [
		{"type": "value_not_in_list", "extra_info": {"received_value": "Gone.SAFETENSORS"}},
		{"type": "value_not_in_list", "extra_info": {"received_value": "euler"}}]}}}`), &reply)
	e := rejectedGraphError(&reply)
	if e.Code != "WORKFLOW_NOT_INSTALLED" || !reflect.DeepEqual(e.Details["missing"], []string{"Gone.SAFETENSORS"}) {
		t.Fatalf("got %v %v", e, e.Details)
	}
	_ = json.Unmarshal([]byte(`{"error": {"type": "prompt_no_outputs", "message": "Prompt has no outputs"}, "node_errors": []}`), &reply)
	info := rejectedGraphError(&reply).Info()
	if info.Code != "GENERATION_FAILED" || info.Retryable || !strings.Contains(info.NextStep, "defect of the workflow") ||
		info.Details["backend_message"] != `{"error": {"type": "prompt_no_outputs", "message": "Prompt has no outputs"}, "node_errors": []}` {
		t.Fatalf("got %+v", info)
	}
}

func TestExecutionSeconds(t *testing.T) {
	e := &historyEntry{Status: status(t, `{"messages": [["execution_start", {"timestamp": 1000}], ["execution_success", {"timestamp": 4550}]]}`)}
	if got := executionSeconds(e); got == nil || *got != 3.5 {
		t.Fatalf("got %v", got) // 3.55 is 3.5499... in binary, as in Python
	}
	e = &historyEntry{Status: status(t, `{"messages": [["execution_start", {"timestamp": 1000}]]}`)}
	if executionSeconds(e) != nil {
		t.Fatal("no execution_success must give nil")
	}
}

func TestClientJobCalls(t *testing.T) {
	f := comfytest.New(t, fixturePath())
	c := New(f.URL() + "/")
	ctx := context.Background()
	if c.URL != f.URL() {
		t.Fatalf("URL keeps a trailing slash: %s", c.URL)
	}
	if up, err := c.IsUp(ctx); !up || err != nil {
		t.Fatalf("IsUp: %v %v", up, err)
	}
	info, err := c.ObjectInfo(ctx)
	if err != nil || len(info) != 10 {
		t.Fatalf("ObjectInfo: %d %v", len(info), err)
	}
	api, err := graph.UIToAPI(readFile(t, "..", "..", "workflows", "it-illustration", "workflow.json"), info)
	if err != nil {
		t.Fatal(err)
	}
	id, err := c.Submit(ctx, api)
	if err != nil || id != "job-1" || f.Job(id).Graph["3394534790677934"] == nil {
		t.Fatalf("Submit: %s %v", id, err)
	}
	if e, err := c.History(ctx, id); e != nil || err != nil {
		t.Fatalf("first poll must be empty: %v %v", e, err)
	}
	e, err := c.History(ctx, id)
	if err != nil || e == nil || !e.Done || len(e.Images) != 1 || e.Images[0].Filename != "job-1.png" || e.Images[0].Type != "output" {
		t.Fatalf("History: %+v %v", e, err)
	}
	if e.BackendSeconds == nil || *e.BackendSeconds != 3.5 {
		t.Fatalf("backend seconds of a done job: %v", e.BackendSeconds)
	}
	png, err := c.View(ctx, e.Images[0])
	if err != nil || string(png[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("View: %v", err)
	}
	if s, _ := c.QueueState(ctx, id); !s.Absent() {
		t.Fatalf("QueueState of a done job: %+v", s)
	}

	f.SetMode(comfytest.Slow)
	slow, _ := c.Submit(ctx, api)
	if e, err := c.History(ctx, slow); err != nil || e != nil {
		t.Fatalf("a running job that /history does not list yet: %+v %v", e, err)
	}
	f.SetMode(comfytest.Pending)
	pending, _ := c.Submit(ctx, api)
	if s, _ := c.QueueState(ctx, slow); !s.Running {
		t.Fatalf("slow: %+v", s)
	}
	if s, _ := c.QueueState(ctx, pending); s != (graph.Queue{Position: 2}) {
		t.Fatalf("pending: %+v", s)
	}
	c.Cancel(ctx, slow)
	c.Cancel(ctx, pending)
	if !reflect.DeepEqual(f.Interrupts(), []string{slow}) || !reflect.DeepEqual(f.Deleted(), []string{pending}) {
		t.Fatalf("interrupts %v deleted %v", f.Interrupts(), f.Deleted())
	}
	c.Free(ctx)
	if f.Frees() != 1 {
		t.Fatal("Free sent no POST /free")
	}

	f.SetMode(comfytest.Reject)
	_, err = c.Submit(ctx, api)
	if codeOf(err) != "WORKFLOW_NOT_INSTALLED" {
		t.Fatalf("a rejected graph with a model file: %v", err)
	}
}

// TestJobErrorOfTheMPSOutOfMemoryText: the text of PyTorch when a Mac runs out
// of memory on MPS ("MPS backend out of memory ...") gives OUT_OF_MEMORY,
// retryable, action_by caller, and the backend message is kept.
func TestJobErrorOfTheMPSOutOfMemoryText(t *testing.T) {
	const text = "MPS backend out of memory (MPS allocated: 9.07 GB, other allocations: 6.50 GB, max allowed: 18.13 GB). " +
		"Tried to allocate 256.00 MB on private pool. Use PYTORCH_MPS_HIGH_WATERMARK_RATIO=0.0 to disable upper limit " +
		"for memory allocations (may cause system failure)."
	info := jobError("job-1", status(t, execError("RuntimeError", text))).Info()
	if info.Code != "OUT_OF_MEMORY" || !info.Retryable || info.ActionBy != "caller" {
		t.Fatalf("got %s retryable=%v action_by=%s", info.Code, info.Retryable, info.ActionBy)
	}
	if msg, _ := info.Details["backend_message"].(string); !strings.HasPrefix(msg, "MPS backend out of memory") {
		t.Fatalf("backend_message = %q", msg)
	}
}

// TestHistoryOfAFailedJobIsItsError: History maps a job with status "error"
// to the horus error of jobError, here OUT_OF_MEMORY. The service
// internal/generate returns that error as it is.
func TestHistoryOfAFailedJobIsItsError(t *testing.T) {
	f := comfytest.New(t, fixturePath())
	c := New(f.URL())
	ctx := context.Background()
	f.SetMode(comfytest.OOM)
	id, err := c.Submit(ctx, &graph.API{})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = c.History(ctx, id) // the first poll is empty
	e, err := c.History(ctx, id)
	var he *herr.Error
	if e != nil || !errors.As(err, &he) {
		t.Fatalf("got %+v %v", e, err)
	}
	if i := he.Info(); i.Code != "OUT_OF_MEMORY" || !i.Retryable || i.ActionBy != "caller" || i.Details["job_id"] != id {
		t.Fatalf("got %+v", i)
	}

	f.SetMode(comfytest.NoImage)
	id, _ = c.Submit(ctx, &graph.API{})
	_, _ = c.History(ctx, id)
	if e, err := c.History(ctx, id); err != nil || e == nil || !e.Done || len(e.Images) != 0 {
		t.Fatalf("a done job without an image: %+v %v", e, err)
	}
}

// TestCancelledCallGivesTheContextError: a request of a cancelled call
// returns the error of the context, not BACKEND_UNAVAILABLE.
func TestCancelledCallGivesTheContextError(t *testing.T) {
	f := comfytest.New(t, fixturePath())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(f.URL()).History(ctx, "job-1"); !errors.Is(err, context.Canceled) || codeOf(err) != "" {
		t.Fatalf("got %v", err)
	}
}

// TestEmptyURLReadsTheSettingAtEachRequest: the client that cmd/horus makes
// at the start follows HORUS_COMFYUI_URL, as comfy.New("") at each call did.
func TestEmptyURLReadsTheSettingAtEachRequest(t *testing.T) {
	f := comfytest.New(t, fixturePath())
	c := New("")
	useSettings(t, map[string]string{"HORUS_COMFYUI_URL": freeURL(t)})
	if up, err := c.IsUp(context.Background()); up || err != nil {
		t.Fatalf("nothing listens: %v %v", up, err)
	}
	useSettings(t, map[string]string{"HORUS_COMFYUI_URL": f.URL() + "/"})
	if up, err := c.IsUp(context.Background()); !up || err != nil {
		t.Fatalf("the fake listens: %v %v", up, err)
	}
	if got := c.Address(); got != f.URL() {
		t.Errorf("Address = %s, want %s", got, f.URL())
	}
	if got := New("http://127.0.0.1:9/").Address(); got != "http://127.0.0.1:9" {
		t.Errorf("Address of a fixed URL = %s", got)
	}
}

func TestSubmitHTTPErrorWithoutJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer srv.Close()
	_, err := New(srv.URL).Submit(context.Background(), &graph.API{})
	if codeOf(err) != "GENERATION_FAILED" || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("got %v", err)
	}
}

func TestSubmitWithoutPromptID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"number": 1}`))
	}))
	defer srv.Close()
	_, err := New(srv.URL).Submit(context.Background(), &graph.API{})
	if codeOf(err) != "INTERNAL" {
		t.Fatalf("got %v", err)
	}
}

// TestVersion is the answer that horus doctor reads (the check "ComfyUI
// server"): the version of a server that answers, and an error for a server
// that answers with an error or with no JSON, and for no server.
func TestVersion(t *testing.T) {
	server := func(status int, answer string) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/system_stats" {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(answer))
		}))
		t.Cleanup(srv.Close)
		return srv.URL
	}
	ctx := context.Background()
	if v, err := New(server(200, `{"system":{"comfyui_version":"0.9.9"}}`) + "/").Version(ctx); err != nil || v != "0.9.9" {
		t.Fatalf("Version = %q, %v", v, err)
	}
	if _, err := New(server(500, `{}`)).Version(ctx); err == nil {
		t.Error("HTTP 500 gave no error")
	}
	if _, err := New(server(200, "not json")).Version(ctx); err == nil {
		t.Error("an answer that is not JSON gave no error")
	}
	if _, err := New(freeURL(t)).Version(ctx); err == nil {
		t.Error("no server gave no error")
	}
}

// freeURL is the address of a port where nothing listens.
func freeURL(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	u := "http://" + l.Addr().String()
	l.Close()
	return u
}

func TestBackendDown(t *testing.T) {
	c := New(freeURL(t))
	ctx := context.Background()
	if up, err := c.IsUp(ctx); up || err != nil {
		t.Fatalf("IsUp: %v %v", up, err)
	}
	if _, err := c.History(ctx, "x"); codeOf(err) != "BACKEND_UNAVAILABLE" {
		t.Fatalf("History: %v", err)
	}
	c.Cancel(ctx, "x") // no panic, no error
	c.Free(ctx)
}

func useSettings(t *testing.T, env map[string]string) {
	t.Helper()
	t.Setenv("HORUS_ROOT", t.TempDir()) // no .env file
	for k, v := range env {
		t.Setenv(k, v)
	}
	settings.Reset()
	t.Cleanup(settings.Reset)
}

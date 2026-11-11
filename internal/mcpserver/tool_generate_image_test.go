// Tests of the tool generate_image as a handler, with a fake of the image
// service. The same tool through the wiring (the real service, the real
// ComfyUI adapter and the fake ComfyUI of comfytest) is tested in cmd/horus
// (image_test.go). Part of the port of horus/server.py.
package mcpserver

import (
	"context"
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/herr"
)

const testSubject = "On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt."

// fakeImages is a fake of the image service. Generate records the request,
// sends one progress report and returns result or err.
type fakeImages struct {
	mu      sync.Mutex
	running bool
	nFrees  int
	reqs    []generate.Request
	result  *generate.Result
	err     error
}

func (f *fakeImages) Generate(ctx context.Context, r generate.Request, o generate.Options) (*generate.Result, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, r)
	res, err := f.result, f.err
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	o.Report(ctx, 0, 0, "job "+res.JobID+" submitted")
	return res, nil
}

func (f *fakeImages) Running(ctx context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running, nil
}

func (f *fakeImages) FreeMemory(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nFrees++
}

func (f *fakeImages) setRunning(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running = v
}

func (f *fakeImages) frees() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nFrees
}

func (f *fakeImages) requests() []generate.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]generate.Request(nil), f.reqs...)
}

// TestGenerateImageToolCallsTheService: the arguments become the request,
// the result is the envelope (the seed 2^64-1 comes back exactly), and a
// report of the service is a progress notification only with a progressToken.
func TestGenerateImageToolCallsTheService(t *testing.T) {
	useRepoWorkflows(t)
	secs := 3.5
	images := &fakeImages{result: &generate.Result{Workflow: "it-illustration", Path: "/x/a.png", Bytes: 8, Width: 1024, Height: 768,
		Seed: 1<<64 - 1, Seconds: 1.5, BackendSeconds: &secs, JobID: "job-1"}}
	got := make(chan string, 16)
	cs := connect(t, New(images, nil), &mcp.ClientOptions{ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
		got <- r.Params.Message
	}})
	params := &mcp.CallToolParams{Name: "generate_image", Arguments: json.RawMessage(`{"workflow": "it-illustration", "subject": "` +
		testSubject + `", "out_path": "/x/a.png", "seed": 18446744073709551615, "width": 1024, "overwrite": true}`)}
	params.SetProgressToken("tok-1")
	res, err := cs.CallTool(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	env := envelopeOf(t, res)
	r, _ := env["result"].(map[string]any)
	if res.IsError || env["ok"] != true || r["path"] != "/x/a.png" || r["job_id"] != "job-1" || r["backend_seconds"] != 3.5 {
		t.Fatalf("got %v", env)
	}
	if text := res.Content[0].(*mcp.TextContent).Text; !strings.Contains(text, `"seed": 18446744073709551615`) {
		t.Fatalf("the seed 2^64-1 must come back exactly: %s", text)
	}
	reqs := images.requests()
	maxSeed, _ := new(big.Int).SetString("18446744073709551615", 10)
	if len(reqs) != 1 {
		t.Fatalf("requests: %v", reqs)
	}
	q := reqs[0]
	if q.Workflow != "it-illustration" || q.Subject != testSubject || q.OutPath == nil || *q.OutPath != "/x/a.png" ||
		!reflect.DeepEqual(q.Seed, maxSeed) || q.Width != int64(1024) || q.Height != nil || !q.Overwrite || q.Steps != nil {
		t.Fatalf("request: %+v", q)
	}
	// The client can run its notification handler after CallTool returns, so
	// the test waits for the notification (with a time limit).
	if m := nextProgress(t, got); m != "job job-1 submitted" {
		t.Fatalf("progress with a token: %q", m)
	}

	// Without a progressToken: no notification. Arguments not given are nil.
	res = callTool(t, cs, "generate_image", map[string]any{"workflow": "it-illustration", "subject": testSubject})
	if res.IsError {
		t.Fatalf("got %v", envelopeOf(t, res))
	}
	if q := images.requests()[1]; q.OutPath != nil || q.Seed != nil || q.Width != nil || q.Overwrite {
		t.Fatalf("defaults: %+v", q)
	}
	// The client runs its handlers in the order of arrival. A third call with a
	// token and another job id follows the second call. If the second call had
	// sent a notification, it would come before the third one.
	images.result.JobID = "job-3"
	params3 := &mcp.CallToolParams{Name: "generate_image", Arguments: map[string]any{"workflow": "it-illustration", "subject": testSubject}}
	params3.SetProgressToken("tok-3")
	if _, err := cs.CallTool(context.Background(), params3); err != nil {
		t.Fatal(err)
	}
	if m := nextProgress(t, got); m != "job job-3 submitted" {
		t.Fatalf("the next notification must be the one of the third call; the call without a token sent %q", m)
	}
	if len(got) != 0 {
		t.Fatalf("more notifications than expected: %d", len(got))
	}
}

// nextProgress returns the next progress message of ch. It fails the test
// when none arrives in 10 seconds. Callers hold no lock.
func nextProgress(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case m := <-ch:
		return m
	case <-time.After(10 * time.Second):
		t.Fatal("no progress notification in 10 seconds")
		return ""
	}
}

// TestGenerateImageToolReturnsTheServiceError: an error of the service is
// the error envelope, with its code and action_by.
func TestGenerateImageToolReturnsTheServiceError(t *testing.T) {
	useRepoWorkflows(t)
	images := &fakeImages{err: herr.New("BACKEND_UNAVAILABLE", "ComfyUI does not answer", nil)}
	cs := connect(t, New(images, nil), nil)
	res := callTool(t, cs, "generate_image", map[string]any{"workflow": "it-illustration", "subject": testSubject})
	e, _ := envelopeOf(t, res)["error"].(map[string]any)
	if !res.IsError || e["code"] != "BACKEND_UNAVAILABLE" || e["action_by"] != "human" {
		t.Fatalf("got %v", e)
	}
}

// Tests of the image path through the wiring of cmd/horus: the MCP server,
// the image service and the real ComfyUI adapter, against the fake ComfyUI of
// comfytest on an ephemeral port. The handler and the service have their own
// tests with fakes (internal/mcpserver, internal/generate). These tests prove
// that the parts fit: the wire format, the cancel of a job, the error codes.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/comfy/comfytest"
	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/mcpserver"
	"github.com/hieplam/horus/internal/settings"
)

const testSubject = "On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt."

// connect starts the server and a client over an in-memory transport.
func connect(t *testing.T, s *mcp.Server, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, opts).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func useRepoWorkflows(t *testing.T) {
	t.Helper()
	root, _ := filepath.Abs(filepath.Join("..", "..", "workflows"))
	t.Setenv("HORUS_WORKFLOWS_DIR", root)
	settings.Reset()
	t.Cleanup(settings.Reset)
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// envelopeOf reads the envelope from the text content of a tool result.
func envelopeOf(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &env); err != nil {
		t.Fatal(err)
	}
	return env
}

// imageEnv starts a fake ComfyUI, points the settings at it and at a
// temporary folder, and makes the wait intervals short.
func imageEnv(t *testing.T) *comfytest.Fake {
	t.Helper()
	useRepoWorkflows(t)
	f := comfytest.New(t, filepath.Join("..", "..", "tests", "fixtures", "object_info.json"))
	tmp := t.TempDir()
	t.Setenv("HORUS_ROOT", tmp)
	t.Setenv("HORUS_COMFYUI_URL", f.URL())
	t.Setenv("HORUS_AUTOSTART", "0")
	t.Setenv("HORUS_OUTPUT_DIR", filepath.Join(tmp, "out"))
	settings.Reset()
	oldPoll, oldReport, oldAbsent := generate.PollInterval, generate.ReportInterval, generate.AbsentLimit
	generate.PollInterval, generate.ReportInterval, generate.AbsentLimit = 5*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond
	t.Cleanup(func() {
		generate.PollInterval, generate.ReportInterval, generate.AbsentLimit = oldPoll, oldReport, oldAbsent
	})
	return f
}

// TestImageTimeoutThroughTheAdapter: over the time limit, the adapter
// interrupts a running job and deletes a pending job on the server.
func TestImageTimeoutThroughTheAdapter(t *testing.T) {
	f := imageEnv(t)
	svc := newImages("")
	o := generate.Options{Timeout: 150 * time.Millisecond}
	f.SetMode(comfytest.Slow)
	_, err := svc.Generate(context.Background(), generate.Request{Workflow: "it-illustration", Subject: testSubject}, o)
	var he *herr.Error
	if !errors.As(err, &he) || he.Code != "GENERATION_TIMEOUT" || !reflect.DeepEqual(f.Interrupts(), []string{"job-1"}) {
		t.Fatalf("running: %v interrupts=%v", err, f.Interrupts())
	}
	f.SetMode(comfytest.Pending)
	_, err = svc.Generate(context.Background(), generate.Request{Workflow: "it-illustration", Subject: testSubject}, o)
	if !errors.As(err, &he) || he.Code != "GENERATION_TIMEOUT" || !reflect.DeepEqual(f.Deleted(), []string{"job-2"}) {
		t.Fatalf("pending: %v deleted=%v", err, f.Deleted())
	}
}

// TestImageCancelThroughTheAdapter: a cancelled call gives context.Canceled
// and interrupts the running job on the server.
func TestImageCancelThroughTheAdapter(t *testing.T) {
	f := imageEnv(t)
	f.SetMode(comfytest.Slow)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for len(f.Jobs()) == 0 || f.Polls(f.Jobs()[0]) == 0 {
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()
	_, err := newImages("").Generate(ctx, generate.Request{Workflow: "it-illustration", Subject: testSubject}, generate.Options{Timeout: 5 * time.Second})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(f.Interrupts(), []string{"job-1"}) {
		t.Fatalf("got %v interrupts=%v", err, f.Interrupts())
	}
}

// TestImageOutOfMemoryThroughTheAdapter: a CUDA out-of-memory error in
// /history becomes OUT_OF_MEMORY, retryable, action_by caller.
func TestImageOutOfMemoryThroughTheAdapter(t *testing.T) {
	f := imageEnv(t)
	f.SetMode(comfytest.OOM)
	_, err := newImages("").Generate(context.Background(), generate.Request{Workflow: "it-illustration", Subject: testSubject}, generate.Options{})
	var he *herr.Error
	if !errors.As(err, &he) {
		t.Fatalf("not a horus error: %v", err)
	}
	if i := he.Info(); i.Code != "OUT_OF_MEMORY" || !i.Retryable || i.ActionBy != "caller" {
		t.Fatalf("got %+v", i)
	}
}

func TestGenerateImageTool(t *testing.T) {
	useRepoWorkflows(t)
	f := comfytest.New(t, filepath.Join("..", "..", "tests", "fixtures", "object_info.json"))
	tmp := t.TempDir()
	t.Setenv("HORUS_ROOT", tmp)
	t.Setenv("HORUS_COMFYUI_URL", f.URL())
	t.Setenv("HORUS_AUTOSTART", "0")
	t.Setenv("HORUS_OUTPUT_DIR", filepath.Join(tmp, "out"))
	settings.Reset()
	old := generate.PollInterval
	generate.PollInterval = 5 * time.Millisecond
	t.Cleanup(func() { generate.PollInterval = old })

	var mu sync.Mutex
	var got []string
	cs := connect(t, mcpserver.New(newImages("")), &mcp.ClientOptions{ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
		mu.Lock()
		got = append(got, r.Params.Message)
		mu.Unlock()
	}})
	out := filepath.Join(tmp, "images", "a.png")
	params := &mcp.CallToolParams{Name: "generate_image", Arguments: json.RawMessage(
		`{"workflow": "it-illustration", "subject": "` + testSubject + `", "out_path": ` + quote(out) + `, "seed": 18446744073709551615}`)}
	params.SetProgressToken("tok-1")
	res, err := cs.CallTool(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	env := envelopeOf(t, res)
	r, _ := env["result"].(map[string]any)
	if res.IsError || env["ok"] != true || r["path"] != out || r["job_id"] != "job-1" {
		t.Fatalf("got %v", env)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !json.Valid([]byte(text)) || !strings.Contains(text, `"seed": 18446744073709551615`) {
		t.Fatalf("the seed 2^64-1 must come back exactly: %s", text)
	}
	data, err := os.ReadFile(out)
	if err != nil || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("no PNG: %v", err)
	}
	mu.Lock()
	n := len(got)
	first := ""
	if n > 0 {
		first = got[0]
	}
	mu.Unlock()
	if n == 0 || first != "job job-1 submitted" {
		t.Fatalf("progress with a token: %v", got)
	}

	// Without a progressToken: no notification.
	mu.Lock()
	got = nil
	mu.Unlock()
	res = callTool(t, cs, "generate_image", map[string]any{"workflow": "it-illustration", "subject": testSubject})
	if res.IsError {
		t.Fatalf("got %v", envelopeOf(t, res))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 0 {
		t.Fatalf("progress without a token: %v", got)
	}
}

func TestGenerateImageToolErrors(t *testing.T) {
	useRepoWorkflows(t)
	t.Setenv("HORUS_ROOT", t.TempDir())
	t.Setenv("HORUS_COMFYUI_URL", "http://127.0.0.1:1")
	t.Setenv("HORUS_AUTOSTART", "0")
	settings.Reset()
	cs := connect(t, mcpserver.New(newImages("")), nil)
	for _, c := range []struct {
		args map[string]any
		code string
	}{
		{map[string]any{"workflow": "nope", "subject": testSubject}, "WORKFLOW_NOT_FOUND"},
		{map[string]any{"workflow": "it-illustration", "subject": "a box"}, "INVALID_ARGUMENT"},
		{map[string]any{"workflow": "it-illustration", "subject": testSubject, "seed": -1}, "INVALID_ARGUMENT"},
		{map[string]any{"workflow": "it-illustration", "subject": testSubject}, "BACKEND_UNAVAILABLE"},
	} {
		res := callTool(t, cs, "generate_image", c.args)
		e, _ := envelopeOf(t, res)["error"].(map[string]any)
		if !res.IsError || e["code"] != c.code {
			t.Errorf("%v: %v", c.args, e)
		}
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

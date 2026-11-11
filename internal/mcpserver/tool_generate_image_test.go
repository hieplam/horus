// Tests of the tool generate_image through MCP, against a fake ComfyUI on an
// ephemeral port. Part of the port of horus/server.py.
package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/comfy/comfytest"
	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/settings"
)

const testSubject = "On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt."

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
	cs := connect(t, New(), &mcp.ClientOptions{ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
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
	cs := connect(t, New(), nil)
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

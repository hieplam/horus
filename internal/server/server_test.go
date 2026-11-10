// Tests of the port of horus/server.py: the tool list against the golden file
// made from the Python server, and the SDK features that the port needs.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/settings"
)

type golden struct {
	Server       string           `json:"server"`
	Version      string           `json:"version"`
	Instructions string           `json:"instructions"`
	Tools        []map[string]any `json:"tools"`
}

func readGolden(t *testing.T) golden {
	t.Helper()
	raw, err := os.ReadFile("testdata/tools.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// connect starts the Go server and a client over an in-memory transport.
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
	var b bytes.Buffer
	logTo(&b)
	t.Cleanup(func() { logTo(os.Stderr) })
}

func asJSON(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestToolsMatchPython compares tools/list of the Go server, as a client sees
// it, with tools/list of the Python server: for each tool the name, title,
// description, annotations, input schema and output schema. It also compares
// the server name, version and instructions.
func TestToolsMatchPython(t *testing.T) {
	useRepoWorkflows(t)
	g := readGolden(t)
	cs := connect(t, New(), nil)

	init := cs.InitializeResult()
	if init.ServerInfo.Name != g.Server || init.ServerInfo.Version != g.Version {
		t.Errorf("server info %+v, Python %s %s", init.ServerInfo, g.Server, g.Version)
	}
	if init.Instructions != g.Instructions {
		t.Errorf("instructions differ\n got %q\nwant %q", init.Instructions, g.Instructions)
	}

	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]any{}
	for _, tl := range list.Tools {
		got[tl.Name] = asJSON(t, tl)
	}
	if len(got) != len(g.Tools) || len(got) != 5 {
		t.Fatalf("Go has %d tools, Python %d, want 5", len(got), len(g.Tools))
	}
	for _, py := range g.Tools {
		name, _ := py["name"].(string)
		goTool, ok := got[name].(map[string]any)
		if !ok {
			t.Errorf("tool %s is missing in Go", name)
			continue
		}
		for _, key := range []string{"inputSchema", "outputSchema", "annotations", "title", "description", "name"} {
			if !reflect.DeepEqual(goTool[key], py[key]) {
				gj, _ := json.Marshal(goTool[key])
				pj, _ := json.Marshal(py[key])
				t.Errorf("%s: %s differs\n  Go %s\nPython %s", name, key, gj, pj)
			}
		}
		for key := range goTool {
			if _, ok := py[key]; !ok {
				t.Errorf("%s: Go has the key %s, Python has not", name, key)
			}
		}
	}
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func envelopeOf(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	sc, ok := asJSON(t, res.StructuredContent).(map[string]any)
	if !ok {
		t.Fatalf("no structuredContent: %+v", res)
	}
	var text map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &text); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sc, text) {
		t.Fatalf("structuredContent and text differ:\n%v\n%v", sc, text)
	}
	return sc
}

// TestErrorEnvelope checks structuredContent with isError (an SDK feature),
// the text form with an indent of 1, and the key order of the envelope.
func TestErrorEnvelope(t *testing.T) {
	useRepoWorkflows(t)
	cs := connect(t, New(), nil)
	res := callTool(t, cs, "describe_workflow", map[string]any{"workflow": "nope"})
	if !res.IsError {
		t.Fatal("isError is not set")
	}
	env := envelopeOf(t, res)
	e, _ := env["error"].(map[string]any)
	if env["ok"] != false || env["result"] != nil || e["code"] != "WORKFLOW_NOT_FOUND" {
		t.Fatalf("envelope %v", env)
	}
	if known, _ := e["details"].(map[string]any)["known"].([]any); len(known) != 1 || known[0] != "it-illustration" {
		t.Fatalf("known = %v", e["details"])
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.HasPrefix(text, "{\n \"ok\": false,\n \"result\": null,\n \"error\": {\n  \"code\": \"WORKFLOW_NOT_FOUND\",\n  \"message\": \"no workflow named 'nope'\",\n  \"retryable\": false,\n  \"action_by\": \"caller\",") {
		t.Fatalf("text form differs from json.dumps(indent=1):\n%s", text)
	}

	res = callTool(t, cs, "uninstall_workflow", map[string]any{"workflow": "bad name!"})
	e, _ = envelopeOf(t, res)["error"].(map[string]any)
	if e["code"] != "INVALID_ARGUMENT" || e["details"].(map[string]any)["field"] != "workflow" {
		t.Fatalf("bad name: %v", e)
	}
}

// TestNotPortedToolsAnswerInternal checks that each tool part that is not
// ported yet answers with the INTERNAL envelope and not with a crash.
func TestNotPortedToolsAnswerInternal(t *testing.T) {
	useRepoWorkflows(t)
	cs := connect(t, New(), nil)
	calls := map[string]map[string]any{
		"list_workflows":     {},
		"describe_workflow":  {"workflow": "it-illustration"},
		"install_workflow":   {"workflow": "it-illustration"},
		"uninstall_workflow": {"workflow": "it-illustration"},
	}
	for name, args := range calls {
		res := callTool(t, cs, name, args)
		e, _ := envelopeOf(t, res)["error"].(map[string]any)
		if !res.IsError || e["code"] != "INTERNAL" || e["action_by"] != "human" {
			t.Errorf("%s: %v", name, e)
		}
		if strings.Contains(res.Content[0].(*mcp.TextContent).Text, "workflow.json") {
			t.Errorf("%s names the graph file", name)
		}
	}
}

// TestArgumentErrors checks the answer to arguments that fail the schema:
// isError, no structuredContent, and the field in the text, as pydantic does.
func TestArgumentErrors(t *testing.T) {
	useRepoWorkflows(t)
	cs := connect(t, New(), nil)
	for _, c := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{}, "workflow\n  Field required"},
		{map[string]any{"workflow": 5}, "workflow\n  Input should be a valid string"},
	} {
		res := callTool(t, cs, "describe_workflow", c.args)
		text := res.Content[0].(*mcp.TextContent).Text
		if !res.IsError || res.StructuredContent != nil || !strings.HasPrefix(text, "Error executing tool describe_workflow: 1 validation error for describe_workflowArguments") || !strings.Contains(text, c.want) {
			t.Errorf("%v: isError=%v sc=%v text=%q", c.args, res.IsError, res.StructuredContent, text)
		}
	}
	a, problems := parseArgs(json.RawMessage(`{"workflow":"w","subject":"s","seed":7.0,"width":null,"extra":1}`), generateImage().args)
	if len(problems) > 0 || *a.intPtr("seed") != 7 || a.intPtr("width") != nil || a.boolean("overwrite") || a.strPtr("out_path") != nil {
		t.Fatalf("args %v, problems %v", a, problems)
	}
	if _, problems := parseArgs(json.RawMessage(`{"workflow":"w","subject":"s","seed":7.5,"overwrite":"yes"}`), generateImage().args); len(problems) != 2 {
		t.Fatalf("problems %v", problems)
	}
}

// TestSDKProgressNotifications checks the SDK feature that G2 and G5 need: a
// tool sends progress notifications when the caller sends a progressToken,
// and sends none without a token.
func TestSDKProgressNotifications(t *testing.T) {
	useRepoWorkflows(t)
	var mu sync.Mutex
	var got []string
	s := mcp.NewServer(&mcp.Implementation{Name: "horus", Version: Version}, nil)
	s.AddTool(&mcp.Tool{Name: "slow", InputSchema: json.RawMessage(`{"type":"object"}`)},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			report := reporter(req)
			report(ctx, 1, 2, "half")
			report(ctx, 2, 2, "done")
			return okResult(map[string]any{}), nil
		})
	cs := connect(t, s, &mcp.ClientOptions{ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
		mu.Lock()
		got = append(got, r.Params.Message)
		mu.Unlock()
	}})
	params := &mcp.CallToolParams{Name: "slow", Arguments: map[string]any{}}
	params.SetProgressToken("tok-1")
	if _, err := cs.CallTool(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "slow", Arguments: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n >= 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // a notification of the call without a token would arrive by now
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(got, ",") != "half,done" {
		t.Fatalf("progress messages %v, want [half done] from the call with a token only", got)
	}
}

func TestTitle(t *testing.T) {
	for in, want := range map[string]string{"ok": "Ok", "out_path": "Out Path", "job_id": "Job Id", "backend_seconds": "Backend Seconds"} {
		if got := title(in); got != want {
			t.Errorf("title(%q) = %q, want %q", in, got, want)
		}
	}
}

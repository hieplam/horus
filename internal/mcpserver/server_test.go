// Tests of the MCP server: the tool list against the golden file
// made from the first version of the server (Python), and the SDK features that the server needs.
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/settings"
	"github.com/hieplam/horus/internal/version"
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
// the server name, version and instructions. The Python server had no
// generate_speech: the golden file holds that tool and the instructions as
// the Go server first gave them.
func TestToolsMatchPython(t *testing.T) {
	useRepoWorkflows(t)
	g := readGolden(t)
	cs := connect(t, New(&fakeImages{}, nil, nil), nil)

	init := cs.InitializeResult()
	// The version is the version of the build (TestServerInfoVersionIsTheVersionOfTheBuild),
	// not the 0.1.0 of the Python server.
	if init.ServerInfo.Name != g.Server {
		t.Errorf("server info %+v, Python %s", init.ServerInfo, g.Server)
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
	if len(got) != len(g.Tools) || len(got) != 6 {
		t.Fatalf("Go has %d tools, the golden file %d, want 6", len(got), len(g.Tools))
	}
	addSetupConsent(t, g.Tools)
	addJobQueue(t, g.Tools)
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

// setupDescription is the text that the description of install_workflow
// has after the text of Python.
const setupDescription = " On a machine that needs a setup step first, the first call without accept_setup=true returns the error " +
	"SETUP_CONFIRMATION_REQUIRED with each setup step in details.steps: for an image workflow a system package and ComfyUI, " +
	"for a speech workflow Python 3.12, FFmpeg and the speech venv. " +
	"Ask the user, then call again with accept_setup=true. The setup consent comes before the license consent."

// addSetupConsent puts three changes into the golden tool list. Phase 4:
// install_workflow has the argument accept_setup and a description that
// names it (spec docs/specs/2026-11-15-release-install.md, "Setup through
// MCP"). The speech epic: install_workflow also installs a speech workflow,
// so its title and its first sentence name no kind, and uninstall_workflow
// also uninstalls one, so its title names no kind and its first sentence
// names both (spec docs/specs/2026-11-20-text-to-speech.md, "Install and
// uninstall"). The golden file itself does not change.
func addSetupConsent(t *testing.T, tools []map[string]any) {
	t.Helper()
	uninstall := false
	for _, py := range tools {
		if py["name"] == "uninstall_workflow" {
			py["title"] = "Uninstall a workflow"
			py["description"] = strings.Replace(py["description"].(string), "of one image workflow to free disk space.",
				"of one workflow (image or speech) to free disk space.", 1)
			uninstall = true
		}
	}
	if !uninstall {
		t.Fatal("the golden file has no uninstall_workflow")
	}
	for _, py := range tools {
		if py["name"] != "install_workflow" {
			continue
		}
		py["title"] = "Install a workflow"
		desc := strings.Replace(py["description"].(string), "of one image workflow, so that generate_image can use it.",
			"of one workflow, so that generate_image or generate_speech can use it.", 1)
		py["description"] = desc + setupDescription
		props := py["inputSchema"].(map[string]any)["properties"].(map[string]any)
		props["accept_setup"] = map[string]any{
			"default":     false,
			"description": "True only after the user accepted the setup steps shown in details.steps of SETUP_CONFIRMATION_REQUIRED.",
			"title":       "Accept Setup",
			"type":        "boolean",
		}
		return
	}
	t.Fatal("the golden file has no install_workflow")
}

// addJobQueue puts the fields of the job queue into the golden tool list
// (spec docs/specs/2026-11-19-job-queue.md, "MCP contract"): the argument
// idempotency_key and the result fields backend_job_id and queue_seconds of
// generate_image, and the result fields ready and queue of list_workflows,
// with their models. The golden file itself does not change.
func addJobQueue(t *testing.T, tools []map[string]any) {
	t.Helper()
	nullable := func(base map[string]any, title, desc string) map[string]any {
		return map[string]any{"anyOf": []any{base, map[string]any{"type": "null"}}, "title": title, "description": desc}
	}
	ref := func(name string) map[string]any { return map[string]any{"$ref": "#/$defs/" + name} }
	str := func(title, desc string) map[string]any {
		m := map[string]any{"type": "string", "title": title}
		if desc != "" {
			m["description"] = desc
		}
		return m
	}
	typed := func(typ, title, desc string) map[string]any {
		m := map[string]any{"type": typ, "title": title}
		if desc != "" {
			m["description"] = desc
		}
		return m
	}
	found := 0
	for _, py := range tools {
		switch py["name"] {
		case "generate_image":
			found++
			in := py["inputSchema"].(map[string]any)["properties"].(map[string]any)
			key := nullable(map[string]any{"type": "string"}, "Idempotency Key", "Optional key of this request, for a later call again "+
				"with the same request. horus keeps it with the job in the stack, and makes a new image for each call.")
			key["default"] = nil
			in["idempotency_key"] = key
			res := py["outputSchema"].(map[string]any)["$defs"].(map[string]any)["ImageResult"].(map[string]any)
			props := res["properties"].(map[string]any)
			props["job_id"] = str("Job Id", "The id of the job: the horus job id in the stack, else the id of the job in the backend.")
			props["backend_job_id"] = str("Backend Job Id", "The id of the job in the backend (ComfyUI).")
			props["queue_seconds"] = nullable(map[string]any{"type": "number"}, "Queue Seconds",
				"Time the job waited in the horus queue before its run. Null: no horus queue.")
			res["required"] = append(res["required"].([]any), "backend_job_id", "queue_seconds")
		case "list_workflows":
			found++
			defs := py["outputSchema"].(map[string]any)["$defs"].(map[string]any)
			list := defs["ListResult"].(map[string]any)
			props := list["properties"].(map[string]any)
			props["ready"] = nullable(ref("Readiness"), "Ready",
				"In the Docker stack: the 4 dependencies of horus and their state. Null: horus runs on this machine without the stack.")
			props["queue"] = nullable(ref("QueueState"), "Queue",
				"In the Docker stack: the jobs that wait, the jobs that run, and the slots of the worker. Null: no horus queue.")
			list["required"] = append(list["required"].([]any), "ready", "queue")
			defs["Readiness"] = map[string]any{"title": "Readiness", "type": "object",
				"required": []any{"ready", "up", "checks", "down"},
				"properties": map[string]any{
					"ready":  typed("boolean", "Ready", "True only when the 4 dependencies are up."),
					"up":     typed("integer", "Up", "The count of the dependencies that are up."),
					"checks": map[string]any{"items": ref("CheckResult"), "title": "Checks", "type": "array"},
					"down": map[string]any{"items": map[string]any{"type": "string"}, "title": "Down", "type": "array",
						"description": "The dependencies that are down."},
				}}
			defs["CheckResult"] = map[string]any{"title": "CheckResult", "type": "object",
				"required": []any{"dependency", "up", "detail"},
				"properties": map[string]any{
					"dependency": map[string]any{"enum": []any{"horus-db", "comfyui", "horus-worker", "horus-api"}, "title": "Dependency", "type": "string"},
					"up":         typed("boolean", "Up", ""),
					"detail":     str("Detail", ""),
				}}
			defs["QueueState"] = map[string]any{"title": "QueueState", "type": "object",
				"required": []any{"waiting", "running", "slots"},
				"properties": map[string]any{
					"waiting": typed("integer", "Waiting", "Jobs in the horus queue that wait for a slot."),
					"running": typed("integer", "Running", "Jobs that run now."),
					"slots":   typed("integer", "Slots", "The jobs that the worker runs at the same time (HORUS_WORKER_CONCURRENCY)."),
				}}
		}
	}
	if found != 2 {
		t.Fatalf("the golden file has %d of generate_image and list_workflows", found)
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
	cs := connect(t, New(&fakeImages{}, nil, nil), nil)
	res := callTool(t, cs, "describe_workflow", map[string]any{"workflow": "nope"})
	if !res.IsError {
		t.Fatal("isError is not set")
	}
	env := envelopeOf(t, res)
	e, _ := env["error"].(map[string]any)
	if env["ok"] != false || env["result"] != nil || e["code"] != "WORKFLOW_NOT_FOUND" {
		t.Fatalf("envelope %v", env)
	}
	if known, _ := e["details"].(map[string]any)["known"].([]any); len(known) != 2 || known[0] != "it-illustration" || known[1] != "vi-speech" {
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

// TestArgumentErrors checks the answer to arguments that fail the schema:
// isError, no structuredContent, and the field in the text.
func TestArgumentErrors(t *testing.T) {
	useRepoWorkflows(t)
	cs := connect(t, New(&fakeImages{}, nil, nil), nil)
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
	a, problems := parseArgs(json.RawMessage(`{"workflow":"w","subject":"s","seed":7.0,"width":null,"extra":1}`), generateImage(nil, nil).args)
	if len(problems) > 0 || *a.intPtr("seed") != 7 || a.intPtr("width") != nil || a.boolean("overwrite") || a.strPtr("out_path") != nil {
		t.Fatalf("args %v, problems %v", a, problems)
	}
	if _, problems := parseArgs(json.RawMessage(`{"workflow":"w","subject":"s","seed":7.5,"overwrite":"yes"}`), generateImage(nil, nil).args); len(problems) != 2 {
		t.Fatalf("problems %v", problems)
	}
}

// TestSDKProgressNotifications checks the SDK feature that G2 and G5 need: a
// tool sends progress notifications when the caller sends a progressToken,
// and sends none without a token.
func TestSDKProgressNotifications(t *testing.T) {
	useRepoWorkflows(t)
	got := make(chan string, 16)
	s := mcp.NewServer(&mcp.Implementation{Name: "horus", Version: version.Version}, nil)
	s.AddTool(&mcp.Tool{Name: "slow", InputSchema: json.RawMessage(`{"type":"object"}`)},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			report := reporter(req)
			report(ctx, 1, 2, "half")
			report(ctx, 2, 2, "done")
			return okResult(map[string]any{}), nil
		})
	cs := connect(t, s, &mcp.ClientOptions{ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
		got <- r.Params.Message
	}})
	// The client can run its notification handler after CallTool returns, and
	// it runs its handlers in the order of arrival. So the test calls the tool
	// with a token, then without one, then with a token again. The messages
	// of the first and third call must arrive, and none between them.
	withToken := func() {
		params := &mcp.CallToolParams{Name: "slow", Arguments: map[string]any{}}
		params.SetProgressToken("tok-1")
		if _, err := cs.CallTool(context.Background(), params); err != nil {
			t.Fatal(err)
		}
	}
	withToken()
	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "slow", Arguments: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	withToken()
	var seen []string
	for len(seen) < 4 {
		select {
		case m := <-got:
			seen = append(seen, m)
		case <-time.After(10 * time.Second):
			t.Fatalf("progress messages %v after 10 seconds, want 4", seen)
		}
	}
	if strings.Join(seen, ",") != "half,done,half,done" || len(got) != 0 {
		t.Fatalf("progress messages %v (and %d more), want [half done half done]: the call without a token sends none", seen, len(got))
	}
}

func TestTitle(t *testing.T) {
	for in, want := range map[string]string{"ok": "Ok", "out_path": "Out Path", "job_id": "Job Id", "backend_seconds": "Backend Seconds"} {
		if got := title(in); got != want {
			t.Errorf("title(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestServerInfoVersionIsTheVersionOfTheBuild checks that the server gives
// the one version variable of the program in its serverInfo, also after a
// release build changes that variable.
func TestServerInfoVersionIsTheVersionOfTheBuild(t *testing.T) {
	useRepoWorkflows(t)
	old := version.Version
	t.Cleanup(func() { version.Version = old })
	for _, want := range []string{old, "1.2.3"} {
		version.Version = want
		cs := connect(t, New(&fakeImages{}, nil, nil), nil)
		if got := cs.InitializeResult().ServerInfo.Version; got != want {
			t.Errorf("serverInfo version %q, want %q", got, want)
		}
	}
}

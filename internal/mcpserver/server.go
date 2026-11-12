// Package mcpserver is the horus MCP server (stdio).
//
// Tools (one file each, tool_<name>.go):
//
//	list_workflows      the image workflows on this machine, and if each one is installed
//	describe_workflow   one workflow in full: sizes, how to write the subject, examples, license
//	install_workflow    download and verify the model files of a workflow
//	uninstall_workflow  delete the model files of a workflow
//	generate_image      make one image with a workflow; waits until the PNG is on disk
//
// Every tool returns the same envelope, in structuredContent and as JSON text:
//
//	{"ok": true,  "result": {...}, "error": null}
//	{"ok": false, "result": null,  "error": {"code", "message", "retryable", "action_by", "next_step", "details"}}
//
// A failed call also sets isError. The error codes are in internal/herr and docs/mcp.md.
package mcpserver

import (
	"context"
	"encoding/json"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/settings"
)

// serverVersion is the server version.
const serverVersion = "0.1.0"

// serverInstructions is the instructions text that the server gives to a client at the start of a session.
const serverInstructions = `horus makes images on this machine with local AI models. You do not need to know how.
1. Call list_workflows. Each workflow is one image model and graph; pick the one whose summary and good_for fit.
2. Call describe_workflow for that workflow once, and read subject.rules.
   horus adds no style: it sends your subject to the model exactly as written. You own the look,
   the palette, the light and the composition, so write them in the subject.
3. Call generate_image. It waits until the PNG is on disk (often 3 to 8 minutes, longer on a cold start)
   and returns its absolute path. Open the file and check it against what you asked.
Every result has ok, result and error. On ok=false, read error.code, error.action_by and error.next_step:
action_by "retry" = call again, "caller" = change your arguments, "human" = ask the user to do error.next_step.
Never install or uninstall a workflow without the user's consent: those tools ask for it.`

// call is one tool call: the checked arguments and the request (for progress).
type call struct {
	args   args
	req    *mcp.CallToolRequest
	report Reporter
}

// tool is one tool: its definition, its argument fields and its body.
type tool struct {
	name        string
	title       string
	description string
	annotations *mcp.ToolAnnotations
	args        []field
	output      obj
	body        func(ctx context.Context, c *call) (any, error)
}

// Images is what the tools need from the image service internal/generate.
// cmd/horus gives the server a *generate.Service. A test gives it a fake.
type Images interface {
	// Generate makes one image (the tool generate_image).
	Generate(ctx context.Context, r generate.Request, o generate.Options) (*generate.Result, error)
	// Running reports if the image backend answers (backend_running of list_workflows).
	Running(ctx context.Context) (bool, error)
	// FreeMemory unloads the models before uninstall_workflow deletes their files.
	FreeMemory(ctx context.Context)
}

// tools is the list of the 5 tools. Each tool lives in its own file.
func tools(images Images, setup Setup) []tool {
	return []tool{listWorkflows(images, setup), describeWorkflow(setup), installWorkflow(setup), uninstallWorkflow(images, setup), generateImage(images)}
}

func (t tool) definition() *mcp.Tool {
	in, _ := json.Marshal(model(t.name+"Arguments", t.args...))
	out, _ := json.Marshal(t.output)
	return &mcp.Tool{
		Name:         t.name,
		Title:        t.title,
		Description:  t.description,
		Annotations:  t.annotations,
		InputSchema:  json.RawMessage(in),
		OutputSchema: json.RawMessage(out),
	}
}

func (t tool) handler() mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a, problems := parseArgs(req.Params.Arguments, t.args)
		if len(problems) > 0 {
			logf("WARNING", "Tool '%s' rejected arguments", t.name)
			return argsError(t.name, problems), nil
		}
		logf("INFO", "call %s progress_token=%v", t.name, pyNone(progressToken(req)))
		if e := workflowsDirError(); e != nil {
			logf("INFO", "call %s failed: %s", t.name, e)
			return failResult(e), nil
		}
		c := &call{args: a, req: req, report: reporter(req)}
		return guard(ctx, t.name, func(ctx context.Context) (any, error) { return t.body(ctx, c) }), nil
	}
}

func pyNone(v any) any {
	if v == nil {
		return "None"
	}
	return v
}

// New makes the server with the 5 tools. The tools make images, and ask
// the image backend, through images. They set up the model files of a
// workflow through setup.
func New(images Images, setup Setup) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "horus", Version: serverVersion}, &mcp.ServerOptions{Instructions: serverInstructions})
	for _, t := range tools(images, setup) {
		s.AddTool(t.definition(), t.handler())
	}
	return s
}

// Run serves the tools over stdio until the client closes the connection.
func Run(ctx context.Context, images Images, setup Setup) error {
	if path := settings.Get("HORUS_LOG_FILE", ""); path != "" {
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			defer f.Close()
			logTo(teeWriter{os.Stderr, f})
		} else {
			logf("WARNING", "cannot open HORUS_LOG_FILE: %v", err)
		}
	}
	return New(images, setup).Run(ctx, &mcp.StdioTransport{})
}

type teeWriter struct {
	a, b interface{ Write([]byte) (int, error) }
}

func (t teeWriter) Write(p []byte) (int, error) {
	_, _ = t.b.Write(p)
	return t.a.Write(p)
}

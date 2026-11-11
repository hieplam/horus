// The envelope that every tool returns, and the guard around each call.
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/herr"
)

// Envelope is {"ok", "result", "error"}, in this key order.
type Envelope struct {
	OK     bool       `json:"ok"`
	Result any        `json:"result"`
	Error  *herr.Info `json:"error"`
}

// encode writes JSON as json.dumps(data, ensure_ascii=False, indent=indent)
// does: no HTML escapes, and no indent when indent is empty.
func encode(v any, indent string) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// result puts the envelope in structuredContent and, as JSON text with an
// indent of 1, in the text content.
func result(env Envelope) *mcp.CallToolResult {
	data, err := encode(env, "")
	if err != nil {
		return result(Envelope{Error: ptr(herr.New("INTERNAL", "cannot encode the result: "+err.Error(), nil).Info())})
	}
	text, _ := encode(env, " ")
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(text)}},
		StructuredContent: json.RawMessage(data),
		IsError:           !env.OK,
	}
}

func ptr[T any](v T) *T { return &v }

// okResult wraps the result of a tool that succeeded.
func okResult(v any) *mcp.CallToolResult { return result(Envelope{OK: true, Result: v}) }

// failResult wraps a horus error. It also sets isError.
func failResult(e *herr.Error) *mcp.CallToolResult {
	return result(Envelope{OK: false, Error: ptr(e.Info())})
}

// guard runs the body of a tool. A *herr.Error becomes its envelope. Any
// other error or a panic is a bug: it still returns the INTERNAL envelope,
// never a bare stack trace.
func guard(ctx context.Context, name string, body func(context.Context) (any, error)) (res *mcp.CallToolResult) {
	defer func() {
		if r := recover(); r != nil {
			logf("ERROR", "call %s crashed\n%v\n%s", name, r, debug.Stack())
			res = failResult(herr.New("INTERNAL", fmt.Sprintf("panic: %v", r), map[string]any{"tool": name}))
		}
	}()
	v, err := body(ctx)
	if err == nil {
		return okResult(v)
	}
	var he *herr.Error
	if errors.As(err, &he) {
		logf("INFO", "call %s failed: %s", name, he)
		return failResult(he)
	}
	logf("ERROR", "call %s crashed\n%v", name, err)
	return failResult(herr.New("INTERNAL", err.Error(), map[string]any{"tool": name}))
}

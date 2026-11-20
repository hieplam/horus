// horus errors: the table of error codes, for a reader that is not Go (the
// contract test). And horus
// mcp: the flags of the MCP server.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/hieplam/horus/internal/herr"
)

// Errors is horus errors.
func Errors(args []string, out, errOut io.Writer) int {
	fs := newFlags("horus errors", errOut)
	asJSON := fs.Bool("json", false, "print the table as one JSON object: code -> retryable, action_by, next_step")
	if code, done := parse(fs, args); done {
		return code
	}
	if *asJSON {
		out.Write(herr.TableJSON())
		return 0
	}
	for _, s := range herr.Codes {
		retry := "no"
		if s.Retryable {
			retry = "yes"
		}
		fmt.Fprintf(out, "%-30s retryable=%-3s action_by=%-6s %s\n", s.Code, retry, s.ActionBy, s.NextStep)
	}
	return 0
}

// MCP is horus mcp. serve runs the MCP server over stdio until ctx ends;
// cmd/horus gives it the server with its services. ready is the readiness of
// the stack (HORUS_DATABASE_URL is set), or nil on the direct path. In the
// stack, the session starts only when the 4 checks pass: else horus mcp
// writes the report to errOut and exits 1.
func MCP(ctx context.Context, args []string, errOut io.Writer, ready Readiness, serve func(context.Context) error) int {
	fs := newFlags("horus mcp", errOut)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: horus mcp")
		fmt.Fprintln(fs.Output(), "Serves list_workflows, describe_workflow, install_workflow, uninstall_workflow, generate_image and generate_speech over stdio.")
		fmt.Fprintln(fs.Output(), "With HORUS_DATABASE_URL (in the stack), the session starts only when the 4 checks of horus ready pass.")
	}
	if code, done := parse(fs, args); done {
		return code
	}
	if ready != nil {
		if r := ready(ctx); !r.Ready {
			fmt.Fprintf(errOut, "horus mcp: horus is not ready, so this session does not start. Start the dependency that is down, then start the session again.\n%s", r.Text())
			return 1
		}
	}
	if err := serve(ctx); err != nil && ctx.Err() == nil {
		fmt.Fprintf(errOut, "horus mcp: %v\n", err)
		return 1
	}
	return 0
}

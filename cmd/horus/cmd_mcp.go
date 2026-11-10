// horus mcp: the MCP server over stdio. Ports horus/server.py (main).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/hieplam/horus/internal/server"
)

func init() { register("mcp", "serve the 5 horus tools over stdio (MCP)", runMCP) }

func runMCP(args []string) int {
	fs := flag.NewFlagSet("horus mcp", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: horus mcp")
		fmt.Fprintln(fs.Output(), "Serves list_workflows, describe_workflow, install_workflow, uninstall_workflow and generate_image over stdio.")
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := server.Run(ctx); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "horus mcp: %v\n", err)
		return 1
	}
	return 0
}

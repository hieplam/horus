# 0003. Use the stdio transport. The HTTP transport comes later

- **Status:** Accepted
- **Date:** 2026-11-15
- **Deciders:** the owner
- **Old ids in issue #4:** D2, D3, D55, risks R6 and R7

## Context

MCP has two transports. With stdio, the client starts the server as a child
process, and nothing listens on the network. With Streamable HTTP, the server
runs alone and the client connects to a URL. An HTTP server on localhost
permits DNS rebinding: a web page in the browser of the user calls the server.
A long download needs MCP tasks (the server returns a task id at once, and the
client polls). The Go MCP SDK v1.8.0 has no MCP tasks.

## Decision

- `horus mcp` uses stdio. This is the only transport today.
- horus has no daemon. ComfyUI is already a shared server with a job queue.
- The HTTP transport (`horus serve`, Phase 5) does not start until the Go MCP
  SDK has MCP tasks.
- When it starts, `horus serve` binds `127.0.0.1`, checks the Host and Origin
  headers (answer 403 when wrong), and needs a bearer token.

## Consequences

- The server has no network port and no token today.
- `generate_image` and `install_workflow` hold one call open for minutes. The
  progress notifications of the server show that the call is alive.
- Two agents each start their own `horus mcp`. ComfyUI puts their jobs in one
  queue.
- To reverse this decision, add `horus serve`. The services do not change,
  because a handler holds the transport
  ([0006](0006-design-rules-and-three-layers.md)).

# 0030. `horus-api` serves MCP over stdio and over HTTP

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the owner, in the planning session of epic #180

## Context

The owner: "when horus is on it have 2 way to communicate: stdio and http, so
if it down none of those approach work."

- [ADR 0003](0003-stdio-transport-http-later.md) says that stdio is the only
  transport, and that the HTTP transport waits for MCP tasks in the Go MCP SDK.
  It also sets the rules for the HTTP transport: bind 127.0.0.1, check the
  `Host` and `Origin` headers (403 when wrong), and need a bearer token.
- The Go MCP SDK v1.8.0 has the HTTP server `mcp.NewStreamableHTTPHandler`
  (`mcp/streamable.go:231`).
- With the stack of [ADR 0029](0029-horus-runs-as-a-stack-of-three-containers.md),
  a long call waits in the queue and sends a progress message every 10 s, on
  both transports.
- Not verified: that Claude Code and Codex keep an HTTP call open for many
  minutes, and that each one reads a variable in a header.

## Decision

- Over stdio, an agent runs `docker exec -i horus-api horus mcp`.
  `horus register -docker` adds that command.
- Over HTTP, an agent sends Streamable HTTP to `http://127.0.0.1:8190/mcp`.
  `horus register -http` adds the URL, with a header that names the variable
  `HORUS_API_TOKEN` and not the token.
- `/mcp` follows the rules of ADR 0003: the stack publishes the port only on
  127.0.0.1, a wrong `Host` or `Origin` header gets 403, and a request without
  the right bearer token gets 401.
- The token is in the `.env` of the stack and in the environment of an agent.
  horus never writes it to a log, a URL or a command line.
- Both ways give the same tools and the same envelope.
- When horus is not ready, neither way serves.
- This replaces the line "`horus mcp` uses stdio. This is the only transport
  today" of ADR 0003. The other rules of ADR 0003 stay.

## Consequences

- An agent harness without stdio can use horus.
- The contract test runs three times: on the host, over stdio into the stack,
  and over HTTP.
- The long install of [ADR 0011](0011-setup-through-mcp-needs-two-consents.md)
  still has no MCP tasks. In the stack, `install_workflow` downloads only
  model files.
- To reverse this decision, remove the route `/mcp` and `horus register
  -http`. Stdio stays.

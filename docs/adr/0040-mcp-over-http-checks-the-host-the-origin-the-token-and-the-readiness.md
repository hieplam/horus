# 0040. `/mcp` checks the Host, the Origin, the token and the readiness in this order, ends a call when its agent goes away, and a client config names only the variable of the token

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the agent of task #204, by the standing rules of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md), the
sections "Two ways in", "Readiness" and "Settings", and the goal rows
`http-guard`, `same-contract` and `ready-when-four-up`.
[ADR 0030](0030-horus-api-serves-mcp-over-stdio-and-http.md) and
[ADR 0003](0003-stdio-transport-http-later.md) give the rules of `/mcp`: bind
127.0.0.1, 403 for a wrong `Host` or `Origin` header, 401 without the right
bearer token, and no token in a log, a URL or a command line.

- The task gives the order of the checks: `Host` (403), `Origin` (403), the
  bearer token (401, a compare in constant time), the readiness (503 with the
  report).
- The spec: a new session cannot start while a dependency is down, and a call
  in an open session gets `NOT_READY`. A request of Streamable HTTP without the
  header `Mcp-Session-Id` starts a new session.
- [ADR 0038](0038-the-horus-ui-renders-on-the-server-and-checks-the-published-port.md)
  put the check of the `Host` header in `internal/webui`. `internal/archtest`
  does not let a handler import another handler, so `internal/mcpserver`
  cannot use `webui.HostAllowed`.
- The Go MCP SDK v1.8.0 runs a tool with a context of the session, not of the
  HTTP request. Measured in `internal/mcpserver/http_test.go`: a client that
  closes the connection of its `POST` does not end the call, and the job
  stays `queued`. A `DELETE` of the session waits until each call of the
  session ends (`jsonrpc2.Connection.Close` waits for the calls), so a waiting
  `generate_image` holds the `DELETE` for minutes. Over stdio, the end of the
  session cancels the call and its job.
- `HORUS_API_TOKEN` can be absent: an owner who uses only stdio needs no token.
- Not verified before this task: that Claude Code and Codex read a variable in
  the config of an HTTP server. Measured in this task with the real programs
  against the test stack (the pull request of task #204 has the output):
  Claude Code 2.1.329 puts the value of `HORUS_API_TOKEN` in place of
  `${HORUS_API_TOKEN}` in a header, in a `--mcp-config` file and in the user
  config that `claude mcp add --header` writes: `claude -p` called
  `list_workflows` over HTTP, and a wrong token gave 401. Codex 0.166.0 reads
  the variable that `bearer_token_env_var` names: its MCP client initialized
  a session with the server `horus`, and a wrong token gave 401. A tool call
  that the model of Codex makes is not verified: the Codex account of the PC
  had no usage left.

## Decision

- `/mcp` runs 4 checks in this order, and the first check that fails gives the
  answer: the `Host` header is `127.0.0.1:<port>` or `localhost:<port>` (else
  403); the `Origin` header is absent or the origin of horus api itself (else
  403); `Authorization: Bearer <token>` has the token of `HORUS_API_TOKEN`
  (else 401 with `WWW-Authenticate: Bearer`); a request without
  `Mcp-Session-Id` runs the readiness, and gets 503 with the report as JSON
  when a dependency is down.
- A call in an open session runs the readiness gate of each call, as over
  stdio, and gets `NOT_READY`.
- The checks are pure functions in the base package `internal/httpguard`.
  The Horus UI and `/mcp` call the same `httpguard.HostAllowed`. The compare
  of the token compares the SHA-256 of both values with
  `subtle.ConstantTimeCompare`.
- Without `HORUS_API_TOKEN`, `/mcp` refuses each request with 401, and
  `horus api` writes one log line at its start that says why. The readiness
  report does not check the token.
- `/mcp` removes the header `Authorization` before the SDK, so no tool sees
  the token.
- `/mcp` joins each tool call to the `POST` that carries it. When the `POST`
  ends first, the call ends. Before a `DELETE` of a session, `/mcp` cancels
  each call of the session. So a job is cancelled when its agent goes away,
  as over stdio.
- A session has no idle time limit. It ends at its `DELETE` or when
  `horus api` stops. The HTTP server has no time limit for an answer.
- `compose.yaml` gives `HORUS_API_TOKEN` only to `horus-api`.
- `horus register -http` writes the name of the variable, never the token:
  `claude mcp add --scope user --transport http horus <url> --header
  "Authorization: Bearer ${HORUS_API_TOKEN}"`, and `codex mcp add horus --url
  <url> --bearer-token-env-var HORUS_API_TOKEN`. The URL is
  `http://127.0.0.1:<HORUS_API_PORT>/mcp`.

## Consequences

- A web page in a browser cannot use `/mcp`, and an agent without the token
  cannot either.
- The agent needs `HORUS_API_TOKEN` in its own environment. The docs give a
  command for each system that copies the value of `.env` into the
  environment of the user, with no token on a command line.
- A new check of a request to `horus api` goes into `internal/httpguard`.
- A new version of the Go MCP SDK that cancels a call when its request ends
  makes the join of a call to its `POST` a second safety, not a fault.
- To reverse this decision, remove `internal/httpguard` and the route `/mcp`
  ([ADR 0030](0030-horus-api-serves-mcp-over-stdio-and-http.md)). Stdio stays.

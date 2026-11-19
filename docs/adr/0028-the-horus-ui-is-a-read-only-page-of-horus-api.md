# 0028. The Horus UI is a read-only page of `horus-api` on 127.0.0.1

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the owner, in the planning session of epic #180

## Context

The owner wants a page that shows how many jobs wait, the job that runs, the
recent results, the worker and the workflows of the MCP server. The owner
ratified a mockup:
[`2026-11-19-job-queue-ui.html`](../specs/2026-11-19-job-queue-ui.html).

A page on localhost can be read by a web page of another domain through DNS
rebinding, as [ADR 0003](0003-stdio-transport-http-later.md) says for an
HTTP server.

## Decision

- `horus api`, the main process of `horus-api`, serves the page at `/`, a JSON
  summary, `/readyz` and a stream of Server-Sent Events, on the port
  `HORUS_API_PORT` (default 8190).
- The page is read-only. It has no button that changes a job.
- `compose.yaml` publishes the port only on 127.0.0.1.
- The page refuses a request whose `Host` header is not `127.0.0.1:<port>` or
  `localhost:<port>`.
- The page needs no token. Only `/mcp` needs the token of
  [ADR 0030](0030-horus-api-serves-mcp-over-stdio-and-http.md).
- The page is in the program (`go:embed`), with no build step.
- The page has the 11 parts of the reference: the readiness strip, 5 tiles and
  5 panels.

## Consequences

- A person sees the load without a login.
- A cancel button, or another write, needs a login first. That is later work.
- The page shows the subjects of the jobs, so it stays on 127.0.0.1.
- To reverse this decision, remove `internal/webui` and its route.

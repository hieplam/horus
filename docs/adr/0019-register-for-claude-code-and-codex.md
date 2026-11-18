# 0019. `horus register` serves Claude Code and Codex

- **Status:** Accepted
- **Date:** 2026-11-15
- **Deciders:** the planning session of Phase 4. The owner ratified the phase goal
- **Old ids in issue #4:** D66

## Context

The long-term view of the owner: horus works with many agent vendors, today
Claude Code and OpenAI Codex. Each client has its own command to add an MCP
server.

## Decision

- `horus register -client claude` (the default) and
  `horus register -client codex` add the server `horus` to that client.
- A second run changes nothing.
- `HORUS_ROOT` goes to the client only when there is a repo root.
- `horus register` adds only `horus`, never `comfy-mcp`.

## Consequences

- A person needs no knowledge of the `mcp add` command of each client.
- A third client is one more value of `-client` in the `provision` service.
- To reverse this decision, remove the `-client` flag. A Codex user then adds
  the server by hand.

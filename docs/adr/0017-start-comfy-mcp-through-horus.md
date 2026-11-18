# 0017. `.mcp.json` starts the comfy MCP server through `horus comfy-mcp`

- **Status:** Accepted
- **Date:** 2026-11-12
- **Deciders:** the planning session of Phase 3. The owner ratified the phase goal
- **Old ids in issue #4:** D46

## Context

`.mcp.json` started `comfy-mcp` from a Windows venv path, so it failed on
macOS. The first idea was `uv tool install comfy-mcp==0.10.0`. But the venv of
`horus install-comfy-mcp` is not on the PATH, and the owner's PC has no `uv`.
`comfy-mcp` on Windows also needs UTF-8 settings and the path of `comfy`.

## Decision

- `.mcp.json` starts the comfy MCP server as `horus comfy-mcp` on both
  systems.
- `horus comfy-mcp` runs the `comfy-mcp` of the venv of
  `horus install-comfy-mcp`, then the `comfy-mcp` of the PATH.
- It sets `PYTHONUTF8`, `PYTHONIOENCODING` and `COMFY_BIN` for that process.

## Consequences

- `.mcp.json` has no path and no OS branch. Only `horus` must be on the PATH.
- When `horus` is not on the PATH, both servers of `.mcp.json` fail.
  `horus doctor` names the fix.
- Only an agent in a clone gets `comfy-mcp`. `horus register` adds only
  `horus`.
- To reverse this decision, put a `comfy-mcp` path into `.mcp.json` again.
  That path is different on each system.

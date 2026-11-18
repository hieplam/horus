# 0001. Write horus in Go, one program file for each platform

- **Status:** Accepted
- **Date:** 2026-11-10
- **Deciders:** the owner
- **Old ids in issue #4:** D0, D1, D13, D15, D22, D23

## Context

horus was a Python package (about 1,300 lines) and 10 PowerShell scripts
(about 1,530 lines). It worked only on Windows: `.mcp.json` started programs
from a Windows venv path, and all setup was PowerShell. The first idea was to
keep Python, port the PowerShell to Python and install with `uv`. The owner
wants a thin layer that many agent vendors can start with no venv.

## Decision

- horus is one Go source tree. The build selects the platform with `GOOS` and
  `GOARCH`, and the result is one program file: `horus.exe` for Windows,
  `horus` for macOS.
- The Go module is `github.com/hieplam/horus`. The MCP library is the official
  Go SDK, `github.com/modelcontextprotocol/go-sdk`.
- A developer builds with `go install ./cmd/horus`. `.mcp.json` starts
  `horus mcp` by name, from the PATH. CI builds with `go build -o bin/`.
- The developer tools stay Python: `tests/mcp_contract.py`,
  `tests/workflow_parity.py`, `tools/goal_check.py` and
  `scripts/lint-public.py`. No user runs them.
- `horus errors -json` prints the error codes. The contract test reads the
  codes from that command.

## Consequences

- A user needs no Python and no venv to run horus. The machine still needs
  Python, because ComfyUI is a Python program.
- A real `.exe` on the PATH starts from Claude Code on Windows without a
  `cmd /c` shim.
- A contributor needs Go for the program and Python 3.12 for the test tools.
- To reverse this decision, the whole program must be written again in a
  different language.

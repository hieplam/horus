# 0002. Port first with the same behaviour, then restructure

- **Status:** Accepted. The child process of Phase 1 is superseded by [0006](0006-design-rules-and-three-layers.md)
- **Date:** 2026-11-11
- **Deciders:** the owner
- **Old ids in issue #4:** D8, D11, D12, D14, D16, D19, D21, D29

## Context

A port and a new design in one step give no fixed point to compare with. The
Python server started `workflow.ps1` as a child process and parsed its last
output line, `HORUS_JSON {...}`.

## Decision

- The work runs in five phases, in this order:
  1. Port all of `horus/` and `scripts/` to Go with the same behaviour.
  2. Restructure the Go code into three layers.
  3. macOS.
  4. Release, install line, uninstall and setup through MCP.
  5. The HTTP transport.
- Phase 1 kept the Python shape: one Go file for each Python module or script,
  and `horus mcp` started `horus workflow <action> -json` as a child process.
  Phase 2 ended that child process.
- Flags use the Go style: `-Json` became `-json`. A flag error exits 2. A
  message that named a `.ps1` script names the `horus` subcommand.
- `install-workflow-models.ps1` has no port. `horus workflow install` and
  `horus workflow check` replace it.

## Consequences

- Phase 1 had a proof of equal behaviour: the contract test and the parity
  test compare the Go program with the recorded output of the old code.
- Phase 2 moved the code of Phase 1, so some code was written two times.
- The order is history now. Phases 1 to 4 are done. Phase 5 waits
  ([0003](0003-stdio-transport-http-later.md)).

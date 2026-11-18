# 0006. Five design rules, three layers, and a test with no allow list

- **Status:** Accepted
- **Date:** 2026-11-11
- **Deciders:** the owner
- **Old ids in issue #4:** D26, D27, D28, D39

## Context

The Phase 1 port mirrored the Python modules. A handler called an adapter
directly, and the server started a child process and parsed a text line. The
owner wants each part behind an interface that a test can replace.

## Decision

- Five design rules apply to all code:
  1. Simple. Add no design pattern that hides no complexity.
  2. Deep modules: a package hides its work behind a small interface (John
     Ousterhout, "A Philosophy of Software Design").
  3. Pure functions first. Code with no input and no output gets no interface.
  4. Interfaces only at input and output. The service declares the interface
     that it needs.
  5. One direction of calls.
- Three layers: handlers (`internal/cli`, `internal/mcpserver`), then services
  (`generate`, `provision`, `doctor`, `devtools`), then adapters (`comfy`,
  `download`, `modelfiles`, `backend`). `cmd/horus` creates the adapters and
  gives them to the services.
- `horus mcp` calls the `provision` service in its own process. It starts no
  `horus workflow` child process. `horus workflow ... -json` stays as a command.
- `internal/archtest` enforces the direction of imports, the OS boundary
  (OS-specific code only in `internal/platform` and `internal/backend`) and the
  token rule ([0005](0005-only-download-reads-the-token.md)).
- The rules of `internal/archtest` have no allow list. No later change adds an
  entry.

## Consequences

- A test gives a service a fake adapter, so most tests need no ComfyUI and no
  network.
- A shortcut from a handler to an adapter fails CI. The fix is in the code,
  not in the test.
- The full rules and the layer table are in
  [`../specs/2026-11-11-restructure.md`](../specs/2026-11-11-restructure.md).
- To reverse this decision, delete `internal/archtest`. The code then loses
  the proof of its structure.

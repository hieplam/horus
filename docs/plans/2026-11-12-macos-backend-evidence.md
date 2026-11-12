# Evidence log: horus on macOS through comfy-cli (Phase 3)

Spec: [`../specs/2026-11-12-macos-backend.md`](../specs/2026-11-12-macos-backend.md).
Plan: [`2026-11-12-macos-backend.md`](2026-11-12-macos-backend.md).
Epic: [#98](https://github.com/hieplam/horus/issues/98).

Each task adds one section `## <task id>` with the literal output of its Green
commands.

## Decisions

| Date | Decision | Why | By |
|---|---|---|---|
| 2026-11-12 | The owner ratified the 8 goal rows, the task list and the standing rules of the spec. | The one ratification of this phase. | Owner |
| 2026-11-12 | The default install folder on macOS is `<home>/horus`, with ComfyUI in `<home>/horus/ComfyUI`. | It is outside the TCC folders, it has no space in its path, and a user can see it. | Owner |
| 2026-11-12 | The session on the Mac uses the account `hoacao33`. The owner merges its pull requests by hand. | The owner's merge rule lets an agent merge only a pull request of the owner. | Owner |
| 2026-11-12 | The real run on the Mac downloads the model files again (11 GB) into `~/horus`. `~/horus-lab` is not changed. | It proves the whole setup from nothing. | Owner |
| 2026-11-12 | Issues #82 and #91 are part of this phase (tasks T3 and T6). | They change `internal/doctor` and `docs/setup.md`, and this phase changes both. | Plan |
| 2026-11-12 | `HORUS_GENERATE_TIMEOUT` keeps its default of 1800 s. | The cold run of #53 took 1033 s, so 767 s are spare. | Plan |
| 2026-11-12 | `horus doctor` gets a WARN level that does not change the exit code. | The clone on the Mac is inside `~/Desktop`. A FAIL line for it could never pass, and the row `tcc-folders` asks for a warning. | Plan |
| 2026-11-12 | The comfy-cli backend writes the same state file as the Windows backend, and `Stop` reads it. | comfy-cli 1.25.0 keeps one background ComfyUI for each user (`comfy_cli/command/launch.py`, `ConfigManager().background`), and `comfy stop` stops that one. Without a record of its own, horus could stop a ComfyUI that a person started. | Plan |
| 2026-11-12 | `.mcp.json` starts the comfy MCP server as `horus comfy-mcp`, a new command that runs the `comfy-mcp` of the venv or of the PATH. The draft said "comfy-mcp as a command on the PATH". | A venv is not on the PATH, and `uv` is not installed on the PC. With `horus comfy-mcp` the PATH needs only `horus` on both systems, and the PC needs no new program. The row `one-config` holds: each entry names a command on the PATH, and the Windows-only paths go from 2 to 0. | Plan |
| 2026-11-12 | The comfy-cli backend finds `comfy` on the PATH, then in the venv of `horus install-comfy-mcp`. | That venv already holds comfy-cli 1.25.0, so a Mac needs no second install of it and no PATH entry for it. | Plan |
| 2026-11-12 | The verify task of the draft ("the Mac and the PC") is two tasks: T8 on the PC, then T9 on the Mac. T8 waits for the real run on the Mac (T7) and the docs (T6). | One session cannot run on two machines. The final commit is verified on both. | Plan |
| 2026-11-12 | The baseline of `same-behaviour` is commit `943d430` with 215 Go test functions (`go test -list . ./...`). | Measured on `main` before the first task. | Plan |

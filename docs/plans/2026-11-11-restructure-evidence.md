# Evidence log: restructure horus into three layers (Phase 2)

Plan: [`2026-11-11-restructure.md`](2026-11-11-restructure.md). Spec:
[`../specs/2026-11-11-restructure.md`](../specs/2026-11-11-restructure.md).
Each task adds one section `## <task id>` with the literal output of its Green
commands.

## Decisions

| Date | Decision | Why | By |
|---|---|---|---|
| 2026-11-11 | The owner ratified the 8 goal rows, the design rules and the standing rules of the spec. | The one ratification of this phase. | Owner |
| 2026-11-11 | `horus mcp` calls the `provision` service in its own process. D12 and D16 of #4 (a `horus workflow` child process) end. | The owner wants each part behind an interface that a test can replace. A child process with a text line is not such an interface. | Owner |
| 2026-11-11 | The row `process-tree` proves `Backend.Stop` and the detached start. It no longer proves the end of a download child. | After the decision above, the server has no download child. | Plan |
| 2026-11-11 | PR #49 (fixes of #39 to #42) and PR #51 merged before the first task. #40 and #41 are not tasks of this phase. | The fixes existed, and the move tasks touch the same files. | Plan |

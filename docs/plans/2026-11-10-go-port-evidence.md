# Evidence log: port horus to Go (Phase 1)

Plan: [`2026-11-10-go-port.md`](2026-11-10-go-port.md). One section for each
task, with the literal output of its Red and Green commands.

## Decisions

Decisions made under the standing rules of the spec, newest last.

| Date | Task | Decision | Why |
|---|---|---|---|
| 2026-11-10 | plan | The `real-image` row does not install the model files from nothing. | The owner tests the install on a fresh machine. |
| 2026-11-10 | plan | Phase 1 has its own epic issue. Issue #4 stays the page of the whole goal. | The epic closes when its tasks close, and Phases 2 to 4 have no tasks yet. |

## Baseline (2026-11-10, commit `c5379f9`)

- Contract test against the Python server on the owner's PC: `contract: 30 of 30 passed`.
- `horus/*.py` plus `scripts/*.ps1`: 2,827 lines.

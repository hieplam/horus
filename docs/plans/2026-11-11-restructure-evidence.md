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
| 2026-11-11 | T0 chose the allow list file names: `internal/archtest/allow_os-boundary.txt`, `internal/archtest/allow_direction.txt`, `internal/archtest/allow_token.txt`. One entry on each line. A blank line or a line that starts with `#` is not an entry. Task T1 uses these names. | The goal tool must read the lists before T1 exists. | Plan (T0) |
| 2026-11-11 | The row `layout` counts the packages of `go list ./...` that are not in `tools/phase2_packages.txt` (12 today). It also prints the number of listed packages that do not exist yet (11 today). The row passes only when both numbers are 0. | The ratchet "12 to 0" counts the extra packages. A missing package must also fail the row. | Plan (T0) |
| 2026-11-11 | The row `same-behaviour` accepts a renamed golden file when git reports the rename as 100 percent equal (`R100`). Any other change under the contract paths fails. | A golden file of `internal/server/testdata` moves with its package. The content does not change. | Plan (T0) |
| 2026-11-11 | The row `process-tree` needs a passing test whose name has `Tree` in it, in `internal/platform`. The row counts each occurrence of `taskkill` in Go files (8 today, in 6 lines). | The plan names "the tree test" but no test name. | Plan (T0) |
| 2026-11-11 | The rows `backend` and `commands` pass only with `--real`. Without it they print that the Go tests are fine and that the owner's PC is needed. `commands` passes when `horus doctor` has 0 FAIL lines and the plan of `horus setup it-illustration` says "nothing is missing". | Their oracle runs on the owner's PC, like the real-image row of Phase 1. | Plan (T0) |
| 2026-11-11 | The baseline commit of `same-behaviour` is `3e27da4`. The baseline number of Go test functions is 157. | Measured at T0 (see the Baseline section). | Plan (T0) |

## Baseline

Measured at `3e27da4` (main after PR #54), before any move task.

- Packages off target: 12 (`internal/models`, `internal/server` and the 10 packages of `internal/scripts/`). Listed packages that do not exist yet: 11.
- Go test functions (`go test -list . ./...`, lines that start with `Test`): 157.
- `taskkill` references in Go files: 8 (in 6 lines of `internal/models/kill_unix.go` and `internal/models/kill_windows.go`).
- Files that name `HF_TOKEN` (10, test files included): `internal/herr/herr.go`, `internal/models/models.go`, `internal/models/models_test.go`, `internal/scripts/downloadmodel/command.go`, `internal/scripts/downloadmodel/downloadmodel.go`, `internal/scripts/downloadmodel/downloadmodel_test.go`, `internal/scripts/workflow/install.go`, `internal/scripts/workflow/workflow_test.go`, `internal/settings/settings.go`, `internal/settings/settings_test.go`.
- Files that break the rule `os-boundary` (19): 9 files with an OS name (`start_*`, `kill_*`, `freespace_*`, `diskfree_windows`, `detach_*`) and 10 more that use `runtime.GOOS`, `syscall` or a `//go:build` line (`checksetup.go`, `freespace_other.go`, `installcomfymcp.go`, `installcomfymcp_test.go`, `diskfree_other.go`, `installcomfyui.go`, `register_test.go`, `startcomfyui.go`, `files.go`, `workflow_test.go`).

## T0

Red (the tool before the change):

    python old_goal_check.py --phase 2 --no-ci
    old_goal_check.py: error: unrecognized arguments: --phase 2      (exit 2)

Green: `python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>/horus.exe mcp` (exit 1):

    FAIL  layout - 12 packages off target, 11 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 157 Go test functions (baseline 157); 0 changed contract files
    FAIL  os-boundary - missing: internal/archtest (task T1); allow list length unknown
    FAIL  direction - missing: internal/archtest (task T1); allow list length unknown
    FAIL  process-tree - missing: internal/platform (task T2); 8 taskkill references
    FAIL  backend - missing: internal/backend (task T7)
    FAIL  token - missing: internal/archtest (task T1); allow list length unknown
    FAIL  commands - missing: internal/cli, internal/doctor (tasks T8 and T9)
    goal: 1 of 8 rows passed

Ratchet: rows with a measuring command 0 to 8.

Phase 1 (python tools/goal_check.py --no-ci, unchanged behaviour): 7 of 8 rows passed. Only real-image fails (needs --real on the GPU machine).

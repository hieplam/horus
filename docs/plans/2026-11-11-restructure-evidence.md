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
| 2026-11-11 | The rule `token` flags a Go string whose value is `HF_TOKEN`, or that starts with `HF_TOKEN=` (a line of a `.env` file). It does not flag the error code `HF_TOKEN_INVALID`, a comment, or a longer text. The exempt places are `internal/download` and the two files of the refusal: `internal/settings/settings.go` and `internal/settings/settings_test.go`. The allow list has 4 files. | The `next_step` text of `HF_TOKEN_INVALID` in `internal/herr` names `HF_TOKEN`, and `internal/herr/testdata/errors.golden.json` holds that text. A golden file must not change, so a plain text match could never reach 0. Code that reads the token names the variable in a string of its own. | Agent (T1) |
| 2026-11-11 | The rule `os-boundary` flags a file when a part of its name after the first `_` is an OS name of Go or `unix`, when a `//go:build` line names an OS, when it uses `runtime.GOOS`, or when it imports `syscall` or `golang.org/x/sys/...`. Test files count. Nothing under `internal/platform` or `internal/backend` is flagged. There is no planted file for an old `// +build` line: `gofmt` adds a `//go:build` line to such a file, and CI fails a file that `gofmt -l` lists. The code still reads both kinds of line. | The spec names the four cases. The baseline of 19 files counts `_unix` files and test files. | Agent (T1) |
| 2026-11-11 | The layers of the Phase 1 packages in `internal/archtest/layers.go`: `internal/server` and `internal/scripts/runillustration` are handlers. `internal/models`, `checksetup`, `installcomfymcp`, `newworkflow`, `register`, `syncworkflows` and `workflow` are services. `downloadmodel`, `installcomfyui` and `startcomfyui` are adapters. A package that is not in the table fails the rule `direction`. | Each one gets the layer of the package that its code moves to (spec table, plan T1 step 2). `runillustration` moves to `internal/cli` (plan T5 step 5). | Agent (T1) |
| 2026-11-11 | `internal/archtest` and `internal/comfy/comfytest` are in the layer "test". Only a test file can import them. The spec sets no import limit for them, so archtest sets none. | The spec says only that a test file can import `comfytest`. | Agent (T1) |
| 2026-11-11 | An entry of the allow list `direction` is a pair `<package> -> <import>` for the whole package, test files included. The ban of a service is on the exact import paths `net/http` and `os/exec`. | The plan writes the entry as `internal/generate -> internal/comfy` (T5 Red). The spec names the two paths. | Agent (T1) |
| 2026-11-11 | A test file of a service follows the rules of the service, as the spec says. So T5 step 3 has one option only: a test of `internal/generate` uses a fake of the interface. A test that needs the wire format of the real `comfy.Client` goes to `internal/comfy` or `cmd/horus`. Today `internal/generate/generate_test.go` imports `internal/comfy`. | The spec wins over the plan. The other option of the plan keeps the entry `internal/generate -> internal/comfy` on the list. | Agent (T1) |
| 2026-11-11 | The Red of T1 printed `directory not found` and `[setup failed]` (Go 1.27.3), not "no Go files" or "cannot find package". | It is the same fact: the package does not exist. | Agent (T1) |
| 2026-11-11 | `tools/goal_check.py` does not change in T1. It reads the three lists in the format of T0 and prints their lengths. | The format of T0 fits the lists of T1. | Agent (T1) |
| 2026-11-11 | The interface of `internal/platform` is 9 functions: `StartTree`, `EndTree`, `NoConsole`, `FreeSpace`, `IsRooted`, `VenvBin`, `ExeName`, `PythonLauncher` and `BlockDelete`. The id of a tree is a string: the name of the Job Object on Windows, the process group number on Unix. A caller stores the id and gives it to `EndTree`. | Design rule 2: a caller sees no SysProcAttr, handle or signal. A string id can go to a file, so a later process (T7 `Backend.Stop`) can end the tree. | Agent (T2) |
| 2026-11-11 | On Windows, `StartTree` copies a handle of the Job Object into the root process. | Windows removes the name of a job when its last handle closes, also while processes of the job run. Measured: with the copy in the starter instead of the root, check 2 of the tree test fails with `2 of 2 processes alive 5 s after EndTree`. Limit: when the root ends and its children still run, the name is gone, and `EndTree` reaches nothing. For ComfyUI the root is the server, so this limit does not apply. | Agent (T2) |
| 2026-11-11 | On Windows, the root starts suspended (`CREATE_SUSPENDED`). `StartTree` puts it in the job, then resumes its thread. The standard library `syscall` with kernel32 procedures is enough; no new dependency. | The root cannot start a child before it is in the job. | Agent (T2) |
| 2026-11-11 | A tree on Windows starts with `CREATE_NO_WINDOW` and `CREATE_NEW_PROCESS_GROUP`. `horus start-comfyui` used `DETACHED_PROCESS` before. | With `DETACHED_PROCESS` the root has no console, so each console program that ComfyUI starts opens a new window. With `CREATE_NO_WINDOW` the tree shares one hidden console. The tree still gets no Ctrl+C and still lives after its starter. | Agent (T2) |
| 2026-11-11 | The `horus workflow` child of `internal/models` now starts with `StartTree`. A cancel ends the tree with `EndTree`. On Windows: `TerminateJobObject` instead of `taskkill /T /F`. On Unix: a new session instead of a new process group, and SIGTERM, a wait of at most 3 s (`EndWait`), then SIGKILL, instead of SIGKILL at once. | The plan removes `taskkill`. The process tree has one implementation. `TestCancelKillsTheProcessTree` of `internal/models` and the contract test still pass. The child is a Go program, and SIGTERM ends it at once. | Agent (T2) |
| 2026-11-11 | `StartTree` does not use `CREATE_BREAKAWAY_FROM_JOB`. The tree is a nested job in the job of its starter, if the starter is in one. | The old code did not break away either. A break-away fails when the outer job does not allow it. The tree test passes on this PC (started from a Claude Code session) and in CI on `windows-latest`. | Agent (T2) |
| 2026-11-11 | `BlockDelete` (the test helper of `TestRemoveThatCannotDeleteGivesDeleteFailed`) is in `internal/platform`, not in a new package `internal/platform/platformtest`. | A new package raised the row `layout` from 12 to 13 packages off target (measured). The ratchet must not move the wrong way. | Agent (T2) |
| 2026-11-11 | The default ComfyUI folder stays in `internal/settings` (`ComfyDir`). `internal/platform` gets only the venv paths. | `settings.ComfyDir` has no code for one OS today. T7 moves the layout of the install to `internal/backend`. | Agent (T2) |
| 2026-11-11 | The free space function of `installcomfyui` now calls `platform.FreeSpace` on the root of the drive. On macOS and Linux it gives the error of `statfs`, not "not supported on this OS yet". | The command refuses on those systems before it reads the free space, so a user sees no change. The two copies of the free space code become one, as the plan says. | Agent (T2) |
| 2026-11-11 | Check 1 of the tree test also sends SIGKILL to the process group of the ended starter on Unix, as a terminal does when it closes. A process of the tree in that group fails check 1. On Windows, check 1 has no such part. | The plan's stub check says that a start that is not detached fails check 1. Without the signal, a child on Unix lives after its parent ends also when it is not detached. | Agent (T2) |
| 2026-11-11 | `horus download-model` keeps its flag handling (`Main` in `command.go`) in `internal/download`. `cmd/horus` calls it. `internal/cli` is not created in T4. | A handler can import services and base packages only. Before T6 the only callee is the adapter `internal/download`, so a file in `internal/cli` would add an entry to `allow_direction.txt`. The list must not grow. The 13 tests of the old package call `Main` and move unchanged. T6 creates `internal/provision` and `internal/cli`, and can move the flag handling then. | Agent (T4) |
| 2026-11-11 | `internal/download` exports the constant `DefaultTokenVar`. `internal/scripts/workflow/install.go` no longer sets `TokenEnvVar`, because the downloader already defaults to the same variable. `workflow_test.go` clears the variable by `download.DefaultTokenVar`. | The token rule flags a string that names the variable. A constant of the download package names it once, in the one allowed place. The lookup (environment first, then the `.env` file) did not change. | Agent (T4) |
| 2026-11-11 | The entry `internal/scripts/workflow -> internal/scripts/downloadmodel` of `allow_direction.txt` is renamed to `internal/scripts/workflow -> internal/download`. The line of `layers.go` for the old package is deleted, because `internal/download` is already in the target table. | A rename is not a new entry. T6 removes it. | Agent (T4) |
| 2026-11-11 | `tools/goal_check.py`: the Phase 1 row `downloader` now tests `./internal/download`. | The old path does not exist. It still needs 9 passed tests (13 pass). | Agent (T4) |

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

## T1

Red, before the package existed: `go test ./internal/archtest` (exit 1):

    stat ...\internal\archtest: directory not found
    FAIL	./internal/archtest [setup failed]

Green: `go test -v ./internal/archtest`:

    --- PASS: TestRules (0.03s)
    --- PASS: TestPlanted (0.01s)
    --- PASS: TestCompare (0.00s)
    ok  	github.com/hieplam/horus/internal/archtest	0.792s

Green: `python tools/goal_check.py --phase 2 --no-ci --server-cmd <worktree>\bin\horus.exe mcp` (exit 1):

    FAIL  layout - 12 packages off target, 10 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 160 Go test functions (baseline 157); 0 changed contract files
    FAIL  os-boundary - 19 entries on the allow list; archtest: ok
    FAIL  direction - 10 entries on the allow list; archtest: ok
    FAIL  process-tree - missing: internal/platform (task T2); 8 taskkill references
    FAIL  backend - missing: internal/backend (task T7)
    FAIL  token - 4 entries on the allow list; archtest: ok
    FAIL  commands - missing: internal/cli, internal/doctor (tasks T8 and T9)
    goal: 1 of 8 rows passed

Green: a new file `internal/catalog/x_windows.go` (`package catalog`), not committed:

    --- FAIL: TestRules (0.03s)
        archtest_test.go:74: os-boundary: internal/catalog/x_windows.go: the file name has the OS name windows (internal/catalog/x_windows.go). Fix the code: the allow list only becomes shorter
    FAIL	github.com/hieplam/horus/internal/archtest	0.662s

Stub check 1: `Scan` changed to `return nil, nil`, not committed. `go test -run TestPlanted ./internal/archtest` fails with 23 lines `the rules flag [], want [...]`, one for each planted bad file:

    --- FAIL: TestPlanted (0.00s)
        archtest_test.go:151: internal/catalog/name_windows.go: the rules flag [], want [os-boundary]
        archtest_test.go:151: internal/catalog/goos.go: the rules flag [], want [os-boundary]
        ...

Stub check 2: the extra line `internal/catalog/catalog.go` in `allow_token.txt`, not committed:

    --- FAIL: TestRules (0.03s)
        archtest_test.go:74: token: "internal/catalog/catalog.go" no longer breaks the rule: remove this entry from allow_token.txt

Planted files: 34 under `internal/archtest/testdata/planted`. 23 are bad (os-boundary 8, direction 12, token 3). 11 must not be flagged: the two exempt packages, a build tag that is not an OS, a handler that imports a service, a test file that imports `comfytest`, the wiring, a package that imports itself, the download package and the refusal with `HF_TOKEN`, `HF_TOKEN_INVALID`, and a comment.

Ratchet: rules that a test enforces 0 -> 3. Baseline of the next tasks: `os-boundary` 19 files, `direction` 10 pairs, `token` 4 files.

Side measure: `layout` now reports 10 listed packages missing (11 at T0), because `internal/archtest` exists.

## T2

Red (before the change, at `5582e6a`):

    go test ./internal/platform
    stat ...\internal\platform: directory not found
    FAIL	./internal/platform [setup failed]

    python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>/horus.exe mcp
    FAIL  os-boundary - 19 entries on the allow list; archtest: ok
    FAIL  process-tree - missing: internal/platform (task T2); 8 taskkill references

Green on this PC (Windows 11):

    go test -count=1 -v ./internal/platform
    --- PASS: TestVenvPathsFollowTheOS (0.00s)
    --- PASS: TestIsRootedIsIsPathRootedOfThisOS (0.00s)
    --- PASS: TestTreeOutlivesItsStarterAndEndsFromAnotherProcess (0.26s)
    --- PASS: TestTreeEndsWithItsContext (0.08s)
    --- PASS: TestEndTreeRefusesAnIDThatIsNotATree (0.00s)
    --- PASS: TestFreeSpaceOfThisSystem (0.00s)
    ok  	github.com/hieplam/horus/internal/platform	0.836s

    python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>/horus.exe mcp   (rebased on T3, 7f52f09)
    FAIL  layout - 11 packages off target, 8 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 164 Go test functions (baseline 157); 0 changed contract files
    FAIL  os-boundary - 3 entries on the allow list; archtest: ok
    FAIL  direction - 10 entries on the allow list; archtest: ok
    PASS  process-tree - tree test: ok; 0 taskkill references
    FAIL  backend - missing: internal/backend (task T7)
    FAIL  token - 4 entries on the allow list; archtest: ok
    FAIL  commands - missing: internal/cli, internal/doctor (tasks T8 and T9)
    goal: 2 of 8 rows passed

    git grep -n taskkill -- '*.go'
    (no output, exit 1)

Stub checks, run on this PC:

- `EndTree` that returns nil: `check 2: 2 of 2 processes alive 5 s after EndTree`.
- The job handle copied into the starter, not into the root: `check 2: 2 of 2 processes alive 5 s after EndTree`. The name of the job ends with the handle of the starter.

The 3 files that stay on the allow list `os-boundary` are the `goos` variables of `checksetup.go`, `installcomfyui.go` and `startcomfyui.go` (tasks T7 and T8). The three test files that used `runtime.GOOS` now call `platform.ExeName` and `platform.BlockDelete`, and the venv test moved to `internal/platform`.

Ratchet: `os-boundary` allow list 19 -> 3. `taskkill` references 8 -> 0. Packages off target 11 -> 11 (T2 adds no package outside the list), listed packages missing 9 -> 8 (`internal/platform` exists). Go test functions 160 -> 164. Contract `30 of 30 passed`, parity `17 of 17 cases equal`, `gofmt -l .` prints nothing, `lint-public: 0 finding(s)`.

## T3

Red, before the move: `go list ./internal/mcpserver` (exit 1):

    stat ...\internal\mcpserver: directory not found

Green: `go list ./internal/mcpserver` prints `github.com/hieplam/horus/internal/mcpserver`. `go list ./internal/server` prints `stat ...\internal\server: directory not found`. `go test ./...` passes. `python tests/mcp_contract.py --server-cmd <bin>/horus.exe mcp` prints `contract: 30 of 30 passed`. The golden file shows as a pure rename: `internal/{server => mcpserver}/testdata/tools.golden.json | 0`.

Green: `python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>/horus.exe mcp`:

    FAIL  layout - 11 packages off target, 9 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 160 Go test functions (baseline 157); 0 changed contract files

Ratchet: packages off target 12 -> 11.

Decision: `internal/archtest/layers.go` already lists `internal/mcpserver` as a handler in the target table. So the line `internal/server` was deleted, not renamed. The entry of `allow_direction.txt` was renamed to `internal/mcpserver -> internal/comfy`.

## T4

Red, before the move: `go list ./internal/download` (exit 1):

    stat ...\internal\download: directory not found

Before: `go test -list .` of the old package printed 13 tests. After: `go test -list . ./internal/download` prints the same 13 names (checked with a diff).

Green: `go test ./...` passes. `python tools/goal_check.py --phase 2` prints:

    FAIL  layout - 10 packages off target, 7 listed packages missing
    PASS  same-behaviour - ... parity: 17 of 17; go test ./...: ok; 164 Go test functions (baseline 157); 0 changed contract files
    PASS  token - 0 entries on the allow list; archtest: ok

`python tests/mcp_contract.py --server-cmd <bin>/horus.exe mcp` prints `contract: 30 of 30 passed`. `python tests/workflow_parity.py --horus bin/horus.exe` prints `parity: 17 of 17 cases equal`. `gofmt -l .` prints nothing. `lint-public: 0 finding(s)`. `go vet ./...` is clean for windows, darwin/arm64 and linux/amd64.

Ratchet: `token` allow list 4 -> 0. Packages off target 11 -> 10. Go test functions 164 -> 164. The `direction` list keeps 10 entries (one renamed).

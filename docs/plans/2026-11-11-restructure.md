# Plan: restructure horus into three layers (Phase 2)

Spec: [`../specs/2026-11-11-restructure.md`](../specs/2026-11-11-restructure.md).
Epic: [#52](https://github.com/hieplam/horus/issues/52). Evidence log:
[`2026-11-11-restructure-evidence.md`](2026-11-11-restructure-evidence.md).

## How to read this plan

- This file wins over the copy of a section in an issue.
- Read the spec first: the design rules, the layer table, the import rules and
  the standing rules apply to every task.
- Each task is one pull request. Its body has `Closes #<issue>` and a
  `## Verify` section with **Red**, **Green** and **Stub check**, each with the
  literal result. In that section, indent every line that starts with `# `
  (the check tool ends the section at such a line).
- Each task adds one section `## <task id>` to the evidence log, with the
  literal output of its Green commands. A decision goes to the "Decisions"
  section of the evidence log.
- Before a pull request: `gofmt -l .` prints nothing, `go test ./...` passes,
  `python scripts/lint-public.py` exits 0, and
  `tests/mcp_contract.py` ends with `contract: 30 of 30 passed`.
- The commands below use `python` for the Python of the test tools (3.12 with
  `pip install -r tests/requirements.txt`) and `horus` for a fresh build
  (`go install ./cmd/horus`).
- "The goal tool" is `python tools/goal_check.py --phase 2 --no-ci`. It prints
  one line for each of the 8 rows and then `goal: N of 8 rows passed`. Each
  archtest row also prints the length of its allow list.

## T0 - Baseline and goal tool

Files: `tools/goal_check.py`, `tools/phase2_packages.txt`, the evidence log.

Steps:

1. Add `--phase 2` to `tools/goal_check.py`. Without `--phase`, the tool checks
   the 8 rows of Phase 1 as before. With `--phase 2` it checks the 8 rows of
   the Phase 2 spec. A row whose tool does not exist yet fails with a detail
   that names what is missing. A row never raises.
2. Commit `tools/phase2_packages.txt`: the package list of the spec's layer
   table, one import path on each line.
3. Row details: `layout` prints the number of packages that differ from the
   list. `same-behaviour` runs the contract test, the parity test and
   `go test ./...`, compares the number of Go test functions
   (`go test -list . ./...`) with the baseline number stored in the tool, and
   fails when `git diff --stat <baseline commit> -- tests/mcp_contract.py
   tests/workflow_parity.py tests/fixtures '*golden*'` is not empty. The rows
   `os-boundary`, `direction` and `token` run `go test ./internal/archtest` and
   read the allow lists. `process-tree` runs the tree test of
   `internal/platform` and counts `taskkill` in Go files. `backend` and
   `commands` run their Go tests. With `--real`, `backend` also makes the real
   image and `commands` runs `horus doctor` and `horus setup it-illustration`.
4. Create the evidence log with a "Decisions" section and a "Baseline" section:
   the output of the goal tool, the 12 packages off target, the number of Go
   test functions, the number of `taskkill` references, the files that hold
   `HF_TOKEN`.

Verify:

- **Goal:** every row of the spec has one committed command that measures it
  (rows: all).
- **Red:** `python tools/goal_check.py --phase 2 --no-ci` exits 2 with
  `unrecognized arguments: --phase 2`.
- **Green:** the same command prints 8 row lines and
  `goal: 1 of 8 rows passed` (only `same-behaviour` passes), and exits 1.
  `python tools/goal_check.py --no-ci` still prints the Phase 1 result of
  `main`.
- **Stub check:** without the code, the command fails on its arguments. A tool
  that prints PASS for every row gives `8 of 8`, not `1 of 8`.
- **Ratchet:** rows with a measuring command: 0 → 8.

## T1 - archtest with allow lists

Files: `internal/archtest/` (new), `tools/goal_check.py` (read the lists).

Steps:

1. Write `internal/archtest`: it walks every `.go` file of the module and
   applies three rules of the spec: `os-boundary`, `direction`, `token`.
2. The layer of each package is one table in `internal/archtest/layers.go`,
   copied from the spec. A package that the table does not name gets the
   layer of the package that its code moves to (for example
   `internal/scripts/workflow` counts as a service), so that the lists show
   the real work.
3. Each rule has an allow list in `internal/archtest/allow_<rule>.txt`: the
   files (or `package -> import` pairs) that break the rule today. An entry
   that no longer breaks the rule fails the test with "remove this entry", so
   the list is always exact.
4. `testdata/` holds one planted bad file for each case of each rule. A test
   fails when the scanner does not flag one of them.

Verify:

- **Goal:** rows `os-boundary`, `direction`, `token`: a broken rule fails a
  test.
- **Red:** `go test ./internal/archtest` fails with "no Go files" or "cannot
  find package".
- **Green:** `go test ./internal/archtest` prints `ok`. The goal tool prints
  the three allow list lengths: `os-boundary` 19 (the count of the baseline),
  `direction` and `token` with the measured numbers. A new file
  `internal/catalog/x_windows.go` on a scratch branch makes the test fail.
- **Stub check:** an empty scanner does not flag the planted files, and the
  planted-file test fails. An allow list with an extra entry fails with
  "remove this entry".
- **Ratchet:** rules that a test enforces: 0 → 3. The three list lengths are
  the baseline of the next tasks.

## T2 - internal/platform

Files: `internal/platform/` (new), the 5 packages that hold OS files today.

Steps:

1. Create `internal/platform` with a small interface for callers:
   - start a child without a console window;
   - start a detached process tree that keeps running after its starter ends,
     and end that tree later from another process (Windows: a named Job Object
     without the "end on close" flag, then `TerminateJobObject`. Unix: a new
     session and process group, `SIGTERM`, then `SIGKILL` after a wait);
   - free disk space of a folder;
   - the default folders for each OS (ComfyUI folder, Python of a venv);
   - "is this an absolute path of this OS".
2. Move the code of `internal/comfy/start_*.go`, `internal/models/kill_*.go`,
   `internal/scripts/downloadmodel/freespace_*.go`,
   `internal/scripts/installcomfyui/diskfree_*.go`,
   `internal/scripts/startcomfyui/detach_*.go`, and the `runtime.GOOS` uses of
   `installcomfymcp` and `workflow/files.go`, behind those functions. The two
   free-space copies become one.
3. Write the tree test: a test helper program starts a detached child that
   starts a grandchild. Check 1: both live after the starter ends. Check 2:
   a second process ends the tree, and 0 processes live after 5 s.
4. Remove `taskkill` from the code.
5. Remove from the allow list of `os-boundary` every file that this task
   cleans. The files that stay are the three `goos` variables of
   `installcomfyui.go`, `startcomfyui.go` and `checksetup.go` (tasks T7 and
   T8 move them), and test files that only skip by OS, if any.

Verify:

- **Goal:** row `process-tree`, and most of row `os-boundary`.
- **Red:** `go test ./internal/platform` fails with "cannot find package". The
  goal tool prints `os-boundary` with 19 files.
- **Green:** `go test ./internal/platform` prints `ok` in the CI jobs
  `go windows/amd64`, `go darwin/arm64` and `go linux/amd64`. The goal tool
  prints `PASS  process-tree` and `os-boundary` with at most 4 files.
  `git grep -n taskkill -- '*.go'` prints nothing.
- **Stub check:** a `KillTree` that does nothing leaves the grandchild alive,
  and check 2 fails. A start that is not detached fails check 1.
- **Ratchet:** `os-boundary` allow list 19 → at most 4. `taskkill` references
  → 0.

## T3 - internal/server becomes internal/mcpserver

Files: `internal/server/` → `internal/mcpserver/`, `cmd/horus`.

Steps:

1. Move the package with `git mv`. Change the package name and the imports.
2. Change nothing else. The tools, the schemas and
   `testdata/tools.golden.json` stay byte for byte.

Verify:

- **Goal:** row `layout` (1 of the 12 packages), guarded by `same-behaviour`.
- **Red:** `go list ./internal/mcpserver` fails with "cannot find package".
- **Green:** `go list ./internal/mcpserver` prints the package.
  `go list ./internal/server` fails. `go test ./...` passes. The contract test
  prints `contract: 30 of 30 passed`. `git diff --stat main -- '*golden*'`
  shows only the rename.
- **Stub check:** without the move, the first two commands give the opposite
  results.
- **Ratchet:** packages off target 12 → 11.

## T4 - internal/download

Files: `internal/scripts/downloadmodel/` → `internal/download/`,
`internal/cli/download_model.go` (new package `internal/cli`).

Steps:

1. Move the downloader to `internal/download`. Its exported surface is one
   function that downloads one file with resume, size check and SHA-256
   check, plus its options and its typed error.
2. `internal/download` reads `HF_TOKEN` itself (from the environment, then
   from `.env`). No other package passes the token to it.
3. Create `internal/cli` and move the flag handling of `horus download-model`
   there (`command.go`). `cmd/horus` calls `internal/cli`.
4. Shorten the allow lists of `token`, `direction` and `layout`.

Verify:

- **Goal:** row `token`, and row `layout` (1 package).
- **Red:** `go list ./internal/download` fails with "cannot find package".
- **Green:** `go test ./internal/download` prints `ok` with every test of the
  old package (compare `go test -list .` before and after: the same names).
  The goal tool prints `PASS  token` with 0 files on its allow list.
  The parity test prints `parity: 17 of 17 cases equal`.
- **Stub check:** an empty `internal/download` has no tests, so the count of
  Go test functions falls below the baseline and `same-behaviour` fails.
- **Ratchet:** `token` allow list → 0. Packages off target 11 → 10.

## T5 - internal/graph, and generate behind interfaces

Files: `internal/graph/` (new), `internal/comfy`, `internal/generate`,
`internal/cli/run_illustration.go`, `internal/mcpserver`, `cmd/horus`.

Steps:

1. Move the pure graph code (`internal/comfy/graph.go`, `pyjson.go` and their
   tests) to `internal/graph`. The golden file test of the graph moves with
   it and stays equal.
2. `internal/generate` declares the interface of what it needs from ComfyUI
   (for example: make sure that it is up, read the node information, submit a
   graph, poll a job, cancel a job, fetch an image). Keep the interface as
   small as the service allows: if the service only passes a value through,
   the adapter must do that work.
3. `internal/generate` imports no adapter. `cmd/horus` creates the
   `comfy.Client` and gives it to the service. The tests of `generate` use a
   fake that implements the interface, or the HTTP fake of
   `internal/comfy/comfytest` through the real client where a test needs the
   wire format.
4. `internal/mcpserver` gets the generate service from `cmd/horus`. It does
   not create a `comfy.Client`.
5. Move the flag handling of `horus run-illustration` to `internal/cli`.
6. Shorten the allow lists of `direction` and `layout`.

Verify:

- **Goal:** row `direction` for the image path, guarded by `same-behaviour`.
- **Red:** `go list ./internal/graph` fails. The `direction` allow list holds
  `internal/generate -> internal/comfy`.
- **Green:** `go list -f '{{join .Imports "\n"}}' ./internal/generate` prints
  no line with `internal/comfy`, `net/http` or `os/exec`. `go test ./...`
  passes with no fewer test functions. The contract test prints
  `contract: 30 of 30 passed`. The graph golden file is unchanged.
- **Stub check:** an empty change leaves the import, and archtest fails
  because this task removes the entry from the allow list.
- **Ratchet:** `direction` allow list: every entry of `generate`, of
  `runillustration` and of the image tools of `mcpserver` → gone. Packages
  off target 10 → 9.

## T6 - internal/modelfiles and internal/provision

Files: `internal/scripts/workflow/`, `internal/models/` (deleted),
`internal/modelfiles/` (new), `internal/provision/` (new),
`internal/cli/workflow.go`, `internal/mcpserver`, `cmd/horus`.

Steps:

1. `internal/modelfiles` (adapter): where a model file lives (the folder
   order of the spec of Phase 1), the size and SHA-256 check, remove with the
   refusal rules, the `.part` file, `extra_model_paths.yaml` with its backup.
2. `internal/provision` (service): `List`, `Check`, `Install` and `Remove` of
   a workflow. It declares the interfaces that it needs (model files,
   downloader) and returns typed results: the items with the 17 statuses of
   the Phase 1 spec.
3. `internal/cli/workflow.go` is the handler of `horus workflow`. It prints the
   same text and the same `HORUS_JSON` line as before.
4. `internal/mcpserver` calls `internal/provision` in its own process. Delete
   `internal/models` (the child process, the parser of the `HORUS_JSON` line,
   the text matching of `error: 401`). The mapping from an item status to an
   error code stays, on typed values. A cancel of `install_workflow` ends the
   download through the context, and the `.part` file stays.
5. Shorten the allow lists.

Verify:

- **Goal:** rows `layout` (2 packages) and `direction` for the setup path,
  guarded by `same-behaviour`.
- **Red:** `go list ./internal/provision ./internal/modelfiles` fails.
  `git grep -n "HORUS_JSON" -- internal/mcpserver internal/provision`
  is not the target yet: `internal/models` holds the parser.
- **Green:** the parity test prints `parity: 17 of 17 cases equal`. The
  contract test prints `contract: 30 of 30 passed` (its install, uninstall and
  cancel checks pass). `go list ./internal/models ./internal/scripts/workflow`
  fails. `git grep -n "os/exec" -- internal/mcpserver internal/provision`
  prints nothing. A Go test of `internal/mcpserver` cancels an install against
  a slow fake downloader and finds the `.part` file.
- **Stub check:** a `provision` that does nothing fails the install and
  uninstall checks of the contract test and the 17 parity cases.
- **Ratchet:** packages off target 9 → 7. `direction` allow list: every entry
  of `workflow` and `models` → gone.

## T7 - internal/backend

Files: `internal/backend/` (new), `internal/scripts/installcomfyui/`,
`internal/scripts/startcomfyui/` (both deleted), `internal/comfy` (autostart
removed), `internal/generate`, `internal/provision`, `internal/cli`.

Steps:

1. Read issue #53 (the Mac test run). If it has a result, make sure that the
   interface also fits what the run recorded for `comfy-cli`: the layout, the
   start command and the stop command. If it has no result yet, use the
   reference facts of #4, and record that in the evidence log.
2. `internal/backend`: the Windows portable backend with `Install` (the pinned
   archive, size, SHA-256, free space, 7-Zip), `Start` (detached, in a named
   Job Object, wait up to 300 s), `Stop` (end the tree of a ComfyUI that horus
   started, never another one) and `Layout` (the folders of the install). On
   another OS the constructor returns a backend whose calls answer "not
   supported on this OS yet", as today.
3. The services declare the part that they need: `generate` needs "start when
   it is down", `provision` needs `Install` and `Layout`.
4. Remove the autostart child process from `internal/comfy`. The contract
   behaviour stays: `BACKEND_UNAVAILABLE`, `BACKEND_NOT_INSTALLED` and
   `HORUS_AUTOSTART` act as before.
5. Move the handlers of `horus install-comfyui` and `horus start-comfyui` to
   `internal/cli`. Add `horus stop-comfyui`.
6. Shorten the allow lists.

Verify:

- **Goal:** row `backend`, and the rest of row `os-boundary` for the backend
  code.
- **Red:** `go list ./internal/backend` fails with "cannot find package".
- **Green:** `go test ./internal/backend ./internal/generate` prints `ok`: a
  test with a fake backend proves that a generate call on a ComfyUI that is
  down calls `Start` one time, and a test with a stand-in process proves that
  `Stop` ends it and refuses a process that horus did not start. The contract
  test prints `contract: 30 of 30 passed`. On the owner's PC:
  `horus stop-comfyui`, then
  `python tools/goal_check.py --phase 2 --real` prints `PASS  backend` with a
  1536x1024 PNG.
- **Stub check:** a `Start` that does nothing leaves ComfyUI down, and the
  real run fails with `BACKEND_UNAVAILABLE`. A `Stop` that does nothing leaves
  the stand-in process alive.
- **Ratchet:** packages off target 7 → 5. Images after a start through the
  backend 0 → 1.

## T8 - Commands: doctor, setup, and the end of internal/scripts

Files: `internal/doctor/` (new), `internal/devtools/` (new),
`internal/provision`, `internal/cli`, `cmd/horus`, `internal/scripts/`
(deleted).

Steps:

1. `internal/doctor` holds the checks of `check-setup`. `horus doctor` and
   `horus check-setup` print the same lines.
2. `horus setup <workflow>`: the handler calls `provision`. The service makes
   a plan (backend installed or not, each model file present or not) and the
   handler prints it. With `-yes` the service runs the plan: install the
   backend when it is missing, then the model files.
3. Move `installcomfymcp` and `register` into `internal/provision`, and
   `newworkflow` and `syncworkflows` into `internal/devtools`. Their handlers
   go to `internal/cli`.
4. `cmd/horus` holds the command table and the wiring only.
5. Delete `internal/scripts`. Every allow list of archtest is empty. Remove
   the allow list mechanism from the tests, so a new entry is not possible.

Verify:

- **Goal:** rows `commands`, `layout`, `os-boundary`, `direction`.
- **Red:** `horus doctor` exits 2 with an unknown command message. The goal
  tool prints `FAIL  layout` with 5 packages.
- **Green:** `python tools/goal_check.py --phase 2 --no-ci` prints
  `goal: 8 of 8 rows passed` (the `backend` row without the real image).
  `go test ./internal/provision` proves the plan of `horus setup` on a
  temporary folder: nothing present gives 1 backend step and 3 file steps, and
  everything present gives 0 steps. On the owner's PC: `horus doctor` prints
  0 FAIL lines, and `horus setup it-illustration` (no `-yes`) prints a plan
  with 0 steps and changes no file.
- **Stub check:** a `setup` that prints nothing fails the plan test. An
  `internal/scripts` folder that stays fails `layout`.
- **Ratchet:** packages off target 5 → 0. All three allow lists → 0.
  Commands 0 → 2.

## T9 - Docs and the decision log

Files: `AGENTS.md`, `README.md`, `docs/mcp.md`, `docs/setup.md`,
`docs/running-workflows.md`, issue #4.

Steps:

1. `AGENTS.md`: the layout section shows the layers. Add the design rules of
   the spec in 5 lines, with a link to the spec.
2. `docs/mcp.md`: the diagrams no longer show a `horus workflow` child of the
   server. The text on the token says where it is read.
3. `docs/setup.md` and `README.md`: name `horus doctor` and `horus setup`.
   The old names stay valid, and the docs say so in one line.
4. Issue #4: tick the Phase 2 boxes, add the decisions of this phase to the
   decision log, update "Resume here".

Verify:

- **Goal:** a new agent session reads docs that match the code (rows `layout`
  and `commands` as a reader sees them).
- **Red:** `git grep -n -E "internal/(scripts|server|models)" -- '*.md' ':!docs/plans' ':!docs/specs'`
  prints at least one line.
- **Green:** the same command prints nothing. `python scripts/lint-public.py`
  exits 0. The contract test prints `contract: 30 of 30 passed` (it checks
  that `docs/mcp.md` lists every error code).
  `git grep -n "horus doctor" -- docs/setup.md README.md AGENTS.md` prints at
  least 3 lines.
- **Stub check:** without the edits, the Red command still prints lines.
- **Ratchet:** stale package names in the docs: the Red count → 0.

## T10 - Verify every goal row on main

Files: the evidence log.

Steps:

1. On the owner's PC, on a clean checkout of `main`: `go install ./cmd/horus`,
   then `python tools/goal_check.py --phase 2 --real`.
2. Run the check of the epic tool with `--dry-run` on every closed task of the
   epic.
3. Each row that fails becomes an issue with the output, the row and a Verify
   block. The issue blocks this task.

Verify:

- **Goal:** all 8 rows, on `main`, from one tool.
- **Red:** not applicable. The task builds no code.
- **Green:** `python tools/goal_check.py --phase 2 --real` prints
  `goal: 8 of 8 rows passed` with the commit of `main`, and CI on that commit
  shows `contract windows-latest` and `contract macos-latest` green.
- **Stub check:** the tool gave `1 of 8` in T0 on the tree before the work, so
  `8 of 8` comes from the work.
- **Ratchet:** rows passed 1 → 8.

## T11 - Agent acceptance

Files: the evidence log.

Steps:

1. Act as a new developer and as a new agent. With a fresh build, follow
   `README.md` and `docs/setup.md` on the owner's PC with commands that change
   nothing: `horus -h`, `horus doctor`, `horus workflow list`,
   `horus workflow check it-illustration`, `horus setup it-illustration`.
2. Through `claude -p` with the user-scope `horus` MCP server: call
   `list_workflows`, `describe_workflow` and `generate_image`. Open the PNG and
   compare it with the subject.
3. Read the code as a reviewer, against the design rules of the spec: one
   package at a time, is its interface smaller than what it hides? Does a
   service know a detail of an adapter? Is there an interface with no second
   implementation and no fake?
4. Each defect becomes an issue with evidence, its goal row and a Verify
   block, and blocks this task. A finding that threatens no goal row becomes a
   follow-up outside the epic.

Verify:

- **Goal:** all rows, as the owner would look at them.
- **Red:** not applicable. The task builds no code.
- **Green:** the evidence log has the output of each command of step 1, the
  path of the PNG with the judgement, and the list of findings with their
  issue numbers. 0 issues of this task are open.
- **Stub check:** an empty section has no command output and no PNG, and the
  check of the evidence heading fails.
- **Ratchet:** open defects against the goal rows → 0.

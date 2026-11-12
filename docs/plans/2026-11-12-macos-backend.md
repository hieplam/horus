# Plan: horus on macOS through comfy-cli (Phase 3)

Spec: [`../specs/2026-11-12-macos-backend.md`](../specs/2026-11-12-macos-backend.md).
Epic: [#98](https://github.com/hieplam/horus/issues/98). Evidence log:
[`2026-11-12-macos-backend-evidence.md`](2026-11-12-macos-backend-evidence.md).

## How to read this plan

- This file wins over the copy of a section in an issue.
- Read the spec first: the design, the table "Where each task runs" and the
  standing rules apply to every task. Read the result of the Mac test run
  ([#53](https://github.com/hieplam/horus/issues/53)) too: it is the
  requirement list.
- Each task is one pull request. Its title follows Conventional Commits. Its
  body has `Closes #<issue>` and a `## Verify` section with **Red**, **Green**
  and **Stub check**, each with the literal result. In that section, indent
  every line that starts with `# ` (the check tool ends the section at such a
  line).
- Each task adds one section `## <task id>` to the evidence log, with the
  literal output of its Green commands. A decision goes to the "Decisions"
  section of the evidence log.
- Before a pull request: `gofmt -l .` prints nothing, `go test ./...` passes,
  `python scripts/lint-public.py` exits 0, and `tests/mcp_contract.py` ends
  with `contract: 30 of 30 passed`.
- A task line "Runs on" names the machine. "any" means any machine with Go and
  the test tools. The oracle of macOS code in an "any" task is the CI job
  `go darwin/arm64` on `macos-latest`: read its result on the pull request
  before the merge.
- A pull request from the PC (account `hieplam`) is merged by the agent with
  squash when CI is green. A pull request from the Mac (account `hoacao33`) is
  merged by the owner: the session posts the link in the issue and ends.
- The commands below use `python` for the Python of the test tools (3.12 with
  `pip install -r tests/requirements.txt`) and `horus` for a fresh build
  (`go install ./cmd/horus`).
- "The goal tool" is `python tools/goal_check.py --phase 3`. It prints one line
  for each of the 8 rows and then `goal: N of 8 rows passed`.
- "The fake comfy" is a small program with the name `comfy` that a Go test
  builds or writes into a temporary folder. It appends its arguments to a file
  and plays the part that the test needs (it makes `ComfyUI/main.py`, it starts
  a stand-in server, it stops it). It holds no code of comfy-cli.

## T0 - Baseline and goal tool

Runs on: any.

Files: `tools/goal_check.py`, `tools/test_goal_check.py`, the evidence log.

Steps:

1. Add `--phase 3` to `tools/goal_check.py`, with the 8 rows of the spec. A
   row whose code does not exist yet fails with a detail that names what is
   missing. A row never raises.
2. What each row checks without `--real`:
   - `mac-backend`: the Go tests of `internal/backend` whose names the tool
     pins (the fake comfy gets the three commands, `Stop` refuses a ComfyUI
     that horus did not start), and the CI job `go darwin/arm64` on HEAD.
   - `mac-setup`: the pinned Go test of the default folder, and a Go test that
     runs `horus setup` with the fake comfy on a sandbox.
   - `mac-image`: nothing without `--real`. It then fails with "needs --real on
     macOS".
   - `tcc-folders`: the pinned Go tests of the default folder and of the WARN
     line.
   - `out-of-memory`: the two pinned Go tests with the MPS text.
   - `one-config`: `.mcp.json` holds no `USERPROFILE`, no `\` and no `.exe`,
     and both commands are `horus`. The pinned Go test of #82.
   - `one-setup-doc`: the two Red searches of #91 print nothing, and
     `docs/setup.md` has a section for macOS.
   - `same-behaviour`: as the Phase 2 row, with the baseline of this phase:
     commit `943d430`, 215 Go test functions.
3. What `--real` adds. On macOS: `mac-backend` (start through the backend,
   stop, and a refusal for a ComfyUI that horus did not start), `mac-setup`
   (`horus doctor -workflow it-illustration` with 0 FAIL lines, a plan with 0
   steps, and no `HORUS_COMFYUI_DIR` in the environment or in `.env`),
   `mac-image` (one cold image through MCP, `started_backend: true`),
   `tcc-folders` (a WARN line when the repo is inside a TCC folder) and the
   macOS part of `one-config`. On Windows: the Windows part of `one-config`
   (`horus doctor` with 0 FAIL lines) and the Phase 2 rows `backend` and
   `commands` for `same-behaviour`. A row that needs the other machine says so
   in its detail and fails. `--only` picks rows, as in Phase 2.
4. A pinned test name must be in the output of `go test -v -run` with
   `--- PASS`. A package with no test file must fail the row (issue #80 of
   Phase 2).
5. Add the section "Baseline" to the evidence log: the output of the goal
   tool, the number of Go test functions, the three commands that answer "not
   supported" on macOS, the Windows-only paths of `.mcp.json`.

Verify:

- **Goal:** every row of the spec has one committed command that measures it
  (rows: all).
- **Red:** `python tools/goal_check.py --phase 3` exits 2 with
  `argument --phase: invalid choice: 3`.
- **Green:** `python tools/goal_check.py --phase 3 --no-ci` prints 8 row lines
  and `goal: 1 of 8 rows passed` (only `same-behaviour` passes), and exits 1.
  `python -m pytest tools/test_goal_check.py` passes.
- **Stub check:** the tool is the check. A tool that prints PASS for a row
  whose code does not exist fails the Green count "1 of 8". The tests of
  `tools/test_goal_check.py` feed each row a stub (an empty package, a missing
  test name) and need FAIL.
- **Ratchet:** goal rows with a committed check 0 → 8.

## T1 - The comfy-cli backend, part 1: pick by OS, install, layout, default folder

Runs on: any (oracle: CI on `macos-latest`).

Files: `internal/backend/`, `internal/platform/`, `internal/settings/`,
`cmd/horus/wiring.go`, `internal/cli/comfyui.go`.

Steps:

1. `internal/platform`: a pure function for the default install folder of an
   OS (spec, "The default install folder"). `settings.ComfyDir` calls it.
2. `internal/backend`: the comfy-cli backend with `Installed`, `Layout` and
   `Install` (spec table). `Start` and `Stop` answer "not supported on this OS
   yet" until T2. `backend.New` picks by OS. The constructor takes the values
   that both backends need, and the values of the Windows backend only. Keep
   the interface of the package small.
3. The backend finds `comfy` on the PATH, then in the venv of
   `horus install-comfy-mcp`. The error for a missing `comfy` names both
   fixes.
4. `horus install-comfyui` on macOS refuses `-archive-path`, `-seven-zip` and
   `-keep-archive` with exit 2. The help texts name the default folder of the
   OS.
5. Tests with the fake comfy: `Install` runs exactly
   `comfy --workspace <dir>/ComfyUI install --m-series --version 0.44.0`, it
   skips when ComfyUI is installed, and it fails with the fix text when
   `comfy` is missing. The tests run on every OS (they call the backend of
   macOS by its type, as the tests of the Windows backend do).

Verify:

- **Goal:** rows `mac-backend` (install) and `tcc-folders` (the default
  folder).
- **Red:** `go test ./internal/backend -run ComfyCLI -v` prints
  `testing: warning: no tests to run`.
- **Green:** `go test ./internal/backend ./internal/platform ./internal/settings ./internal/cli`
  prints `ok` for each. The job `go darwin/arm64` of the pull request is
  green. `python tools/goal_check.py --phase 3 --no-ci --only tcc-folders`
  names only the WARN test as missing. The contract test prints
  `contract: 30 of 30 passed`, and the parity test `parity: 17 of 17 cases equal`.
- **Stub check:** an `Install` that does nothing leaves the argument file of
  the fake comfy empty, and the test fails. A default folder that stays
  `ComfyUI_windows_portable` fails the test of the macOS default.
- **Ratchet:** commands that answer "not supported" on macOS 3 → 2.

## T2 - The comfy-cli backend, part 2: start, stop, the state file

Runs on: any (oracle: CI on `macos-latest`).

Files: `internal/backend/`.

Steps:

1. `Start` and `Stop` of the comfy-cli backend (spec table, "Why a state
   file", "The stale state file"). Both backends share the code of the state
   file and of the wait for the server. Do not copy it.
2. The fake comfy starts a stand-in server on `launch` and ends it on `stop`.
3. Tests: `Start` runs exactly
   `comfy --workspace <dir>/ComfyUI launch --background -- --listen 127.0.0.1 --port <port>`
   and the server answers. `Stop` runs `comfy --workspace <dir>/ComfyUI stop`
   and the server no longer answers. `Stop` without a state file refuses a
   server that answers, and the argument file shows no `stop`. A stale state
   file runs no `stop`. `Start` on a folder without ComfyUI gives an error
   with `NotInstalled`.
4. A generate call on a ComfyUI that is down starts it through this backend:
   one test in `cmd/horus` with the fake comfy and the fake ComfyUI.

Verify:

- **Goal:** row `mac-backend` (start, stop, the refusal).
- **Red:** `go test ./internal/backend -run 'ComfyCLI.*(Start|Stop)' -v`
  prints `testing: warning: no tests to run`.
- **Green:** `go test ./internal/backend ./cmd/horus` prints `ok`. The job
  `go darwin/arm64` of the pull request is green.
  `python tools/goal_check.py --phase 3 --only mac-backend` prints
  `PASS  mac-backend` for the part without `--real`. The contract test prints
  `contract: 30 of 30 passed`.
- **Stub check:** a `Start` that does nothing leaves the stand-in server down,
  and the test fails at the wait. A `Stop` that always runs `comfy stop` fails
  the refusal test, because the argument file then shows `stop`.
- **Ratchet:** commands that answer "not supported" on macOS 2 → 0.

## T3 - horus doctor and horus setup on macOS

Runs on: any.

Files: `internal/doctor/`, `internal/cli/doctor.go`, `internal/cli/setup.go`,
`internal/platform/`, `cmd/horus/wiring.go`.

Steps:

1. A result of the doctor gets a level: PASS, WARN or FAIL. A WARN does not
   count as failed.
2. The TCC check (spec, "`horus doctor`"). The test for "inside a TCC folder"
   is a pure function of `internal/platform` that takes the OS name, the home
   folder and the path.
3. Issue #82: without `-server-url`, the doctor checks `HORUS_COMFYUI_URL`.
4. The fix texts of the doctor and the texts of `horus setup` name no Windows
   path on macOS.
5. A Go test runs `horus setup <workflow> -yes` on a sandbox with the fake
   comfy and a fake download server: ComfyUI is installed first, then the
   files, and a second plan has 0 steps.

Verify:

- **Goal:** rows `tcc-folders` (the warning), `one-config` (#82) and
  `mac-setup` (the Go part).
- **Red:** the Red of #82: a test of `internal/cli` with `HORUS_COMFYUI_URL`
  set to a test server and no `-server-url` flag. The doctor asks
  `http://127.0.0.1:8188`, and the test fails.
- **Green:** `go test ./internal/doctor ./internal/cli ./internal/platform ./cmd/horus`
  prints `ok`. `python tools/goal_check.py --phase 3 --no-ci --only tcc-folders`
  prints `PASS  tcc-folders` for the part without `--real`. On the PC,
  `horus doctor` prints the same lines as before the change (`diff` prints
  nothing). The contract test prints `contract: 30 of 30 passed`.
- **Stub check:** a TCC function that always answers "outside" fails the test
  of a path in `~/Desktop`. A WARN that counts as failed fails the test of the
  exit code 0. A fixed default address fails the test of #82.
- **Ratchet:** open issue #82 closes. Levels of a doctor line 2 → 3.

## T4 - OUT_OF_MEMORY for the MPS text

Runs on: any.

Files: `internal/comfy/` (tests), `cmd/horus/image_test.go`.

Steps:

1. Add the MPS text of PyTorch as a case of the job error mapping in
   `internal/comfy`: `MPS backend out of memory (MPS allocated: 9.07 GB, other allocations: 6.50 GB, max allowed: 18.13 GB). Tried to allocate 256.00 MB on private pool. Use PYTORCH_MPS_HIGH_WATERMARK_RATIO=0.0 to disable upper limit for memory allocations (may cause system failure).`
2. Add a case to the wiring test of `cmd/horus`: a failed job with that text
   gives the envelope code `OUT_OF_MEMORY`.
3. Read the `next_step` text of `OUT_OF_MEMORY`. If it names a thing that a
   Mac does not have, record it as a decision. The text of the 21 codes is a
   golden file and does not change in this phase. File a follow-up outside the
   epic if the text misleads a Mac user.
4. Record in the evidence log: the time limit stays 1800 s (spec).

Verify:

- **Goal:** row `out-of-memory`.
- **Red:** not applicable as a failing run: the code already maps the text
  (spec). The Red is the missing proof:
  `git grep -n "MPS backend out of memory" -- '*.go'` prints nothing.
- **Green:** the same search prints 2 lines or more.
  `go test ./internal/comfy ./cmd/horus` prints `ok`.
  `python tools/goal_check.py --phase 3 --no-ci --only out-of-memory` prints
  `PASS  out-of-memory`.
- **Stub check:** in a scratch copy, remove the three text matches of
  `internal/comfy/jobs.go`. The new case must fail. Record the output.
- **Ratchet:** test cases with the MPS text 0 → 2.

## T5 - One .mcp.json

Runs on: any, then the PC for the last Green command.

Files: `.mcp.json`, `cmd/horus/main.go`, `internal/cli/mcptools.go`,
`internal/provision/comfymcp.go`, `internal/doctor/`, `tools/goal_check.py`
(only if a Phase 1 row reads the old entry).

Steps:

1. The command `horus comfy-mcp` (spec, "One `.mcp.json`"). The handler is in
   `internal/cli`. The work is a function of `internal/provision` on its
   `Programs` interface. `platform.Command` gets what it needs to pass the
   standard input and output through.
2. `.mcp.json` as in the spec.
3. The checks 3, 4 and 5 of the doctor follow the new entry: the command of
   the entry is `horus`, and `comfy-mcp` and `comfy` are found in the venv or
   on the PATH. A `.mcp.json` with the old Windows paths still passes when the
   files exist.
4. `horus install-comfy-mcp` keeps its venv. Its last line names
   `horus comfy-mcp`.
5. A Go test starts `horus comfy-mcp` with a fake `comfy-mcp` program that
   echoes its standard input and prints `COMFY_BIN`: the bytes pass through,
   the variable names the `comfy` next to it, and the exit code is the exit
   code of the fake.

Verify:

- **Goal:** row `one-config`.
- **Red:** `git grep -n "USERPROFILE" -- .mcp.json` prints 2 lines.
  `horus comfy-mcp` exits 2 with "unknown command".
- **Green:** the search prints nothing.
  `go test ./internal/cli ./internal/provision ./internal/doctor ./cmd/horus`
  prints `ok`. `python tools/goal_check.py --phase 3 --no-ci --only one-config`
  prints `PASS  one-config` for the part without `--real`. On the PC, with the
  new binary: `horus doctor` prints 0 FAIL lines, and a new `claude` session
  in the repo shows `comfy-mcp` and `horus` as connected (`claude mcp list`).
- **Stub check:** a `horus comfy-mcp` that exits 0 and starts nothing fails
  the echo test. A doctor that does not read the entry passes a `.mcp.json`
  whose command does not exist: the existing test of that case must still
  fail it.
- **Ratchet:** Windows-only paths in `.mcp.json` 2 → 0.

## T6 - Docs: one setup page for both systems

Runs on: any.

Files: `docs/setup.md`, `README.md`, `AGENTS.md`, `docs/mcp.md`,
`docs/concepts.md`, `docs/running-workflows.md`, `ROADMAP.md`, the help texts
in `internal/cli/`, issue #4.

Steps:

1. `docs/setup.md` becomes one page for Windows and macOS. The steps that
   differ stand side by side: the prerequisites (a Mac needs Apple Silicon,
   16 GB, Python 3.12 and Go), the install of ComfyUI, the default folder, the
   time of the first image (#53: 1033 s cold, 469 s warm). It names the TCC
   folders and the WARN line. It says that Linux is not supported yet.
2. Issue #91: the three sentences and the help texts.
3. `AGENTS.md`: the table "Read before you act" and the layout list. The
   sentence about the OS boundary stays true.
4. `docs/mcp.md`: the settings table (the default of `HORUS_COMFYUI_DIR` for
   each OS), `horus comfy-mcp`.
5. Issue #4: tick the Phase 3 boxes that are built, add the decisions of this
   phase to the decision log (D42 and later), update "Resume here".

Verify:

- **Goal:** row `one-setup-doc`.
- **Red:** the Red of #91:
  `git grep -n "Two commands are\|does steps 1 and 4" -- docs/setup.md` prints
  2 lines, and `git grep -n "then ~/ComfyUI_windows_portable" -- internal/cli`
  prints 2 lines. `git grep -n "not supported on this OS yet" -- docs/setup.md`
  names macOS.
- **Green:** both searches of #91 print nothing.
  `python tools/goal_check.py --phase 3 --no-ci --only one-setup-doc` prints
  `PASS  one-setup-doc`. `python scripts/lint-public.py` exits 0.
  `go test ./...` prints `ok` for every package.
- **Stub check:** doing nothing leaves the 4 lines of Red, and the goal tool
  finds no macOS section.
- **Ratchet:** sentences that the commands contradict 4 → 0. Open issue #91
  closes.

## T7 - The real run on the Mac

Runs on: the Mac. The pull request waits for the owner's merge.

Files: the evidence log, `workflows/it-illustration/README.md` (the macOS
evidence), `workflows/it-illustration/examples/` (the PNG, if the README shows
it).

Steps:

1. Before the run: `git pull`, `go install ./cmd/horus`. `~/horus` must not
   exist. If it exists, stop and say so in the issue. Take the line
   `HORUS_COMFYUI_DIR` out of `.env` (keep the token line). Do not touch
   `~/horus-lab`.
2. `horus install-comfy-mcp` (or use the `comfy` of `uv tool`, if it is on the
   PATH). Record which `comfy` the backend used.
3. `horus setup it-illustration` (the plan), then
   `horus setup it-illustration -yes`. Record the time and every line.
4. `horus doctor -workflow it-illustration`. Then
   `horus setup it-illustration` again.
5. `python tools/goal_check.py --phase 3 --real --only mac-backend mac-setup mac-image tcc-folders one-config`.
   Open the PNG and judge it against the subject.
6. `horus stop-comfyui`. Then start a ComfyUI by hand
   (`comfy --workspace ~/horus/ComfyUI launch --background -- --listen 127.0.0.1 --port 8188`)
   and run `horus stop-comfyui`: it must refuse. Stop that ComfyUI with
   `comfy stop`.
7. Each defect becomes an issue with its evidence, the goal row and a Verify
   block. It blocks this task (`E release <task> --found <n>`).

Verify:

- **Goal:** rows `mac-backend`, `mac-setup`, `mac-image`, `tcc-folders` and
  `one-config`, on the real machine.
- **Red:** on the Mac, before step 3: `horus doctor` prints
  `FAIL  ComfyUI folder` for `~/horus`.
- **Green:** step 4 prints 0 FAIL lines and `Plan: 0 steps. Nothing is missing.`
  Step 5 prints `goal: 5 of 5 rows passed`, with a 1536x1024 PNG and
  `started_backend: true`. Step 6 prints the refusal ("horus did not start the
  ComfyUI that answers at ...") and exits 1, and the server still answers
  after it.
- **Stub check:** the "not supported" backend of `main` before T1 fails step 3
  at the install. An image from a ComfyUI that a person started fails
  `started_backend: true`.
- **Ratchet:** images through MCP on macOS 0 → 1. Manual steps of #53 3 → 2.

## T8 - Verify on main, on the PC

Runs on: the PC.

Files: the evidence log, `tools/goal_check.py` (only fixes that a stub probe
needs, as their own issue).

Steps:

1. On `main`: `go install ./cmd/horus` (rename the old file if `horus mcp`
   holds it).
2. `python tools/goal_check.py --phase 3` (with the CI part): every row that
   needs no real machine.
3. `python tools/goal_check.py --phase 3 --real --only one-config` with the
   owner's settings. Then the Phase 2 rows:
   `python tools/goal_check.py --phase 2 --real --only commands`, and
   `--real --only backend` with the port 8189 method (standing rules).
4. Stub probes of the goal tool: for each row, one empty implementation in a
   scratch worktree (a removed test, an old `.mcp.json`, an empty package).
   Each probe must give FAIL. A probe that passes is an issue that blocks this
   task.
5. `E check <n> --dry-run` for every closed task of the epic.
6. The ratchet table of the spec, before → after, in the evidence log.

Verify:

- **Goal:** rows `out-of-memory`, `one-config`, `one-setup-doc`,
  `same-behaviour`, and the parts without a real Mac of the other rows.
- **Red:** not applicable: this task builds nothing. Each stub probe is a Red
  of its own.
- **Green:** step 2 prints `PASS` for `out-of-memory`, `one-setup-doc` and
  `same-behaviour`, and names "needs --real" for the others. Step 3 prints
  `PASS  one-config`, `PASS  commands` and `PASS  backend` with a 1536x1024
  PNG. Every probe of step 4 prints `FAIL`. Every check of step 5 prints PASS.
- **Stub check:** step 4 is the stub check of the tool itself.
- **Ratchet:** the table of the spec: each number at its "after" value, except
  the rows of the real Mac.

## T9 - Verify on main, on the Mac

Runs on: the Mac. The pull request waits for the owner's merge.

Files: the evidence log.

Steps:

1. `git pull`, `go install ./cmd/horus`. `horus stop-comfyui`, so that the
   image run is cold.
2. `python tools/goal_check.py --phase 3 --real` on the commit of `main`.
3. Open the PNG and judge it against the subject.

Verify:

- **Goal:** all 8 rows on `main`.
- **Red:** not applicable: this task builds nothing.
- **Green:** `goal: 8 of 8 rows passed` on the Mac, for the commit that T8
  verified or a later one. The evidence log names the commit, the cold time
  and the PNG.
- **Stub check:** with `HORUS_AUTOSTART=0` and ComfyUI stopped, the row
  `mac-image` must fail with `BACKEND_UNAVAILABLE`. Record the output.
- **Ratchet:** goal rows that pass on `main` 0 → 8.

## T10 - Agent acceptance

Runs on: the Mac. The pull request waits for the owner's merge.

Files: the evidence log.

Steps:

1. **Part A, a new user.** A new agent session gets only `README.md` and
   `docs/setup.md`. It sets up a second install with
   `HORUS_COMFYUI_DIR=~/horus-accept` and `HORUS_COMFYUI_URL` on port 8189,
   and `HORUS_MODELS_DIR` on the model folder of `~/horus/ComfyUI/models` (no
   second download). It notes every step that the page does not name and
   every sentence that a command contradicts. At the end it stops that
   ComfyUI and removes `~/horus-accept`.
2. **Part B, a new agent through MCP.** `claude -p` with a config that starts
   `horus mcp` and allows only `list_workflows`, `describe_workflow` and
   `generate_image`. It makes one image from a subject of its own and judges
   the PNG.
3. **Part C, the design.** Read `internal/backend`, `internal/platform`,
   `internal/doctor`, `internal/provision` and the wiring, package by package,
   against the design rules: a small interface, no copy of code between the
   two backends, no OS test outside `internal/platform` and
   `internal/backend`, pure functions for the rules. The archtest rules are
   not enough: read the code.
4. Each defect against a goal row or a design rule becomes an issue that
   blocks this task. Anything else is a follow-up outside the epic.

Verify:

- **Goal:** the acceptance bar of the spec: no open defect.
- **Red:** not applicable: this task builds nothing.
- **Green:** part A reaches an image with 0 steps that the page does not
  name. Part B returns `ok: true` and a PNG that shows its subject. Part C
  lists each package with "no defect" or an issue number. No issue that this
  task filed is open.
- **Stub check:** part A on the `docs/setup.md` of `943d430` fails at the
  install of ComfyUI on macOS ("not supported on this OS yet").
- **Ratchet:** open defects of the acceptance → 0.

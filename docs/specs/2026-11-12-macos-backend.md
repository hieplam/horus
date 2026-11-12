# Spec: horus on macOS through comfy-cli (Phase 3)

Date: 2026-11-12. Tracking issue of the whole goal:
[#4](https://github.com/hieplam/horus/issues/4). Epic of this phase:
[#98](https://github.com/hieplam/horus/issues/98). Plan:
[`../plans/2026-11-12-macos-backend.md`](../plans/2026-11-12-macos-backend.md).

## What this phase delivers

On a Mac with Apple Silicon, `horus setup it-illustration -yes` installs ComfyUI
through comfy-cli and downloads the model files. Then `generate_image` of the
horus MCP server makes an image, and horus starts and stops ComfyUI itself.
Windows keeps its behaviour. One `.mcp.json` and one `docs/setup.md` serve both
systems.

The requirement list is the result of the Mac test run
([#53](https://github.com/hieplam/horus/issues/53)).

Terms:

- **comfy-cli:** the command-line installer of Comfy Org, the program `comfy`,
  version 1.25.0. Its license is GPL-3.0. horus runs it as a separate process
  only. horus never copies or imports its code.
- **comfy-cli backend:** the second implementation of the `Backend` interface
  of `internal/backend`. The first one is the Windows portable backend.
- **The venv of `horus install-comfy-mcp`:** the Python virtual environment
  `<home>/.venvs/comfy-mcp`. It holds the programs `comfy-mcp` and `comfy`.
- **MPS (Metal Performance Shaders):** the GPU backend of PyTorch on Apple
  Silicon.
- **TCC (Transparency, Consent and Control):** the macOS rule that blocks a
  process from `~/Desktop`, `~/Documents` and `~/Downloads` until the user
  grants access. This spec calls these three folders "the TCC folders".
- **State file:** `horus-comfyui.state.json` in the install folder. A backend
  writes it when it starts ComfyUI. `Stop` reads it.
- **The Mac:** the owner's Apple M4 with 16 GB. **The PC:** the owner's
  Windows PC with the NVIDIA GPU.

## Ratified goal (owner, 2026-11-12)

| Row | Outcome | Oracle | Threshold | Ratchet |
|---|---|---|---|---|
| `mac-backend` | On macOS, `horus install-comfyui`, `horus start-comfyui` and `horus stop-comfyui` work through comfy-cli. `stop-comfyui` refuses a ComfyUI that horus did not start. | Go tests on `macos-latest` with a fake `comfy` program that records its arguments. On the Mac: install into a new folder, start, stop. | The fake gets the three commands of #53. On the Mac the server answers after the start and does not answer after the stop. | Commands that answer "not supported" on macOS: 3 → 0 |
| `mac-setup` | On a Mac with `horus`, comfy-cli and the token, `horus setup it-illustration -yes` makes the machine ready. No `HORUS_COMFYUI_DIR` is needed. | On the Mac, with the default folder: the command, then `horus doctor -workflow it-illustration`. | 0 FAIL lines. A second `horus setup it-illustration` prints `Plan: 0 steps`. | Manual steps of #53: 3 → 2 (the token and the PATH stay until the installer of Phase 4) |
| `mac-image` | An agent gets an image from `generate_image` on the Mac, from a ComfyUI that horus started, with the default settings. | The goal tool with `--real` on the Mac, through MCP, cold. The agent opens the PNG. | One 1536x1024 PNG that shows the subject, `started_backend: true`, no timeout. | Images through MCP on macOS: 0 → 1 |
| `tcc-folders` | The default folders on macOS are outside the TCC folders. `horus doctor` warns when a path of the setup is inside one of them, and still exits 0. | Go tests of the default folder and of the warning. On the Mac: `horus doctor` from the clone in `~/Desktop`. | A `WARN` line that names the path. 0 FAIL lines. | Default folders that fit macOS: 0 → 1 |
| `out-of-memory` | A job that fails with the memory error of MPS gives `OUT_OF_MEMORY`. | A Go test of `internal/comfy` and one of `cmd/horus` with the error text of PyTorch on MPS. | `OUT_OF_MEMORY` for the MPS text. | Test cases with the MPS text: 0 → 2 |
| `one-config` | One `.mcp.json` starts both MCP servers on Windows and on macOS: each entry names a command on the PATH. `horus doctor` follows `HORUS_COMFYUI_URL` (#82). | `horus doctor` on the PC and on the Mac. A Go test for #82. | 0 FAIL lines on both. | Windows-only paths in `.mcp.json`: 2 → 0 |
| `one-setup-doc` | One `docs/setup.md` takes a new user on Windows or macOS to the first image. No sentence contradicts the commands (#91). | The Red searches of #91. The acceptance agent follows the page on the Mac. | The searches print nothing. The agent needs no step that the page does not name. | Sentences that the commands contradict: 4 → 0 |
| `same-behaviour` | Windows users and MCP callers get the same answers as before. | CI on `windows-latest` and `macos-latest`: contract test, parity test, `go test ./...` with archtest. On the PC: the Phase 2 rows `backend` and `commands` with `--real`. | `30 of 30`, `17 of 17`, archtest passes with no allow list, both real rows pass, no fewer Go test functions than the baseline (215). | Never below the baseline on a merged PR |

**Done** means: all 8 rows pass on `main` from the one committed tool
`tools/goal_check.py --phase 3` (with `--real` on the Mac and on the PC), and
the agent acceptance task finds no open defect.

After the ratification of 2026-11-12 the owner is not asked to verify or
decide anything. A finding becomes an issue that blocks the task that it
threatens. One exception, chosen by the owner: a pull request that the session
on the Mac opens comes from the account `hoacao33`, and the owner merges it by
hand.

## The design

The design rules of Phase 2 hold
([`2026-11-11-restructure.md`](2026-11-11-restructure.md)): simple, deep
modules, pure functions first, interfaces only at input and output, one
direction. `go test ./internal/archtest` has no allow list.

### The comfy-cli backend

`internal/backend` gets one new file for the comfy-cli backend. `backend.New`
picks by OS: the portable backend on Windows, the comfy-cli backend on macOS,
and the "not supported on this OS yet" backend on every other OS. The `Backend`
interface does not change (decision D37 of #4). Phase 3 removed one method from it: `Layout` is no
method of `Backend` any more, because no caller outside the package used it (#124).

| Method or type | What the comfy-cli backend does |
|---|---|
| `Installed` | `<dir>/ComfyUI/main.py` is a file. The same rule as on Windows. |
| `layout` (a type of the package, no method of the interface) | `ComfyUI` = `<dir>/ComfyUI`. `Python` = `<dir>/ComfyUI/.venv/bin/python`. `Logs` = `<dir>/ComfyUI/user/comfyui_<port>.log`. |
| `Install` | Skips when ComfyUI is installed. Else it runs `comfy --workspace <dir>/ComfyUI install --m-series --version 0.44.0` with a closed standard input, and gives the lines of `comfy` to `out`. |
| `Start` | When the server answers: nothing. When ComfyUI is not installed: an error with the method `NotInstalled`. Else it runs `comfy --workspace <dir>/ComfyUI launch --background -- --listen 127.0.0.1 --port <port>`, writes the state file, and waits until the server answers. |
| `Stop` | It reads the state file. Without a state file for the same address it stops nothing, and it refuses when a server answers there. With one, it runs `comfy --workspace <dir>/ComfyUI stop`, deletes the state file, and waits until the server does not answer. |

- **The pinned ComfyUI version** is one constant for both backends (`v0.44.0`).
- **How the backend finds `comfy`:** the PATH first, then the venv of
  `horus install-comfy-mcp`. When neither has it, the error names both fixes:
  `horus install-comfy-mcp`, or `uv tool install comfy-cli==1.25.0`.
- **Why a state file.** comfy-cli keeps one background ComfyUI for each user,
  not for each workspace, and `comfy stop` stops that one. A person can start
  that ComfyUI with `comfy launch --background`. The standing rule says that
  `Stop` ends only a ComfyUI that horus started. So horus keeps its own record.
- **The stale state file.** The state file holds the process id that comfy-cli
  reports for the start, if comfy-cli gives one (`comfy --json ... launch`
  prints it in version 1.25.0). When that process is gone, `Stop` runs no
  `comfy stop`: it deletes the state file and says that ComfyUI was not running
  any more. The task confirms the output format on the fake and on the Mac. If
  comfy-cli gives no process id, the task records the limit in the evidence
  log.
- **The text of both backends is the same** where the step is the same: the
  lines of `Start` and `Stop` ("ComfyUI already answers at ...", "OK: stopped
  the ComfyUI that horus started ...").
- **Flags.** `-archive-path`, `-seven-zip` and `-keep-archive` of
  `horus install-comfyui` belong to the Windows backend. On macOS the command
  refuses them with exit 2 and a text that names the flag.
- **Linux** keeps "not supported on this OS yet".

### The default install folder

| OS | Default of `HORUS_COMFYUI_DIR` |
|---|---|
| Windows | `<home>/ComfyUI_windows_portable` (unchanged) |
| macOS | `<home>/horus`, so ComfyUI is in `<home>/horus/ComfyUI` (owner, 2026-11-12) |
| other | `<home>/horus` |

The rule is a pure function of `internal/platform`. `internal/settings` calls
it. No help text or message names `ComfyUI_windows_portable` on macOS.

### `horus doctor`

- A result has three levels: PASS, WARN and FAIL. A WARN line does not change
  the exit code, and the last line stays `All checks passed.` when no check
  failed.
- **The TCC check (macOS only).** For each of these paths, the doctor prints a
  `WARN` line when the path is inside a TCC folder: the repo root, the install
  folder, `HORUS_MODELS_DIR`, `HORUS_TEXT_ENCODER_DIR` and the output folder.
  The line names the path and the fix (move it, or grant access in System
  Settings). The test for "inside" is a pure function of `internal/platform`.
  On another OS the check prints nothing.
- **The server address (#82).** Without `-server-url`, the doctor checks
  `HORUS_COMFYUI_URL`, then `http://127.0.0.1:8188`.
- **comfy-cli and comfy-mcp.** The checks follow the new `.mcp.json` (below):
  `comfy-mcp` and `comfy` are in the venv of `horus install-comfy-mcp` or on
  the PATH.

### One `.mcp.json`

```json
{
  "mcpServers": {
    "comfy-mcp": { "type": "stdio", "command": "horus", "args": ["comfy-mcp"],
                   "env": { "COMFY_LOCAL_URL": "${COMFY_LOCAL_URL:-http://127.0.0.1:8188}" } },
    "horus":     { "type": "stdio", "command": "horus", "args": ["mcp"] }
  }
}
```

`horus comfy-mcp` is a new command. It finds the program `comfy-mcp` (the venv
of `horus install-comfy-mcp` first, then the PATH) and runs it with the
standard input, output and error of `horus`. It sets `COMFY_BIN` to the `comfy`
next to that program when the variable is not set, and `PYTHONUTF8=1` and
`PYTHONIOENCODING=utf-8`. Its exit code is the exit code of `comfy-mcp`. When
`comfy-mcp` is not found, it writes the fix (`horus install-comfy-mcp`) to the
standard error and exits 1.

Why this and not `"command": "comfy-mcp"`: a venv is not on the PATH, and
`uv` is not on the PC. With `horus comfy-mcp`, the only program that the PATH
needs is `horus`, on both systems, and the PC needs no change.
`horus install-comfy-mcp` keeps its venv on both systems (`python3.12` on
macOS).

### Out of memory, and the time limit

- The MPS text of PyTorch is `MPS backend out of memory (MPS allocated: ...)`.
  `internal/comfy/jobs.go` already maps a text with "out of memory" to
  `OUT_OF_MEMORY`. The test run of #53 had no such error, so the text is from
  PyTorch, not from the Mac. This phase adds the test cases. It changes code
  only if a test fails.
- `HORUS_GENERATE_TIMEOUT` keeps its default of 1800 s. The cold run on the
  Mac took 1033 s.

## Where each task runs

| Kind of task | Machine | Pull request from | Merge |
|---|---|---|---|
| Build tasks (code, docs, the goal tool) | any; the oracle of the macOS code is CI on `macos-latest` | `hieplam` | the agent, when CI is green |
| The real run and the verify on the PC | the PC | `hieplam` | the agent |
| The real run, the verify and the acceptance on the Mac | the Mac | `hoacao33` | **the owner, by hand** |

A session on the wrong machine that gets such a task ends without a claim.

## Scope

Out of scope:

- Linux and Intel Macs.
- Releases with release-please, `install.sh`, `install.ps1`, the PATH setup,
  embedded workflows, `horus uninstall`, setup through MCP (Phase 4).
- The HTTP transport `horus serve` (Phase 5).
- Model files for each platform: the int8 files run on MPS (#53).

A finding outside the 8 rows becomes a follow-up issue outside the epic.

## Standing rules

The standing rules of Phase 2 hold. In short:

- A change of behaviour on Windows needs a goal row. `tests/mcp_contract.py`,
  `tests/workflow_parity.py` and every golden file do not change.
- A design gap: decide inside the design rules, record the decision in the
  evidence log, and go on.
- A plan command that no longer fits the tree: use the equivalent command that
  proves the same goal row, and record why. Never weaken what a row proves.
- A flaky test: fix the root cause. Never weaken or delete a test to make
  Green pass.
- Every check must fail on an empty implementation. Probe a new check of the
  goal tool with a stub before you trust it.
- `Backend.Stop` ends only a ComfyUI that horus started.
- **On the PC:** the owner's ComfyUI on port 8188 gets no job and is never
  stopped. The 3 model files in the folder that `extra_model_paths.yaml` maps
  are never deleted, moved or downloaded. A real image run uses a second
  ComfyUI on port 8189 from a scratch folder of junctions (evidence log of
  Phase 2, T7), removed one link at a time. A command that can change files
  runs without `-yes`. `go install` while `horus mcp` runs: rename the old
  file, do not stop the process.

New in this phase:

- **On the Mac:** `~/horus-lab` (the install and the model files of #53) is
  never changed. The real run uses the default folder `~/horus`, which stays
  after the epic as the install of the Mac. The owner agreed to one new
  download of the model files (11 GB) into it.
- A task with "Runs on: the Mac" or "Runs on: the PC" is done only by a
  session on that machine.
- A session on the Mac merges no pull request. It posts the link, and the
  owner merges.
- horus runs `comfy` and `comfy-mcp` as separate processes only.
- Decisions go to the "Decisions" section of the evidence log
  [`../plans/2026-11-12-macos-backend-evidence.md`](../plans/2026-11-12-macos-backend-evidence.md).

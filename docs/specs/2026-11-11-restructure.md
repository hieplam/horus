# Spec: restructure horus into three layers (Phase 2)

Date: 2026-11-11. Tracking issue of the whole goal:
[#4](https://github.com/hieplam/horus/issues/4). Epic of this phase:
[#52](https://github.com/hieplam/horus/issues/52). Plan:
[`../plans/2026-11-11-restructure.md`](../plans/2026-11-11-restructure.md).

## What this phase delivers

The Go code moves from the Phase 1 layout (one package for each old Python
module and each old script) to three layers with one direction of calls. The
behaviour that an agent or a command-line user sees stays the same. Code that
is specific to one operating system lives in two packages only, and a test
enforces that.

Terms:

- **Handler:** code that receives a request from outside. horus has two kinds:
  the commands (`internal/cli`) and the MCP server (`internal/mcpserver`).
- **Service:** code that does one job of horus, for example "make one image".
  It does not know MCP, flags or output text.
- **Adapter:** code that talks to one external thing: ComfyUI, Hugging Face,
  the model files on the disk, the ComfyUI install.
- **Wiring:** the code in `cmd/horus` that creates the adapters and gives them
  to the services.
- **Backend:** the way ComfyUI is installed, started and stopped on one
  platform. Phase 2 has one backend: the Windows portable build.
- **Job Object:** a Windows kernel object that holds a group of processes.
  Windows can end every process of the group in one call.
- **archtest:** the Go test package `internal/archtest`. It reads the source
  tree and fails when a rule of this spec is broken.
- **Allow list:** a committed list of the files that break an archtest rule
  today. A file that is not on the list fails the test at once. The list can
  only become shorter, and the phase is done when it is empty.

## Design rules (owner, 2026-11-11)

1. **Simple.** horus is a small to medium program. Do not add a design pattern
   that hides no complexity.
2. **Deep modules** (John Ousterhout, "A Philosophy of Software Design"). A
   package hides its complexity behind a small interface. A caller must not
   need to know how the package does its work.
3. **Pure functions first.** Code that does no input and no output is a plain
   function. It gets no interface.
4. **Interfaces where horus does input or output.** A service gets ComfyUI, the
   downloader, the model files and the backend as interfaces. The service
   declares the interface that it needs. A test gives the service a fake.
5. **One direction.** A request comes from outside into a handler. The handler
   calls a service. The service calls an adapter through an interface. No call
   goes the other way.

## The layers

```
outside:    a person at a terminal            an AI agent
                    |                               |
handlers:   internal/cli                  internal/mcpserver
                    \                               /
services:   generate    provision    doctor    devtools
                    |  (through interfaces that the service declares)
adapters:   comfy    download    modelfiles    backend
base:       catalog    graph    herr    settings    platform
wiring:     cmd/horus creates the adapters and gives them to the services
```

| Package | Layer | What it holds | Comes from |
|---|---|---|---|
| `cmd/horus` | wiring | `main`, the command table, the creation of adapters and services | `cmd/horus` |
| `internal/cli` | handler | One file for each command: flags in, text out | the `Run` functions of `internal/scripts/*` |
| `internal/mcpserver` | handler | The 5 tools, the envelope, the progress notifications | `internal/server` |
| `internal/generate` | service | One image job from start to end | `internal/generate` |
| `internal/provision` | service | The setup of a machine: backend, model files of a workflow, comfy-mcp, the MCP registration | `internal/scripts/workflow`, `internal/models`, `installcomfymcp`, `register` |
| `internal/doctor` | service | The checks of `horus doctor` | `internal/scripts/checksetup` |
| `internal/devtools` | service | The tools for a workflow author: new workflow, sync workflows | `newworkflow`, `syncworkflows` |
| `internal/comfy` | adapter | The ComfyUI REST client | `internal/comfy` |
| `internal/download` | adapter | Download with resume, size and SHA-256 check. The only code that reads `HF_TOKEN` | `internal/scripts/downloadmodel` |
| `internal/modelfiles` | adapter | Find, check and remove model files, `extra_model_paths.yaml` | `internal/scripts/workflow` (file part) |
| `internal/backend` | adapter | `Install`, `Start`, `Stop`, `Layout` of a ComfyUI install. One file for each backend | `installcomfyui`, `startcomfyui`, the autostart of `internal/comfy` |
| `internal/catalog` | base | `manifest.json`, `models.json` | `internal/catalog` |
| `internal/graph` | base | The conversion of a UI graph to an API graph. Pure | `internal/comfy/graph.go`, `pyjson.go` |
| `internal/herr` | base | The 21 error codes | `internal/herr` |
| `internal/settings` | base | Environment and `.env` | `internal/settings` |
| `internal/platform` | base | Processes (start, detach, end a process tree), free disk space, default folders for each OS | the `_windows.go` and `_unix.go` files of 5 packages |
| `internal/archtest` | test | The rules below | new |

`internal/comfy/comfytest` (the fake ComfyUI for Go tests) stays.

**Import rules that archtest enforces (row `direction`):**

- `internal/cli` and `internal/mcpserver` import services and base packages
  only.
- A service imports base packages only. It imports no adapter and no other
  service, and it imports neither `net/http` nor `os/exec`.
- An adapter imports base packages only.
- A base package imports base packages only.
- `cmd/horus` can import every package.
- A test file follows the rules of its package, but it can import
  `internal/comfy/comfytest`.

## Ratified goal (owner, 2026-11-11)

| Row | Outcome | Oracle | Threshold | Ratchet |
|---|---|---|---|---|
| `layout` | Each concern is in the package that the table above names. `internal/server`, `internal/models` and `internal/scripts` are gone. | `go list ./...` against the committed list `tools/phase2_packages.txt`. | 0 packages that differ. | 12 → 0 packages off target. |
| `same-behaviour` | An agent and a command-line user get the same answers as before. | `tests/mcp_contract.py` and `tests/workflow_parity.py` in CI on `windows-latest` and `macos-latest`, and `go test ./...`. The two test files and all golden files do not change. | `contract: 30 of 30 passed`, `parity: 17 of 17 cases equal`, 0 changed golden files, and no fewer Go test functions than the baseline. | Never below 30, 17 and the baseline count on a merged PR. |
| `os-boundary` | OS-specific code is only in `internal/platform` and `internal/backend`. | `go test ./internal/archtest`. It fails for `runtime.GOOS`, a `//go:build` line with an OS name, an OS-named file, and an import of `syscall` or `golang.org/x/sys`. A planted bad file proves that it flags each case. | 0 files on the allow list. | 19 → 0 files. |
| `direction` | Calls go one way: handler, then service, then adapter. A service gets its input and output as interfaces. | The import rules above, in `go test ./internal/archtest`, with planted bad files. | 0 imports on the allow list. | The baseline count → 0. |
| `process-tree` | `Backend.Stop` ends ComfyUI and every process that ComfyUI started. A ComfyUI that horus started keeps running when `horus mcp` ends. | A Go test in `internal/platform` starts a detached child that starts a grandchild. Check 1: both live after the starter ends. Check 2: a second process ends the tree. It runs on `windows-latest` (Job Object) and `macos-latest` (process group). | Check 1: 2 of 2 processes alive. Check 2: 0 alive after 5 s. No `taskkill` in the code. | `taskkill` references: baseline → 0. |
| `backend` | horus installs, starts and stops ComfyUI only through the `Backend` interface. On Windows the portable build still starts on demand. On macOS the answer is still "not supported on this OS yet". | Go tests with a fake backend. On the owner's PC: stop the ComfyUI that horus started, then `tools/goal_check.py --phase 2 --real`. | horus starts ComfyUI through the backend and makes one 1536x1024 PNG that shows the subject. | 0 → 1 image after a start through the backend. |
| `token` | Only the download code reads the Hugging Face token. | An archtest rule: the text `HF_TOKEN` is in Go code only in `internal/download`, and in the refusal of `internal/settings`. A planted bad file proves the rule. | 0 files on the allow list. | The baseline count → 0. |
| `commands` | `horus doctor` and `horus setup` exist. `horus setup <workflow>` prints its plan, and acts only with `-yes`. The old command names still work. | Go tests for each. On the owner's PC: `horus doctor`, and `horus setup it-illustration` without `-yes`. | `horus doctor` prints 0 FAIL lines. The plan of `horus setup` says that nothing is missing. | 0 → 2 commands. |

**Done** means: all 8 rows pass on `main` from the one committed tool
`tools/goal_check.py --phase 2 --real`, and the agent acceptance task finds no
open defect.

After the ratification of 2026-11-11 the owner is not asked to verify or
decide anything. A finding becomes an issue that blocks the task that it
threatens.

## Decisions of this phase

| Topic | Decision |
|---|---|
| Process boundary (D12 and D16 of #4) | Changed. `horus mcp` calls the `provision` service in its own process. It starts no `horus workflow` child. A cancel ends a download through the Go context, and the `.part` file stays. |
| Token (D7 of #4) | Kept, with a new proof: only `internal/download` reads `HF_TOKEN`, and archtest enforces it. The token is in the memory of `horus mcp` while a download runs. |
| `horus workflow ... -json` | Kept. The command still prints the `HORUS_JSON` line, because `tests/workflow_parity.py` and people use it. Only the server stops reading it. |
| Command names | `horus doctor` and `horus setup` are new. Every Phase 1 command name keeps working until Phase 3 rewrites `docs/setup.md`. |
| `horus setup` | It runs the steps of `provision` in order: backend, then the model files of the workflow. Without `-yes` it prints the plan and changes nothing. |
| Job Object | `internal/platform` starts ComfyUI in a named Job Object without the "end on close" flag. So ComfyUI keeps running after its starter ends, and a later `Stop` opens the job by name and ends the tree. |
| `Backend.Stop` | It ends only a ComfyUI that horus started. It never ends a ComfyUI that a person started. |
| `internal/herr`, `internal/settings` | The names stay, because every package imports them. |
| Follow-ups #40 and #41 | Not part of this phase. PR #49 fixed them and merged before the first move. |
| Mac test run (#53) | It runs in parallel on the owner's Mac. The backend task reads its result when the result exists. |

## Scope

Out of scope:

- The macOS backend, model files for each platform (Phase 3).
- Releases with release-please, `install.sh`, `install.ps1`, embedded
  workflows, `horus uninstall`, setup through MCP (Phase 4).
- The HTTP transport `horus serve` (Phase 5).
- The install of the model files from nothing on a fresh machine. The owner
  tests this.

## Standing rules

- **Moved code keeps its behaviour.** A change of behaviour needs a goal row.
  A difference that this spec names (the server calls `provision` in its own
  process) is allowed. Record each other difference in the evidence log.
- **The test files of the contract do not change:** `tests/mcp_contract.py`,
  `tests/workflow_parity.py` and every golden file.
- **Never touch the model files of the owner's PC.** A test that installs or
  removes files uses a temporary folder. A command on the owner's PC that can
  change files runs without `-yes`.
- **A design gap:** decide inside the design rules above, record the decision
  in the evidence log, and go on.
- A plan command that no longer fits the tree: use the equivalent command that
  proves the same goal row, and record why. Never weaken what a row proves.
- A flaky test: fix the root cause. No retries, no longer sleeps, no looser
  assertions.
- Never weaken or delete a test to make Green pass. A test moves with its code.
- Another heavy job on the PC (for example an image run of another session):
  wait until the GPU is free, run again one time, record both runs.
- An allow list of archtest only becomes shorter. No task adds an entry.
- Decisions go to the "Decisions" section of the evidence log.

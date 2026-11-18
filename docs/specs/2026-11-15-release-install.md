# Spec: release, install line, full setup and uninstall (Phase 4)

Date: 2026-11-15. Tracking issue of the whole goal:
[#4](https://github.com/hieplam/horus/issues/4). Epic of this phase:
[#134](https://github.com/hieplam/horus/issues/134).

> **Record of a finished phase.** Since 2026-11-18 the plan file and the
> evidence log of this phase are not in the tree
> ([ADR 0020](../adr/0020-one-place-for-each-record-of-feature-work.md)). The
> tasks are the issues of the epic, the proof of each task is in its pull
> request, and the decisions are in [`../adr/`](../adr/README.md). Where this
> page says "the evidence log", read `git show 763b1bd:docs/plans/2026-11-15-release-install-evidence.md`. The plan
> was `docs/plans/2026-11-15-release-install.md` at the same commit. The goal tool of this
> phase, `tools/goal_check.py`, is not in the tree: read `git show 80609ae:tools/goal_check.py`
> ([ADR 0021](../adr/0021-goal-suites-are-go-test-packages.md)).

## What this phase delivers

On a new Windows machine or a new Mac with Apple Silicon, one line installs the
`horus` program from a GitHub Release. The program works with no clone of this
repo: it holds the default workflows. `horus setup it-illustration -yes` then
does every step that a program can do, and an agent can do the same setup
through the tool `install_workflow` with the user's consent.
`horus uninstall` removes what horus installed. `horus register` adds the
server to Claude Code or to Codex.

Terms:

- **release-please:** the release tool of Google. It reads the commits on
  `main`, keeps one pull request open with the next version and the changelog,
  and makes a GitHub Release when that pull request is merged.
- **Release asset:** a file of a GitHub Release. This phase has three:
  `horus-windows-amd64.exe`, `horus-darwin-arm64` and `checksums.txt` (one
  SHA-256 line for each program file).
- **Install line:** the one command of the README that downloads and runs
  `install.sh` (macOS) or `install.ps1` (Windows).
- **User folder:** the folder of horus for one user: `HORUS_HOME`, default
  `<home>/horus` on both systems. It holds `.env`, `outputs/` and the copy of
  the built-in workflows. On macOS it also holds `ComfyUI/` (decision D43 of
  #4).
- **Built-in workflows:** the folders of `workflows/` that the build puts into
  the program (`go:embed`).
- **Package step:** a step of a setup plan that installs a system package with
  the package manager of the OS: 7-Zip with `winget` on Windows, Python 3.12
  with Homebrew (`brew`) on macOS.
- **Install record:** the file `horus-install.json` that horus writes into a
  folder that horus made (the ComfyUI install folder, the venv of
  `horus install-comfy-mcp`). `horus uninstall` deletes such a folder only
  when the record is there.
- **The PC:** the owner's Windows PC with the NVIDIA GPU. **The Mac:** the
  owner's Apple M4.
- **Fresh-machine run:** a run of the workflow `fresh-machine.yml` on the
  GitHub runners `windows-latest` and `macos-latest`. A runner is a new
  machine for each run.

## Ratified goal (owner, 2026-11-15)

| Row | Outcome | Oracle | Threshold | Ratchet |
|---|---|---|---|---|
| `release` | A GitHub Release `v0.x` holds the program for Windows and for macOS, and `checksums.txt`. `horus version` prints the version of the release. | A Go test of `horus version`. The CI job `release-build` builds the three assets on each pull request. After the release: the goal tool downloads the assets of the newest release and checks each SHA-256. | 3 assets. Each SHA-256 is equal to its line in `checksums.txt`. `horus version` of the Windows asset prints the version of the tag. | Releases with a program: 0 → 1 |
| `install-line` | One line downloads the program of the platform, checks its SHA-256, and puts `horus` on the PATH. A wrong SHA-256 installs nothing. | The CI jobs `install windows-latest` and `install macos-latest` run the scripts against assets that the job built. The fresh-machine run uses the real release. | A new shell finds `horus`, and `horus version` exits 0. With a wrong line in `checksums.txt`, the script exits with a code that is not 0 and writes no program file. | Commands a person runs to get `horus` on the PATH: 2 or more (install Go, `go install`, edit the PATH) → 1 |
| `binary-only` | A machine with only the program, and no clone, lists and describes the built-in workflows. `.env` and the images are in the user folder. `HORUS_WORKFLOWS_DIR` and a clone still replace the built-in workflows. | Go tests of `cmd/horus` with a temporary home folder and a current folder outside a clone. The install jobs run `horus workflow list` outside the checkout. | The output names `it-illustration`. No `No workflows folder` error. The `.env` of the temporary user folder is read. | Answers `BACKEND_NOT_INSTALLED` ("No workflows folder") outside a clone: 1 → 0 |
| `full-setup` | `horus setup <workflow>` prints every step that the machine needs: the package steps, comfy-cli, ComfyUI, the model files. With `-yes` it runs them. It runs a package step only with `-with-packages`. `install_workflow` runs the same plan after the user's consent. | Go tests with fake `winget`, `brew`, `comfy` and a fake download, on a sandbox. The contract test for the consent of `install_workflow`. The fresh-machine run installs ComfyUI for real on both runners. | The sandbox goes from nothing to `Plan: 0 steps`. Without `-with-packages` no package manager runs. Without `accept_setup` the tool changes nothing and answers `SETUP_CONFIRMATION_REQUIRED`. | Setup steps of `docs/setup.md` that a person does by hand before the first image: measured by T0 → only the token and the license |
| `uninstall` | `horus uninstall` prints its plan and changes nothing. With `-yes` it removes the model files, each folder with an install record, the MCP registration, the PATH entry of the installer and the program. `-models` and `-mcp` limit it to one part. It keeps what horus did not install. | Go tests on a sandbox home folder. A test of `cmd/horus`: setup with the fakes, then uninstall. The fresh-machine run: install line, then `horus uninstall -yes`. | After `-yes`: no file of the plan is left, a folder without an install record is still there, and a new shell does not find `horus`. Without `-yes`: no file changes. | Commands that undo a horus install: 0 → 1 |
| `register` | `horus register -client claude` and `horus register -client codex` add the server `horus` to that client. A second run changes nothing. | Go tests with fake `claude` and `codex` programs that record their arguments. On the PC: the real `codex` with `CODEX_HOME` in a scratch folder. | The fake gets the `mcp add` arguments of the spec. `codex mcp get horus` shows the entry in the scratch folder. | Clients that `horus register` serves: 1 → 2 |
| `readme` | `README.md` shows the install line for both systems and the manual setup steps in its first or second section. `docs/setup.md` starts with the install line. | The goal tool reads the first two sections of the README. The commands of the fresh-machine run are the commands of the README, word for word. | Both install lines are in section 1 or 2. Each README command is in `fresh-machine.yml`. | Position of the install steps in the README: section 3 → section 1 or 2 |
| `same-behaviour` | Callers of the stdio server and users of the old commands get the same answers as before, except the consent of `install_workflow`. | CI on `windows-latest` and `macos-latest`: the contract test, the parity test, `go test ./...` with archtest. On the PC: one real image through MCP from the released program. | Each of the 30 contract checks of the baseline still passes, `17 of 17` parity cases, archtest with no allow list, no fewer Go test functions than the baseline (250), one 1536x1024 PNG. | Never below the baseline on a merged pull request |

**Done** means: all 8 rows pass on `main` from the one committed tool
`python tools/goal_check.py --phase 4 --real` on the PC after the release, the
fresh-machine run on the real release is green on both runners, and the agent
acceptance task finds no open defect.

After the ratification of 2026-11-15 the owner is not asked to verify or
decide anything, with one exception that the owner chose: **the owner
publishes the release.** Task T10 waits for the owner, who turns on one
repository setting and merges the pull request of release-please.

Owner decisions of 2026-11-15, each in its ADR:

- Phase 5 (the HTTP transport `horus serve`) does not start now:
  [ADR 0003](../adr/0003-stdio-transport-http-later.md).
- `install_workflow` runs the whole plan, and horus installs 7-Zip and
  Python 3.12 itself only after a person approves that step:
  [ADR 0011](../adr/0011-setup-through-mcp-needs-two-consents.md).
- The owner publishes the release:
  [ADR 0014](../adr/0014-who-merges-and-who-publishes.md).
- The real run on the Mac is a follow-up outside this epic, and the oracle of
  macOS is the runner `macos-latest`:
  [ADR 0018](../adr/0018-real-runs-on-the-owners-machines.md).

## The design

The design rules of Phase 2 hold
([`2026-11-11-restructure.md`](2026-11-11-restructure.md)).
`go test ./internal/archtest` has no allow list.

### Version and release

- One variable holds the version. The release build sets it with the linker
  flag `-X`. A build without the flag has the version `dev`.
- `horus version` prints one line: `horus <version>`. The MCP server gives the
  same version in its `serverInfo`.
- `release-please-config.json` and `.release-please-manifest.json` are in the
  repo root. The config keeps every version at `0.x` (`bump-minor-pre-major`
  and `bump-patch-for-minor-pre-major`, decision D36). The first release is
  `0.1.0`.
- `.github/workflows/release.yml` runs on each push to `main`. Job 1 runs
  release-please. Job 2 runs only when job 1 made a release: it builds the two
  programs with `CGO_ENABLED=0`, `-trimpath` and the version flag, writes
  `checksums.txt`, and uploads the three assets to the release.
- The same build script runs in the CI job `release-build` on each pull
  request, without the upload. So a broken release build fails a pull request,
  not a release.
- The workflow gets only the permissions that it needs: `contents: write` and
  `pull-requests: write` for the release-please job.

### The install scripts

| | `install.sh` (macOS) | `install.ps1` (Windows) |
|---|---|---|
| Install line | `curl -fsSL https://raw.githubusercontent.com/hieplam/horus/main/install.sh \| sh` | `irm https://raw.githubusercontent.com/hieplam/horus/main/install.ps1 \| iex` |
| Asset | `horus-darwin-arm64` | `horus-windows-amd64.exe` |
| Program file | `<home>/.local/bin/horus` | `%LOCALAPPDATA%\Programs\horus\horus.exe` |
| PATH | One marked line in the shell profile of the user, when the folder is not on the PATH | The folder joins the PATH of the user (registry `HKCU\Environment`) |

Both scripts:

1. Refuse another OS or another CPU with a text that names what is supported.
2. Download the asset and `checksums.txt` from
   `https://github.com/hieplam/horus/releases/latest/download/`.
   `HORUS_VERSION` picks a release. `HORUS_INSTALL_BASE_URL` replaces the
   address (the tests use it). `HORUS_INSTALL_DIR` replaces the folder.
   `HORUS_INSTALL_NO_PATH=1` leaves the PATH unchanged.
3. Calculate the SHA-256 of the download. When it is not equal to the line of
   `checksums.txt`, delete the download, print both values, and exit with a
   code that is not 0.
4. Move the file to its place, and print the next commands:
   `horus setup it-illustration` and `horus register`.

A second run replaces the program (an update). The scripts send no token and
read no secret.

### A machine with only the program

- **The repo root** is `HORUS_ROOT`, then the nearest folder at or above the
  current folder that is a horus clone. A folder is a horus clone when it has
  `workflows/_template/README.md`. A folder that only has a `workflows` folder
  is not a clone. When there is no clone, there is no repo root.
- **The workflows folder** is `HORUS_WORKFLOWS_DIR`, then `<repo root>/workflows`,
  then the copy of the built-in workflows.
- **The copy of the built-in workflows** is `<user folder>/builtin-workflows`.
  The program writes it when the folder is absent or when its stamp file names
  another version. It writes to a temporary folder first and renames it, so two
  programs that start at the same time do not break the copy. The code of
  `internal/catalog` and `internal/provision` still reads a folder path.
- **`.env`:** `<repo root>/.env` when there is a repo root, then
  `<user folder>/.env`. An environment variable still has priority over both.
- **The output folder:** `HORUS_OUTPUT_DIR`, then `<repo root>/.scratch/outputs`
  when there is a repo root, then `<user folder>/outputs`.
- **Texts.** An error text or a `next_step` that names "the `.env` file of the
  horus repo" names the `.env` path that the program reads. The
  `BACKEND_NOT_INSTALLED` answer for a missing workflows folder stays only for
  a `HORUS_WORKFLOWS_DIR` that does not exist.
- The user folder is the pure function `platform.UserDir`. `internal/settings`
  calls it.

### The setup plan

`horus setup <workflow>` prints the facts and then the steps, in this order:

| Step | When | What `-yes` runs |
|---|---|---|
| Package: 7-Zip (Windows) | ComfyUI is not installed and no `7z.exe` is found | `winget install --id 7zip.7zip -e --accept-source-agreements --accept-package-agreements` |
| Package: Python 3.12 (macOS) | comfy-cli is not found and no Python 3.12 is found | `brew install python@3.12` |
| comfy-cli (macOS) | ComfyUI is not installed and `comfy` is not found | the work of `horus install-comfy-mcp` |
| ComfyUI | ComfyUI is not installed | `Backend.Install` |
| Model file | the file is not present with its size | the download, as today |

- **Approval of a package step.** `horus setup <workflow> -yes` with a package
  step in the plan changes nothing and exits 1. Its text names the package,
  the command, and the two ways to go on: run again with `-with-packages`, or
  install the package by hand. `-yes -with-packages` runs the whole plan.
- **No package manager.** When `winget` or `brew` is not found, the step is a
  step for a person. The plan prints what to install and where to get it.
  `-yes` then changes nothing and exits 1.
- **After a package step** horus looks for the program again in the folder
  where the package manager puts it (`C:\Program Files\7-Zip`, the `bin`
  folder of Homebrew), because the PATH of the running process is old.
- The plan stays a pure function of the facts. The service reads the facts
  through its interfaces. The package managers run through
  `provision.Programs`.
- horus never removes a system package.

### Setup through MCP

`install_workflow` runs the plan of `horus setup`.

- New argument `accept_setup` (boolean, default false).
- New error code `SETUP_CONFIRMATION_REQUIRED` (`action_by: human`, not
  retryable). The tool answers it when the plan has a step other than a model
  file and `accept_setup` is false. `details.steps` lists each step: its kind,
  what it installs, the command for a package step, and the folder. The tool
  changes nothing before this answer.
- With `accept_setup=true` the tool runs those steps, also the package steps.
  One consent covers them, because `details.steps` shows each of them to the
  user.
- A step for a person (no package manager) gives `BACKEND_NOT_INSTALLED` with
  a `next_step` that names the package.
- `LICENSE_CONFIRMATION_REQUIRED` and `accept_license` keep their behaviour.
  The order is: the setup consent first, then the license consent.
- A machine where ComfyUI is installed gets the same answers as before.
- The table of error codes has 22 codes. `docs/mcp.md` lists the new code and
  the new argument. The contract test gets checks for the new answers. Each of
  the 30 checks of the baseline stays.

### `horus register`

- `-client claude` (default) keeps the behaviour of today.
- `-client codex` runs `codex mcp get horus`. When that exits 0, it prints the
  SKIP line. Else it runs `codex mcp add horus -- <path of horus> mcp`.
- Both clients get `HORUS_ROOT=<repo root>` only when there is a repo root
  (`-e` for `claude`, `--env` for `codex`). Without a repo root,
  `horus register` works and passes no `HORUS_ROOT`.
- Another client name exits 2 with a text that lists the two names.

### `horus uninstall`

`horus uninstall` prints the plan: one line for each item, with `REMOVE` or
`KEEP`, the path, and the reason of a `KEEP`. Without `-yes` it changes
nothing.

| Part | What `-yes` removes | What it keeps |
|---|---|---|
| Model files | The model files of each workflow, by the rules of `horus workflow remove` | A file that differs from `models.json` |
| ComfyUI | The install folder, when it has an install record. horus stops a ComfyUI that horus started first. | A folder without an install record. A folder whose ComfyUI answers and that horus did not start. |
| comfy-mcp venv | The venv, when it has an install record | A venv without an install record |
| MCP registration | The server `horus` of Claude Code and of Codex, when the client is found and knows the server | |
| User folder | `builtin-workflows/` and the state files of horus | `.env` and `outputs/`: they are data of the user. The plan names both paths. |
| Program | The PATH entry of the installer, and the program file | A system package (7-Zip, Python) |

- `-models` limits the command to the model files. `-mcp` limits it to the MCP
  registration. Without them it does all parts.
- The install record is written by `Backend.Install` and by
  `horus install-comfy-mcp` when they make the folder. An install of an older
  horus has no record, so `horus uninstall` keeps it and prints how to remove
  it by hand.
- **The program removes itself last.** On Windows a running program cannot
  delete its own file, so the Windows code of `internal/platform` ends the
  removal after the process exits. The test waits up to 10 s for the file to
  go.
- The PATH entry is removed by OS code in `internal/platform`: the registry
  value on Windows, the marked profile line on macOS.
- The exit code is 0 when each `REMOVE` item is gone, and 1 when one is left.

### Docs

`README.md` gets the install line and the manual steps in section 1 or 2.
`docs/setup.md` starts with the install line. The clone and `go install` stay
as the path for a developer of this repo. `AGENTS.md`, `docs/mcp.md` and
`ROADMAP.md` follow the new commands.

## Where each task runs

| Kind of task | Machine | Pull request from | Merge |
|---|---|---|---|
| Build tasks | the PC. The oracle of macOS code is CI on `macos-latest`. | `hieplam` | the agent, with squash, when CI is green |
| T10, the release | GitHub | `github-actions` (release-please) | **the owner** |
| The verify and the acceptance | the PC | `hieplam` | the agent |

## Scope

Out of scope:

- Phase 5: `horus serve`, MCP tasks, decision D4.
- A real run on the Mac. It is a follow-up issue outside the epic.
- Linux and Intel Macs. The scripts refuse them.
- A Homebrew tap, a Scoop bucket, code signing and notarisation.
- An update command. A second run of the install line updates the program.

A finding outside the 8 rows becomes a follow-up issue outside the epic.

## Standing rules

The standing rules of Phase 3 hold
([`2026-11-12-macos-backend.md`](2026-11-12-macos-backend.md), "Standing
rules"). In short: a design gap is decided inside the design rules and
recorded. A plan command that no longer fits the tree is replaced by one that
proves the same row. A flaky test gets a root-cause fix. No test is weakened
or deleted. Each check must fail on an empty implementation.

New in this phase:

- **The uninstall rule (risk R8 of #4).** `horus uninstall -yes` and
  `horus workflow remove -yes` never run on the PC with the real home folder.
  Each test and each manual run of them sets the home folder (`USERPROFILE`
  and `HOME`), `HORUS_HOME` and `HORUS_ROOT` to a scratch folder, and unsets
  `HORUS_MODELS_DIR`, `HORUS_TEXT_ENCODER_DIR` and `HORUS_COMFYUI_DIR`. A test
  first checks that each path of the plan is inside the scratch folder, and
  only then runs `-yes`.
- **On the PC** no package step runs for real (`winget` changes the machine of
  the owner). The install line runs only with `HORUS_INSTALL_DIR` in a scratch
  folder and `HORUS_INSTALL_NO_PATH=1`. A real `register` uses `CODEX_HOME` in
  a scratch folder. The rules of Phase 3 for the PC hold: port 8188 gets no
  job, the model files are never deleted, moved or downloaded, and the real
  image uses port 8189.
- **The contract test** gets new checks in the tasks that the plan names. No
  other task changes `tests/mcp_contract.py`, `tests/workflow_parity.py` or a
  golden file. No existing check is removed or made weaker.
- **A fork.** No agent approves, merges or checks out a pull request from a
  fork. The pull request of release-please (author `github-actions`) is merged
  by the owner in this phase.
- **Secrets.** No workflow of this phase reads `HF_TOKEN`. No script prints an
  environment variable.
- Decisions go to the "Decisions" section of the evidence log.

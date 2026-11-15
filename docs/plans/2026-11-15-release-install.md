# Plan: release, install line, full setup and uninstall (Phase 4)

Spec: [`../specs/2026-11-15-release-install.md`](../specs/2026-11-15-release-install.md).
Epic: [#134](https://github.com/hieplam/horus/issues/134). Evidence log:
[`2026-11-15-release-install-evidence.md`](2026-11-15-release-install-evidence.md).

## How to read this plan

- This file wins over the copy of a section in an issue.
- Read the spec first: the design, the table "Where each task runs" and the
  standing rules apply to every task. **Read the uninstall rule of the spec
  before you run a command that deletes files.**
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
  with all checks passed.
- Every task runs on the PC. The oracle of macOS code is the CI job
  `go darwin/arm64` on `macos-latest`: read its result on the pull request
  before the merge.
- A pull request of the account `hieplam` is merged by the agent with squash
  when CI is green. Task T10 is the only task of the owner.
- The commands below use `python` for the Python of the test tools (3.12 with
  `pip install -r tests/requirements.txt`) and `horus` for a fresh build
  (`go install ./cmd/horus`). While `horus mcp` runs on the PC, rename the old
  program file before `go install`. Do not stop the process.
- "The goal tool" is `python tools/goal_check.py --phase 4`. It prints one
  line for each of the 8 rows and then `goal: N of 8 rows passed`.
- "A fake program" is a small program that a Go test writes into a temporary
  folder under the name of the real one (`winget`, `brew`, `claude`, `codex`,
  `comfy`). It appends its arguments to a file and plays the part that the
  test needs.
- "The sandbox" is a temporary folder that a test uses as the home folder, the
  user folder and the repo root (the uninstall rule of the spec).
- The contract test has 30 checks at the baseline. T5 adds checks. A Green
  line "all checks passed" means `contract: N of N passed` with the N of that
  time.

## T0 - Baseline and goal tool

Files: `tools/goal_check.py`, `tools/test_goal_check.py`, the evidence log.

Steps:

1. Add `--phase 4` to `tools/goal_check.py`, with the 8 rows of the spec. A
   row whose code does not exist yet fails with a detail that names what is
   missing. A row never raises.
2. What each row checks without `--real`:
   - `release`: the pinned Go test of `horus version`. The files
     `release-please-config.json`, `.release-please-manifest.json` and
     `.github/workflows/release.yml` exist, and the config holds the two
     `0.x` settings. The CI job `release-build` on HEAD.
   - `install-line`: `install.sh` and `install.ps1` exist at the repo root. The
     CI jobs `install windows-latest` and `install macos-latest` on HEAD.
   - `binary-only`: the pinned Go tests of `cmd/horus` and `internal/settings`
     (the list without a clone, the `.env` of the user folder, the output
     folder, the override by `HORUS_WORKFLOWS_DIR`, the rule for a clone).
   - `full-setup`: the pinned Go tests of the plan (the package steps, the
     refusal without `-with-packages`, the step for a person), the pinned test
     of `cmd/horus` that goes from nothing to `Plan: 0 steps`, and the pinned
     tests of `install_workflow` (`SETUP_CONFIRMATION_REQUIRED`,
     `accept_setup`). `horus errors -json` lists 22 codes.
   - `uninstall`: the pinned Go tests (the plan changes nothing, `-yes`
     removes, the install record, `-models`, `-mcp`, the PATH entry, the
     program file) and the pinned test of `cmd/horus` (setup, then uninstall).
   - `register`: the pinned Go tests with the fake `claude` and the fake
     `codex`.
   - `readme`: both install lines are in the first or the second `## ` section
     of `README.md`. Each command line of the README section is in
     `.github/workflows/fresh-machine.yml`.
   - `same-behaviour`: as the Phase 3 row, with the baseline of this phase:
     the commit of this task, 250 Go test functions, and the names of the 30
     contract checks of the baseline, each of which must still pass.
3. What `--real` adds (on the PC, after the release):
   - `release`: download the three assets of the newest release into a scratch
     folder, check each SHA-256 against `checksums.txt`, and run
     `horus version` of the Windows asset: it prints the version of the tag.
   - `install-line`, `full-setup`, `uninstall`: the newest run of
     `fresh-machine.yml` with the real release as its source is green on both
     runners, and its release tag is the newest tag.
   - `register`: `horus register -client codex` with `CODEX_HOME` in a scratch
     folder, then `codex mcp get horus` exits 0.
   - `same-behaviour`: one real image through MCP from the downloaded Windows
     asset, by the method of the Phase 2 row `backend` (port 8189).
4. A pinned test name must be in the output of `go test -v -run` with
   `--- PASS`, and its body must assert something. A package with no test file
   fails the row.
5. Add the section "Baseline" to the evidence log: the output of the goal
   tool, the number of Go test functions, the names of the 30 contract checks,
   and the number of steps that a person does by hand in `docs/setup.md`
   before the first image (the ratchet of `full-setup`).

Verify:

- **Goal:** every row of the spec has one committed command that measures it
  (rows: all).
- **Red:** `python tools/goal_check.py --phase 4` exits 2 with
  `argument --phase: invalid choice: 4`.
- **Green:** `python tools/goal_check.py --phase 4 --no-ci` prints 8 row lines
  and `goal: 1 of 8 rows passed` (only `same-behaviour` passes), and exits 1.
  `python -m pytest tools/test_goal_check.py` passes.
- **Stub check:** the tool is the check. A tool that prints PASS for a row
  whose code does not exist fails the Green count "1 of 8". The tests of
  `tools/test_goal_check.py` give each row a stub (an empty package, a missing
  test name, a README without the install line) and need FAIL.
- **Ratchet:** goal rows of Phase 4 with a committed check 0 → 8.

## T1 - Version and release build

Files: `cmd/horus/`, `internal/mcpserver/server.go`, `internal/cli/`,
`release-please-config.json`, `.release-please-manifest.json`,
`.github/workflows/release.yml`, `.github/workflows/ci.yml`,
`scripts/build-release.sh`.

Steps:

1. One version variable with the default `dev` (spec, "Version and release").
   `horus version` prints `horus <version>`. The MCP server reports the same
   version. Keep one source of the version: remove the constant
   `serverVersion`.
2. `scripts/build-release.sh <version> <out folder>` builds
   `horus-windows-amd64.exe` and `horus-darwin-arm64` with `CGO_ENABLED=0`,
   `-trimpath` and the version flag, and writes `checksums.txt` in the format
   of `sha256sum`.
3. `release.yml`: the release-please job, then the upload job that runs
   `scripts/build-release.sh` only when a release was made. Use the smallest
   permissions (spec).
4. The config files of release-please: release type `go`, the two `0.x`
   settings, and a first release of `0.1.0`. Read the docs of
   release-please for the exact keys. Do not trust memory.
5. CI job `release-build` on `ubuntu-latest`: run the script with the version
   `0.0.0-ci`, then check that the folder has exactly 3 files and that
   `sha256sum -c checksums.txt` passes. Upload the folder as a workflow
   artifact with the name `release-assets`, for the install jobs of T3.
6. `README.md` of the repo is not changed here (T8).

Verify:

- **Goal:** row `release` (the part before the real release).
- **Red:** `go run ./cmd/horus version` exits 2 with
  `horus: unknown command "version"`.
- **Green:** `go run ./cmd/horus version` prints `horus dev`.
  `go run -ldflags "<the version flag>=1.2.3" ./cmd/horus version` prints
  `horus 1.2.3`. The Go test of the MCP server version passes. The CI job
  `release-build` of the pull request is green. The contract test: all checks
  passed.
- **Stub check:** a `version` command that prints a fixed text fails the
  `1.2.3` case. A build script that writes an empty `checksums.txt` fails
  `sha256sum -c` and the count of 3 files.
- **Ratchet:** `python tools/goal_check.py --phase 4 --no-ci` rows passed
  1 → 1 (the row `release` passes with CI only), and the pinned test of
  `horus version` goes from missing to PASS in its detail.

## T2 - A machine with only the program

Files: `workflows/` (one Go file for the embed), `internal/settings/`,
`internal/platform/`, `internal/catalog/`, `internal/herr/`,
`internal/mcpserver/root.go`, `internal/archtest/layers.go`,
`cmd/horus/wiring.go`.

Steps:

1. Embed the built-in workflows (spec, "A machine with only the program").
   Embed every file that a tool or a command reads. Leave out `_template` and
   the example images when no tool returns their path. Record the choice.
2. `platform.UserDir`: a pure function for the user folder.
3. The new rule for the repo root, the workflows folder, `.env` and the output
   folder. The copy of the built-in workflows is written through a temporary
   folder and a rename. Callers of `settings.WorkflowsDir()` do not change.
4. The texts that name "the `.env` file of the horus repo" name the path that
   the program reads.
5. `internal/archtest/layers.go` gets the new package in the layer `base`.
   Add no allow list.
6. Tests, each with a temporary home folder and a current folder outside a
   clone:
   - `horus workflow list` names `it-illustration`;
   - `list_workflows` of the MCP server gives it too;
   - a `.env` in the user folder gives a setting;
   - the default output folder is `<user folder>/outputs`;
   - `HORUS_WORKFLOWS_DIR` replaces the built-in workflows;
   - a folder with only a `workflows` folder is not a clone;
   - a stamp file of another version makes the program write the copy again;
   - two programs that write the copy at the same time both list the workflow.

Verify:

- **Goal:** row `binary-only`.
- **Red:** from a temporary folder, with `HORUS_ROOT` not set:
  `horus workflow list` prints `No workflows folder` and exits 1.
- **Green:** the same command prints a line with `it-illustration` and exits
  0. `go test ./internal/settings ./internal/platform ./internal/catalog ./internal/archtest ./cmd/horus`
  prints `ok` for each. `python tools/goal_check.py --phase 4 --no-ci --only binary-only`
  prints `PASS  binary-only`. The contract test: all checks passed. The parity
  test prints `parity: 17 of 17 cases equal`.
- **Stub check:** without the embed, the list test finds no workflow and
  fails. A root rule that still accepts each `workflows` folder fails the
  clone test.
- **Ratchet:** goal rows passed 1 → 2.

## T3 - The install scripts

Files: `install.sh`, `install.ps1`, `.github/workflows/ci.yml`,
`scripts/lint-public.py` only when it must know the new files.

Steps:

1. Write both scripts (spec, "The install scripts"). `install.sh` is POSIX
   `sh`. `install.ps1` runs on Windows PowerShell 5.1.
2. CI jobs `install windows-latest` and `install macos-latest`. Each job:
   - downloads the artifact `release-assets` of the job `release-build`;
   - serves that folder with `python -m http.server` on `127.0.0.1`;
   - runs the script with `HORUS_INSTALL_BASE_URL` set to that server, by the
     same pipe as the install line (`cat install.sh | sh`,
     `Get-Content -Raw install.ps1 | iex`);
   - starts a new shell and runs `horus version` and, from a folder outside
     the checkout, `horus workflow list`;
   - changes one character of `checksums.txt`, runs the script again with
     another `HORUS_INSTALL_DIR`, and needs an exit code that is not 0 and an
     empty folder;
   - runs the script a second time on the first folder and needs exit 0 (the
     update).
3. The scripts print no environment variable.

Verify:

- **Goal:** row `install-line` (the part before the real release), and the
  outside-the-checkout part of `binary-only`.
- **Red:** the two install jobs do not exist: `gh run view --json jobs` of the
  newest CI run on `main` names no job that starts with `install`.
- **Green:** both install jobs of the pull request are green, and their logs
  show `horus 0.0.0-ci`, a line with `it-illustration`, and the refusal of the
  wrong SHA-256. `python tools/goal_check.py --phase 4 --only install-line`
  prints `PASS  install-line` for the part without `--real`.
- **Stub check:** a script that skips the SHA-256 check installs the file in
  the wrong-checksum step, and the job fails on the folder that is not empty.
  A script that does not change the PATH fails the new-shell step.
- **Ratchet:** goal rows passed 2 → 3.

## T4 - The whole setup plan

Files: `internal/provision/`, `internal/cli/setup.go`, `internal/platform/`,
`internal/backend/` (find `7z.exe` and Python again after a package step),
`cmd/horus/wiring.go`.

Steps:

1. The plan gets the new steps and their order (spec, "The setup plan"). The
   function that makes the steps from the facts stays pure.
2. The facts come through the interfaces that the service has. Add a method
   only when no existing one gives the fact.
3. `-with-packages`, the refusal without it, and the step for a person when
   the package manager is not found.
4. After a package step, look for the program in the folder of the package
   manager.
5. Tests with the fake programs on the sandbox, for Windows and for macOS
   (pick the OS by the test seam `goos`, as the tests of Phase 3 do):
   - the plan of an empty machine lists the steps in the order of the spec;
   - `-yes` without `-with-packages` runs no program and exits 1;
   - `-yes -with-packages` gives the fake `winget` (or `brew`) exactly the
     arguments of the spec, and the next `horus setup` prints `Plan: 0 steps`;
   - without `winget` (or `brew`) the plan prints the step for a person, and
     `-yes -with-packages` runs nothing and exits 1;
   - a machine with ComfyUI gets the plan of today (no new step).
6. `docs/setup.md` is not changed here (T8).

Verify:

- **Goal:** row `full-setup` (the command).
- **Red:** `go test ./internal/provision ./cmd/horus -run 'Package|WithPackages' -v`
  prints `testing: warning: no tests to run`.
- **Green:** `go test ./internal/provision ./internal/cli ./internal/backend ./cmd/horus`
  prints `ok` for each. The job `go darwin/arm64` of the pull request is green.
  The contract test: all checks passed. The parity test prints
  `parity: 17 of 17 cases equal`.
- **Stub check:** a plan without the package step fails the order test. A
  `-yes` that runs a package step without `-with-packages` leaves a line in
  the argument file of the fake `winget`, and the refusal test fails.
- **Ratchet:** pinned tests of `full-setup` that pass, in the detail of the
  goal tool: 0 → the tests of the command.

## T5 - Setup through MCP

Files: `internal/mcpserver/`, `internal/herr/`, `tests/mcp_contract.py`,
`docs/mcp.md`, `tools/goal_check.py` (the new count of contract checks).

Steps:

1. `install_workflow` runs the plan (spec, "Setup through MCP"): the argument
   `accept_setup`, the code `SETUP_CONFIRMATION_REQUIRED`, `details.steps`,
   and the order of the two consents.
2. Progress: while a step other than a download runs, the progress message
   names the step.
3. The contract test gets checks for: the new answer on a machine without
   ComfyUI, no change on disk before the consent, the install after
   `accept_setup=true` (with the fake ComfyUI and a fake backend install that
   the test sandbox allows), and the same answers as before on a machine with
   ComfyUI. Each of the 30 baseline checks stays, with its name.
4. `docs/mcp.md`: the new argument, the new code (22 codes), and the flow.
   `horus errors -json` lists the new code.
5. Update the count of contract checks in the CI job `contract`, in
   `AGENTS.md` only when it names the count, and in the goal tool.

Verify:

- **Goal:** rows `full-setup` (the MCP part) and `same-behaviour`.
- **Red:** `horus errors -json` has no `SETUP_CONFIRMATION_REQUIRED`
  (`horus errors -json | python -c "import json,sys; print(len(json.load(sys.stdin)))"`
  prints `21`).
- **Green:** the same command prints `22`. `go test ./internal/mcpserver ./internal/herr`
  prints `ok`. The contract test prints `contract: N of N passed` with N above
  30, and the names of the 30 baseline checks are all in its output with a
  pass. `python tools/goal_check.py --phase 4 --no-ci --only full-setup same-behaviour`
  prints `PASS` for both.
- **Stub check:** a tool that installs without the consent fails the check "no
  change on disk before the consent". A tool that ignores `accept_setup=true`
  fails the install check.
- **Ratchet:** goal rows passed 3 → 4. Contract checks 30 → N.

## T6 - horus register for Claude Code and Codex

Files: `internal/provision/register.go`, `internal/cli/mcptools.go`,
`internal/doctor/` only when a check names the registration.

Steps:

1. `-client claude|codex` (spec, "`horus register`"). `HORUS_ROOT` goes to the
   client only when there is a repo root.
2. Tests with the fake `claude` and the fake `codex`: the exact arguments, the
   SKIP line on a second run, no `HORUS_ROOT` without a repo root, exit 2 for
   another client name.
3. A real run on the PC with `CODEX_HOME` in a scratch folder:
   `horus register -client codex`, then `codex mcp get horus`. Put the output
   in the evidence log. Do not run the real `claude mcp add` outside a scratch
   config.

Verify:

- **Goal:** row `register`.
- **Red:** `horus register -client codex` exits 2 with
  `flag provided but not defined: -client`.
- **Green:** `go test ./internal/provision ./internal/cli -run Register -v`
  shows the new tests with `--- PASS`. With `CODEX_HOME` in a scratch folder,
  `codex mcp get horus` exits 0 and prints the path of `horus`.
  `python tools/goal_check.py --phase 4 --no-ci --only register` prints
  `PASS  register`.
- **Stub check:** a `register` that ignores `-client` gives the fake `codex`
  no arguments, and the test fails.
- **Ratchet:** goal rows passed 4 → 5.

## T7 - horus uninstall

Files: `internal/provision/` (the uninstall plan and its run),
`internal/cli/` (the command), `internal/backend/` and
`internal/provision/comfymcp.go` (the install record), `internal/platform/`
(the PATH entry and the removal of the program file), `cmd/horus/`.

**Read the uninstall rule of the spec first. No test and no manual run uses
the real home folder.**

Steps:

1. The install record: `Backend.Install` and `horus install-comfy-mcp` write
   `horus-install.json` into the folder that they make.
2. The uninstall plan is a pure function of the facts, as the setup plan. The
   command prints it (spec, "`horus uninstall`"): `REMOVE` and `KEEP` lines.
3. `-yes` runs the plan in this order: stop the ComfyUI that horus started,
   the model files, ComfyUI, the venv, the MCP registration of both clients,
   the files of the user folder, the PATH entry, the program file.
4. `-models` and `-mcp`.
5. OS code only in `internal/platform`: the PATH entry of the installer (the
   same registry value and the same marked profile line that the scripts of T3
   write) and the removal of the running program.
6. Tests on the sandbox. Each test checks that every path of the plan is
   inside the sandbox before it runs `-yes`:
   - without `-yes` no file changes (compare the file list before and after);
   - `-yes` removes each `REMOVE` item, and a ComfyUI folder without an install
     record stays;
   - `.env` and `outputs/` stay, and the plan names them;
   - `-models` leaves ComfyUI and the registration, `-mcp` leaves the files;
   - the fake `claude` and the fake `codex` get the `mcp remove` arguments;
   - a ComfyUI that answers and that horus did not start is kept;
   - a copy of the program in the sandbox removes itself (wait up to 10 s);
   - one test of `cmd/horus`: setup with the fakes, then uninstall, then the
     sandbox holds only the kept items.
7. Add `horus uninstall -yes` as the last step of the two install jobs of T3,
   and check there that a new shell does not find `horus`.

Verify:

- **Goal:** row `uninstall`.
- **Red:** `horus uninstall` exits 2 with `horus: unknown command "uninstall"`.
- **Green:** `go test ./internal/provision ./internal/platform ./internal/backend ./internal/cli ./cmd/horus`
  prints `ok` for each. Both install jobs of the pull request are green with
  the uninstall step. `python tools/goal_check.py --phase 4 --only uninstall`
  prints `PASS  uninstall` for the part without `--real`. The contract test:
  all checks passed.
- **Stub check:** an `uninstall -yes` that does nothing leaves the files, and
  the removal tests fail. An uninstall that ignores the install record deletes
  the folder without a record, and the keep test fails. An uninstall that acts
  without `-yes` fails the file-list test.
- **Ratchet:** goal rows passed 5 → 6.

## T8 - Docs

Files: `README.md`, `docs/setup.md`, `docs/mcp.md`, `AGENTS.md`, `ROADMAP.md`.

Steps:

1. `README.md`: the install line for both systems and the manual steps in
   section 1 or 2 (decision D34 of #4). The manual steps are: download the
   asset, check its SHA-256, put it on the PATH.
2. `docs/setup.md`: step 0 is the install line. Then `horus setup <workflow>`
   as the one command, with `-with-packages` explained. The steps for a
   developer of this repo (the clone, `go install`) move to their own section.
   Add `horus uninstall` and `horus register -client`.
3. `AGENTS.md`: the rows for the new commands, and the layout lines.
   `ROADMAP.md`: the release and the install line move to "Now". Phase 5 stays
   under "Later", with the reason.
4. Every command of the docs exists: run each one with `-h`.

Verify:

- **Goal:** row `readme` (the README part).
- **Red:** `python tools/goal_check.py --phase 4 --no-ci --only readme` prints
  `FAIL  readme` and names the missing install lines.
- **Green:** the detail of the same command names only the missing
  `fresh-machine.yml` (T9). `python scripts/lint-public.py` exits 0. Each
  `horus <command> -h` that the docs name exits 0.
- **Stub check:** a README with the install line in section 5 fails the row.
- **Ratchet:** position of the install steps in the README: section 3 →
  section 1 or 2.

## T9 - The fresh-machine run

Files: `.github/workflows/fresh-machine.yml`, `tools/goal_check.py` only when
the check of the README commands needs a fix.

Steps:

1. A workflow with `workflow_dispatch` (input `source`: `build` or `release`)
   that also runs when a release is published (then the source is `release`).
   Jobs on `windows-latest` and `macos-latest`.
2. With `source: build` the job builds the assets with
   `scripts/build-release.sh` and serves them (as T3). With `source: release`
   it runs the install line of the README word for word.
3. Then, in a new shell and outside the checkout, the commands of the README
   and of `docs/setup.md`, word for word:
   - `horus version`, `horus workflow list`, `horus doctor` (it can report
     FAIL lines: the log keeps them);
   - `horus setup it-illustration` (the plan);
   - the real install of ComfyUI: on Windows `horus install-comfyui`, on macOS
     `horus install-comfy-mcp` and `horus install-comfyui`. No model file is
     downloaded. No workflow reads `HF_TOKEN`;
   - `horus setup it-illustration` again: the plan has only model file steps;
   - `horus uninstall -yes`: then the ComfyUI folder is gone, and a new shell
     does not find `horus`.
4. Run it with `source: build` on the branch of the pull request
   (`gh workflow run` needs the file on the default branch: when that blocks
   the run, merge first with the two jobs marked `continue-on-error: false`
   and run it on `main`, then fix what it finds in a second pull request).
5. A failure that is a product defect becomes an issue that blocks this task.

Verify:

- **Goal:** rows `install-line`, `full-setup`, `uninstall` and `readme`, on a
  new machine.
- **Red:** `gh workflow list` names no `fresh-machine`.
- **Green:** one run of `fresh-machine.yml` with `source: build` is green on
  both runners. Its log shows the plan with only model file steps after the
  install, and the checks after `horus uninstall -yes`.
  `python tools/goal_check.py --phase 4 --no-ci --only readme` prints
  `PASS  readme`.
- **Stub check:** a workflow that skips the install of ComfyUI fails the check
  "the plan has only model file steps". An `uninstall` that keeps the ComfyUI
  folder fails the last check.
- **Ratchet:** goal rows passed 6 → 7 (`release` and the `--real` parts wait
  for T10).

## T10 - The owner publishes release 0.1.0

This task belongs to the owner. No agent does it.

Steps for the owner:

1. Turn on the repository setting: Settings → Actions → General → Workflow
   permissions → "Allow GitHub Actions to create and approve pull requests".
2. Open the pull request of release-please (title `chore(main): release 0.1.0`,
   author `github-actions`). Check the version and the changelog. Merge it with
   squash.
3. Wait for the workflow `release` to end. Then close this issue.

When the pull request of release-please does not exist after step 1, run the
workflow `release` again (Actions → release → Re-run).

Verify:

- **Goal:** row `release`.
- **Red:** `gh release list` prints nothing.
- **Green:** `gh release view v0.1.0 --json assets --jq '.assets[].name'`
  prints `checksums.txt`, `horus-darwin-arm64` and `horus-windows-amd64.exe`.
- **Stub check:** a release without the upload job has 0 assets, and the Green
  command prints nothing.
- **Ratchet:** releases with a program 0 → 1.

## T11 - Verify on main, after the release

Files: the evidence log. Code only through a fix issue.

Steps:

1. Check that the run of `fresh-machine.yml` that the release started (source
   `release`) is green on both runners. When the release did not start it, run
   it with `gh workflow run fresh-machine.yml -f source=release`.
2. On the PC: `python tools/goal_check.py --phase 4 --real`. Follow the rules
   of the spec for the PC: the install into a scratch folder with
   `HORUS_INSTALL_NO_PATH=1`, port 8189, no change of the model files.
3. Open the PNG of the real image and check that it shows the subject.
4. Run `node "<epic-loop skill>/scripts/epic.mjs" check <n> --dry-run` for
   every closed task of the epic.
5. Each row that fails becomes a fix issue that blocks this task.

Verify:

- **Goal:** all 8 rows, on `main`, on the real release.
- **Red:** not applicable: this task builds nothing. Before T10 the command of
  Green fails on the row `release` ("no release").
- **Green:** `python tools/goal_check.py --phase 4 --real` prints
  `goal: 8 of 8 rows passed` and exits 0. The evidence log has the output, the
  link of the green fresh-machine run, and the path of the PNG.
- **Stub check:** the goal tool fails each row on an empty implementation
  (T0). A release without assets fails `release`. A red fresh-machine run fails
  three rows.
- **Ratchet:** goal rows passed 7 → 8.

## T12 - Agent acceptance

Files: the evidence log, issue #4, `ROADMAP.md` when a line is stale.

Steps:

1. Act as a new user who reads only `README.md` and `docs/setup.md`. Read the
   log of the green fresh-machine run next to the two pages, line by line.
   Each command of the pages must be in the log, and each result that the
   pages promise must be in the output.
2. On the PC, in a scratch home folder (the uninstall rule of the spec): the
   install line with `HORUS_INSTALL_DIR` and `HORUS_INSTALL_NO_PATH=1`, then
   `horus version`, `horus workflow list`, `horus setup it-illustration`,
   `horus uninstall` (the plan only), and through an MCP client
   `list_workflows`, `describe_workflow` and `install_workflow` without
   consent. Judge each text as a user: does it name the item, what it does,
   and the next command?
3. Read each new error text and each `next_step` against the rule "Explaining
   to the user" of `AGENTS.md`.
4. Check the design: `go test ./internal/archtest` passes with no allow list,
   and each new interface hides an input or an output.
5. Each defect becomes an issue with its evidence, the goal row and a Verify
   block, and it blocks this task. A finding outside the 8 rows becomes a
   follow-up issue outside the epic.
6. Update issue #4: tick the boxes of Phase 4, add the decisions of this phase
   to the decision log, write the status and the next action, and record that
   the owner moved Phase 5 to later. Open the follow-up issue for the real run
   on the Mac.

Verify:

- **Goal:** the acceptance bar of the spec: no open defect against the 8 rows.
- **Red:** not applicable: this task builds nothing.
- **Green:** the evidence log has the section `## T12` with the list of what
  was checked and the result of each item. `gh issue list --search "<epic> defect"`
  shows no open defect issue under the epic.
  `python tools/goal_check.py --phase 4 --real` still prints
  `goal: 8 of 8 rows passed`.
- **Stub check:** an acceptance that reads no log has no line-by-line list in
  the evidence log, and the coordinator rejects the section.
- **Ratchet:** open defects against the goal rows: the number found → 0.

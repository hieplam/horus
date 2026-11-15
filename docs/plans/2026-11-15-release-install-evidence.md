# Evidence log: release, install line, full setup and uninstall (Phase 4)

Spec: [`../specs/2026-11-15-release-install.md`](../specs/2026-11-15-release-install.md).
Plan: [`2026-11-15-release-install.md`](2026-11-15-release-install.md).
Epic: [#134](https://github.com/hieplam/horus/issues/134).

Each task adds one section `## <task id>` with the literal output of its Green
commands.

## Decisions

| Date | Decision | Who | Why |
|---|---|---|---|
| 2026-11-15 | Phase 5 (`horus serve`) does not start now. | the owner | The Go MCP SDK v1.8.0 has no MCP tasks. |
| 2026-11-15 | `install_workflow` runs the whole setup plan. No sixth tool. | the owner | A caller learns no new tool. |
| 2026-11-15 | horus installs 7-Zip (`winget`) and Python 3.12 (`brew`) itself, only after a person approves the step. | the owner | A new machine has neither program. |
| 2026-11-15 | The owner turns on the Actions setting and merges the pull request of release-please (T10). | the owner | A release is public. |
| 2026-11-15 | The oracle of macOS is the runner `macos-latest`. The real run on the Mac is a follow-up outside the epic. | the planning session | This PC cannot reach the Mac, and the epic must end without a second machine. |
| 2026-11-15 | The user folder is `<home>/horus` on both systems (`HORUS_HOME`). | the planning session | One folder for a user to find and to remove. On macOS it is the install folder of decision D43. |
| 2026-11-15 | The built-in workflows are copied to `<user folder>/builtin-workflows`, and the code still reads a folder path. | the planning session | Design rule 1: no change of `internal/catalog` and `internal/provision` for a second way to read files. |
| 2026-11-15 | `horus uninstall` deletes a folder only when it has an install record, and it keeps `.env` and `outputs/`. | the planning session | Risk R8 of #4: an uninstall must never delete what horus did not install, or data of the user. |
| 2026-11-15 | A separate consent `accept_setup` with the code `SETUP_CONFIRMATION_REQUIRED` (22 codes). | the planning session | A caller that already sends `accept_license=true` must not approve a system package without a new question to the user. |
| 2026-11-15 | The release assets are plain program files and `checksums.txt`. No archive and no GoReleaser. | the planning session | Design rule 1: two `go build` commands need no tool. |
| 2026-11-15 | The `same-behaviour` row of Phase 4 compares only `tests/workflow_parity.py`, `tests/fixtures` and the golden files with the baseline commit. For `tests/mcp_contract.py` it needs each of the 30 baseline check names (`tools/phase4_contract_checks.txt`) to pass. | the T0 agent (standing rule: a plan command that no longer fits) | T5 must add checks to `tests/mcp_contract.py`. A diff of that file would fail the row. The name list proves that no old check goes away, and it is stronger than a count. |
| 2026-11-15 | The workflow `fresh-machine.yml` (T9) sets `run-name: fresh-machine source=<source> tag=<tag>`. The tool reads the newest run whose title has `source=release`, and it needs the newest tag in that title. | the T0 agent (design gap) | The tool must know the source and the tag of a run. `gh run list` gives the title without more calls. |
| 2026-11-15 | A real image of Phase 4 runs only when `HORUS_COMFYUI_URL` names port 8189. The tool refuses before it starts the program. | the T0 agent (standing rule of the owner's machine) | The ComfyUI of the owner on port 8188 gets no job. |

## Baseline

Measured on 2026-11-15 on the PC, on `main` at commit `38c1838`, before the goal tool had phase 4.
The program was built into the worktree (`go build -o bin/ ./cmd/horus`).

**Red.** The tool has no phase 4:

```
$ python tools/goal_check.py --phase 4
goal_check.py: error: argument --phase: invalid choice: '4' (choose from 1, 2, 3)
exit 2
```

**Numbers.**

| Number | Value | How measured |
|---|---|---|
| Go test functions | 250 | `go test -list . ./...`, count of lines that start with `Test` |
| Contract checks | 30 of 30 pass | `python tests/mcp_contract.py --server-cmd bin/horus.exe mcp` |
| Parity cases | 17 of 17 equal | `python tests/workflow_parity.py` |
| Error codes | 21 | `horus errors -json`, length of the list (T5 raises it to 22) |
| Goal rows of Phase 4 with a committed check | 0 | the tool has no phase 4 |
| Numbered steps that a person does by hand in `docs/setup.md` before the first image (the ratchet of `full-setup`) | 6 (step 0 to step 5) | count of the `## Step N` headings before `## Step 6: make your first image` |
| Programs that a person installs by hand before step 0 | 6 on Windows (Git, Go, Python 3.12, 7-Zip, Visual C++ Redistributable, a Hugging Face account), 4 on macOS (Git, Homebrew, `brew install python@3.12 go uv`, a Hugging Face account) | the Prerequisites tables of `docs/setup.md` |
| Commands that get `horus` on the PATH | 2 or more (`go install ./cmd/horus`, then an edit of the PATH on Windows or in `~/.zshrc`) | step 0 of `docs/setup.md` |
| Position of the install steps in `README.md` | section 3 (`## Install the MCP servers on a new machine`) | `grep -n '^## ' README.md` |

**The 30 contract checks of the baseline.** The file `tools/phase4_contract_checks.txt` holds the same names, one
for each line. The tool of phase 4 needs each name to pass.

1. server identifies as horus and has instructions
2. tools/list has exactly the 5 tools
3. every tool has an outputSchema with ok, result, error
4. uninstall_workflow is marked destructive, list_workflows read-only
5. list_workflows shows installed and partially_installed styles
6. describe_workflow gives subject rules, examples and 3 present files
7. list_workflows and describe_workflow carry no theme or look
8. describe_workflow hides the ComfyUI graph (no node class names)
9. unknown workflow -> WORKFLOW_NOT_FOUND with the known names, isError
10. generate_image writes a PNG at out_path and returns its path and seed
11. horus sends the subject to the model as written (0 characters added)
12. seed 7 reaches the sampler of the graph
13. generate_image sends progress notifications
14. existing out_path -> OUTPUT_EXISTS
15. width 1000 -> INVALID_ARGUMENT field=width, action_by=caller
16. too short subject -> INVALID_ARGUMENT field=subject with the rules
17. style without model files -> WORKFLOW_NOT_INSTALLED naming the file
18. GPU out of memory -> OUT_OF_MEMORY, retryable, action_by=caller
19. a job over the time limit -> GENERATION_TIMEOUT, and horus cancels it on the backend
20. install of an installed style -> ok, already_installed
21. install without accept_license -> LICENSE_CONFIRMATION_REQUIRED, action_by=human, with size
22. uninstall without confirm -> CONFIRMATION_REQUIRED, plan: 1 to delete, 2 shared kept; nothing deleted
23. uninstall with confirm deletes the 1 unshared file and keeps the 2 shared files
24. uninstall with remove_shared deletes the 2 shared files
25. after uninstall, list_workflows shows both styles not_installed
26. errors are the same envelope in structuredContent and in text
27. backend down and autostart off -> BACKEND_UNAVAILABLE, action_by=human
28. it-illustration: manifest has no theme and no caption style file
29. it-illustration: the graph holds no style words
30. docs/mcp.md lists every error code

**The goal tool on the baseline** is the output in `## T0` below, because phase 4 did not exist before T0.

## T0

Task: [#136](https://github.com/hieplam/horus/issues/136). Files: `tools/goal_check.py`, `tools/test_goal_check.py`,
`tools/phase4_contract_checks.txt`, this log.

**What the goal tool of phase 4 does.** `python tools/goal_check.py --phase 4` prints one line for each of the
8 rows of the spec and then `goal: N of 8 rows passed`. A row never raises. A row whose code does not exist
fails, and its detail names what is missing (a file, a test name with the task that writes it, a CI job, a
count of error codes). The Go rows use the pinned tests below: each must report `--- PASS` in `go test -v -run`,
and its body must assert something. A package with no test file fails. Without `--real` a row stands on its
part without `--real`, and the detail says `real part not run`.

**Green (without `--real`, without CI).** Output of
`python tools/goal_check.py --phase 4 --no-ci --server-cmd <worktree>/bin/horus.exe mcp` on commit `38c1838`
plus the T0 change. Exit code 1:

```
FAIL  release - files: missing: release-please-config.json, missing: .release-please-manifest.json, missing: .github/workflows/release.yml; no passing test: TestVersionPrintsTheVersionOfTheLinkerFlag (not in cmd/horus, T1); TestServerInfoVersionIsTheVersionOfTheBuild (not in internal/mcpserver, T1); CI part skipped (--no-ci)
FAIL  install-line - files: missing: install.sh, missing: install.ps1; CI part skipped (--no-ci)
FAIL  binary-only - no passing test: TestWorkflowListWithoutAClonePrintsTheBuiltInWorkflows (not in cmd/horus, T2); ... (6 pins named)
FAIL  full-setup - no passing test: TestPlanOfAnEmptyMachineListsTheStepsInTheOrderOfTheSpec (not in internal/provision, T4); ... (7 pins named); horus errors -json lists 21 codes, needs 22
FAIL  uninstall - no passing test: TestUninstallPlanWithoutYesChangesNothing (not in internal/provision, T7); ... (12 pins named)
FAIL  register - no passing test: TestRegisterClaudeRunsMCPAddWithTheArgumentsOfTheSpec (not in internal/provision, T6); ... (6 pins named)
FAIL  readme - 3 problems: the install line of install.sh is not in section 1 or 2; the install line of install.ps1 is not in section 1 or 2; missing: .github/workflows/fresh-machine.yml
PASS  same-behaviour - contract: 30 of 30, 30 of 30 baseline checks pass; parity: 17 of 17; go test ./...: ok; archtest: ok, 0 allow list files; 250 Go test functions (baseline 250); 0 changed parity or golden files; CI part skipped (--no-ci); real part not run (needs --real on the PC after the release)
goal: 1 of 8 rows passed
note: the real part did not run for same-behaviour (run with --real on the PC after the release)
```

The log shortens the lists of pins with `...`. The tool prints every name.
`python -m pytest tools/test_goal_check.py` prints `98 passed`.

**Ratchet.** Goal rows of Phase 4 with a committed check: 0 → 8. Rows that pass (`--no-ci`): 0 → 1.

**Stub check.** `tools/test_goal_check.py` gives each row a stub that must fail: an empty package and a missing
test name for the Go rows, a test body that asserts nothing, an empty `install.sh`, a release config without
the two `0.x` settings, a README with the install line in section 3, a README command that
`fresh-machine.yml` lacks, a removed baseline contract name (also when the count is higher), a changed
SHA-256 line, a run of `fresh-machine.yml` with `source=build` or for an old tag, and a real image on port
8188. Each of these gives FAIL, and a complete input gives PASS.

**The pinned Go tests.** The later tasks write tests with exactly these names in exactly these packages. A task
renames a pin only by changing `PINNED4` in `tools/goal_check.py` in its own pull request.

| Row | Task | Package | Test name |
|---|---|---|---|
| `release` | T1 | `cmd/horus` | `TestVersionPrintsTheVersionOfTheLinkerFlag` |
| `release` | T1 | `internal/mcpserver` | `TestServerInfoVersionIsTheVersionOfTheBuild` |
| `binary-only` | T2 | `cmd/horus` | `TestWorkflowListWithoutAClonePrintsTheBuiltInWorkflows` |
| `binary-only` | T2 | `cmd/horus` | `TestListWorkflowsOfTheMCPServerWithoutAClonePrintsTheBuiltInWorkflows` |
| `binary-only` | T2 | `internal/settings` | `TestEnvFileOfTheUserFolderGivesASetting` |
| `binary-only` | T2 | `internal/settings` | `TestOutputDirWithoutACloneIsTheOutputsFolderOfTheUserFolder` |
| `binary-only` | T2 | `internal/settings` | `TestHorusWorkflowsDirReplacesTheBuiltInWorkflows` |
| `binary-only` | T2 | `internal/settings` | `TestAFolderWithOnlyAWorkflowsFolderIsNotAClone` |
| `full-setup` | T4 | `internal/provision` | `TestPlanOfAnEmptyMachineListsTheStepsInTheOrderOfTheSpec` |
| `full-setup` | T4 | `internal/provision` | `TestPlanWithoutAPackageManagerHasAStepForAPerson` |
| `full-setup` | T4 | `cmd/horus` | `TestSetupYesWithoutWithPackagesRunsNoProgramAndExitsOne` |
| `full-setup` | T4 | `cmd/horus` | `TestSetupYesWithPackagesRunsTheFakePackageManagerThenAPlanHasZeroSteps` |
| `full-setup` | T4 | `cmd/horus` | `TestSetupYesWithoutAPackageManagerRunsNothingAndExitsOne` |
| `full-setup` | T5 | `internal/mcpserver` | `TestInstallWorkflowWithoutAcceptSetupAnswersSetupConfirmationRequired` |
| `full-setup` | T5 | `internal/mcpserver` | `TestInstallWorkflowWithAcceptSetupRunsTheSetupSteps` |
| `uninstall` | T7 | `internal/provision` | `TestUninstallPlanWithoutYesChangesNothing` |
| `uninstall` | T7 | `internal/provision` | `TestUninstallYesRemovesEachRemoveItem` |
| `uninstall` | T7 | `internal/provision` | `TestUninstallKeepsAFolderWithoutAnInstallRecord` |
| `uninstall` | T7 | `internal/provision` | `TestUninstallKeepsTheEnvFileAndTheOutputsFolder` |
| `uninstall` | T7 | `internal/provision` | `TestUninstallModelsLeavesComfyUIAndTheRegistration` |
| `uninstall` | T7 | `internal/provision` | `TestUninstallMCPLeavesTheFiles` |
| `uninstall` | T7 | `internal/provision` | `TestUninstallKeepsAComfyUIThatAnswersAndHorusDidNotStart` |
| `uninstall` | T7 | `internal/provision` | `TestUninstallRunsMCPRemoveOfBothClients` |
| `uninstall` | T7 | `internal/backend` | `TestInstallWritesTheInstallRecord` |
| `uninstall` | T7 | `internal/platform` | `TestPathEntryOfTheInstallerIsRemoved` |
| `uninstall` | T7 | `internal/platform` | `TestTheProgramFileRemovesItself` |
| `uninstall` | T7 | `cmd/horus` | `TestSetupThenUninstallLeavesOnlyTheKeptItems` |
| `register` | T6 | `internal/provision` | `TestRegisterClaudeRunsMCPAddWithTheArgumentsOfTheSpec` |
| `register` | T6 | `internal/provision` | `TestRegisterClaudeSkipsOnASecondRun` |
| `register` | T6 | `internal/provision` | `TestRegisterCodexRunsMCPAddWithTheArgumentsOfTheSpec` |
| `register` | T6 | `internal/provision` | `TestRegisterCodexSkipsWhenMCPGetExitsZero` |
| `register` | T6 | `internal/provision` | `TestRegisterPassesNoHorusRootWithoutARepoRoot` |
| `register` | T6 | `internal/cli` | `TestRegisterAnotherClientNameExitsTwo` |

Other contracts of the tool that the later tasks must meet:

- **T1:** `release-please-config.json` has `bump-minor-pre-major: true` and `bump-patch-for-minor-pre-major: true`
  (at any depth). The files `.release-please-manifest.json` and `.github/workflows/release.yml` exist. The CI job
  is named `release-build`.
- **T3:** `install.sh` and `install.ps1` are in the repo root, and each names `sha256` and `checksums.txt`. The CI jobs
  are named `install windows-latest` and `install macos-latest`.
- **T5:** `horus errors -json` lists 22 codes. The count of contract checks may grow. Each of the 30 baseline names stays.
- **T8 and T9:** the README holds the two install lines word for word (see `INSTALL_LINES` in the tool) in the first
  or second `## ` section. Each command line in a shell code block (no language, `sh`, `bash`, `powershell`) of those
  sections is in `.github/workflows/fresh-machine.yml`, with white space ignored.
- **T9:** the workflow has `run-name: fresh-machine source=<build|release> tag=<tag>` and a job on each of
  `windows-latest` and `macos-latest` (the job name has the runner name).

**What `--real` runs.** It did not run in T0, because no release exists. The tool refuses a real image unless
`HORUS_COMFYUI_URL` names port 8189. The row `release` downloads the 3 assets of the newest release into
`.scratch/release-assets/<tag>` and checks each SHA-256 and `horus version`. The row `register` uses `CODEX_HOME`
in a scratch folder.

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
| 2026-11-15 | The program embeds each workflow folder in full, also `examples/` and `recipe.json`. It does not embed `_template`. A new workflow folder must be added to the `go:embed` line of `workflows/embed.go`, and `TestEveryWorkflowFolderIsEmbedded` fails when one is missing. | the T2 agent (plan step 1: record the choice) | `describe_workflow` returns the path of each example image, so the images must be on disk. Only `horus new-workflow` reads `_template`, and it works in a clone. |
| 2026-11-15 | The stamp file `.horus-stamp` of the copy holds the SHA-256 of the embedded files, not the program version. | the T2 agent (design gap) | T1 owns the version variable. A build without the linker flag is always `dev`, so a version stamp never writes a changed workflow again. A content stamp changes with each change of a file. |
| 2026-11-15 | `HORUS_ROOT` is the repo root without the clone check. When `HORUS_ROOT` or `HORUS_WORKFLOWS_DIR` names a folder without workflows, `BACKEND_NOT_INSTALLED` stays, and its `next_step` names that setting and the `.env` path. Without both settings, the error comes only when the copy cannot be written, and the `next_step` names `HORUS_HOME`. | the T2 agent (design gap) | The Go tests and the parity test set `HORUS_ROOT` to a sandbox so that no `.env` of a real checkout is read. A clone check on `HORUS_ROOT` would make them read `<home>/horus/.env` of the real home. A setting that the user wrote is not replaced in silence. |
| 2026-11-15 | `HORUS_HOME` comes only from the environment, not from `.env`. | the T2 agent (design gap) | `.env` can be in the user folder, so `.env` cannot move the user folder. |
| 2026-11-15 | `horus workflow`, `horus setup` and `horus doctor -workflow` take `settings.WorkflowsDir()` instead of `<repo root>/workflows`. So `HORUS_WORKFLOWS_DIR` now applies to them too. This changes one line of `internal/cli/setup.go` (lane T4). | the T2 agent (plan step 3) | Without a repo root, `<repo root>/workflows` is the relative folder `workflows`, and `horus workflow list` fails. One rule for the folder is simpler. The parity test passes `-workflows-dir`, so its output does not change. |
| 2026-11-15 | The table of error codes keeps the `next_step` of `HF_TOKEN_INVALID` (the golden file `internal/herr/testdata/errors.golden.json` must not change). The MCP server sets the `next_step` of each `HF_TOKEN_INVALID` answer to a text with the path of `settings.EnvPath()`. `internal/download` reads the token from `settings.EnvPath()`. | the T2 agent (plan step 4, standing rule: no golden file changes) | The text must name the file that the program reads. `internal/download` is not in the file list of T2, but without the change the token of `<user folder>/.env` is never read. |
| 2026-11-15 | Two tests of the old root rule are replaced by tests of the new rule: `TestRootFallsBackToTheExecutableFolder` by `TestRootWithoutACloneIsEmpty`, and `TestRootFindsWorkflowsFolderAboveTheCurrentFolder` by `TestRootFindsTheCloneAboveTheCurrentFolder` (its sandbox is now a clone). | the T2 agent (the spec changes the rule) | The spec removes the folder of the program as a root and the rule "any folder with `workflows/`". The new tests check the same places with the new rule. No test of another behaviour changed. |
| 2026-11-15 | The test "two programs write the copy at the same time" starts the test program 4 times at once as child processes, with one empty user folder. | the T2 agent | Separate processes test the same race as two `horus` programs: the temporary folder and the rename. |
| 2026-11-15 | `tests/mcp_contract.py` and `tests/workflow_parity.py` do not change. | the T2 agent | Both set `HORUS_WORKFLOWS_DIR` to their sandbox (the parity test also sets `HORUS_ROOT`), so the new root rule does not reach them. |
| 2026-11-15 | The package step and the comfy-cli step are in a plan only when ComfyUI is not installed. This also holds for the Python 3.12 step of macOS. | the T4 agent (design gap) | The spec says that a machine with ComfyUI gets the plan of today. A Python step without a comfy-cli step and a ComfyUI step installs a package that no step uses. |
| 2026-11-15 | The package of each OS is plain data: `platform.Package`, given by `backend.SetupNeeds(goos)`. The service gets it with `Service.WithSystem`, and `provision.New` does not change. A service without a system plans as in Phase 3. | the T4 agent (design rules 1 and 4) | The backend knows what its install needs. The service uses no `runtime.GOOS` (archtest). No caller of `provision.New` changes. |
| 2026-11-15 | horus finds `winget` and `brew` only on the PATH. After a package step, it looks for the program in the folders of the package: `%ProgramFiles%\7-Zip`, and the folder of `brew` (the bin folder of Homebrew). No path of Homebrew is written in the code. | the T4 agent (design gap) | A test on `macos-latest` sets the PATH and so can never find the real `brew` of the runner. |
| 2026-11-15 | The test of `-yes -with-packages` for Windows replaces the install of the portable build with a fake (the seam `install` of `cmd/horus`). The fake also needs `7z` in the 7-Zip folder of `%ProgramFiles%`. The macOS case runs the real comfy-cli backend with the fake `brew`, a fake `python3.12` and the fake `comfy`. | the T4 agent (standing rule: a plan command that no longer fits) | The portable install downloads 1.9 GB. The test of `findSevenZip` proves that the real install finds `7z.exe` in that folder. |
| 2026-11-15 | `TestSetupWithoutYesChangesNoFile` now sets its system: Windows, with a fake `7z` on the PATH. Its checks did not change. | the T4 agent (standing rule: no test is made weaker) | Without that, the test reads the real PATH. On `macos-latest` the plan then has a comfy-cli step, and the step count of the test is wrong. |
| 2026-11-15 | The refusal is the pure method `Plan.Refusal(withPackages)`. `RunPlan` calls it first, and `horus setup` calls it before it prints "Running the plan.". | the T4 agent (design rule 3) | T5 (`install_workflow`) needs the same check before it changes anything. |
| 2026-11-15 | `install_workflow` reads the plan of `horus setup` first. Only a plan with a step that is not a model file takes the new path. A plan with only model files (ComfyUI is installed), or a plan that fails to read, goes on as before. | the T5 agent (spec: "A machine where ComfyUI is installed gets the same answers as before") | The old answers stay the same, and the contract check "a machine with ComfyUI: accept_setup=true changes no answer of install_workflow" proves it. |
| 2026-11-15 | The license consent is asked only when the plan downloads model files. A plan whose model files are present asks only the setup consent. | the T5 agent (design gap) | The license covers the model files. A call that downloads nothing has no license to accept. |
| 2026-11-15 | Both consents come before the first change. With `accept_setup=true` and without `accept_license`, the tool answers `LICENSE_CONFIRMATION_REQUIRED` and runs no step. | the T5 agent (design gap: the spec gives the order of the consents) | An install that stops halfway between the two consents leaves ComfyUI without its model files. |
| 2026-11-15 | A failed setup step (not a model file) gives `BACKEND_NOT_INSTALLED`, with the step in `details.step`, the last 20 output lines in `details.output_tail`, and a `next_step` that names `accept_setup=true`. A failed download keeps the codes of today. | the T5 agent (design gap) | ComfyUI is still not installed after the failure, and a person must fix the cause. No new code is needed: the spec has 22 codes. |
| 2026-11-15 | A progress notification goes out when each setup step starts ("setup step 1 of 2: install the system package 7-Zip: ..."), then every 5 s while the step runs. | the T5 agent (plan step 2) | A package step can end in less than 5 s. Without the first notification the caller never sees it. |
| 2026-11-15 | `TestCodesMatchPython` and `TestToolsMatchPython` put the one change of Phase 4 into the golden data in the test: the code `SETUP_CONFIRMATION_REQUIRED` after `LICENSE_CONFIRMATION_REQUIRED`, and the argument `accept_setup` with its description. Every other row and key is compared as before. The golden files do not change. | the T5 agent (standing rule: no golden file changes, no test is made weaker) | The spec adds the code and the argument. The tests name the exact difference, as the `.ps1` rule of `TestCodesMatchPython` does. |
| 2026-11-15 | The contract test cannot run a successful ComfyUI install on Windows: the portable install always downloads the pinned 1.9 GB archive with a fixed SHA-256, and no setting replaces it. So its install check runs the package step with a fake `winget` (Windows) or `brew` (macOS) that installs nothing, and checks that the tool stops at the missing program. The full install after `accept_setup=true` (package, ComfyUI, model files) is proven by `TestInstallWorkflowWithAcceptSetupRunsTheSetupSteps` on both runners, and for real by the fresh-machine run (T9). | the T5 agent (standing rule: a plan command that no longer fits) | A test seam in the shipped program only for the contract test adds a way to skip the install (design rule 1). The rule of the PC forbids a real download. |
| 2026-11-15 | The contract sessions without ComfyUI set the PATH to a folder of fake programs and the system folders only, `HOME`, `USERPROFILE`, `HORUS_HOME`, `HORUS_ROOT` and `ProgramFiles` to the sandbox, and `HTTP_PROXY` and `HTTPS_PROXY` to the closed port 127.0.0.1:9. | the T5 agent (rule of the PC: no real `winget` or `brew`, no real download) | So the real package manager is never found, and no download can leave the machine, also when a step has a defect. |
| 2026-11-15 | The goal tool has one count of contract checks, `CONTRACT_CHECKS = 37`. The row `contract` of phases 1 to 3 needs 37 of 37. The part of phase 4 needs 37 or more and each of the 30 baseline names. | the T5 agent (plan step 5) | The count went from 30 to 37. One constant keeps the tool and the CI job `contract` the same. |

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

## T1

Task: `horus version`, the one version variable, the release build script, the CI job `release-build`,
`release.yml` and the release-please config. Issue [#137](https://github.com/hieplam/horus/issues/137).

**Decisions of T1.**

| Decision | Why |
|---|---|
| The version variable is `version.Version` in the new base package `internal/version`. The linker flag is `-X github.com/hieplam/horus/internal/version.Version=<version>`. The package is in the layer table of `internal/archtest/layers.go` as `base`. | `cmd/horus` and `internal/mcpserver` both need the version, and the handler layer must not import the wiring layer. A package in `base` is the only place both can import. The constant `serverVersion` is removed. |
| `TestToolsMatchPython` no longer compares the server version with the `0.1.0` of the Python golden file. It compares the name only. `TestServerInfoVersionIsTheVersionOfTheBuild` checks the version. | The version is now the version of the build (`dev` in a test). The Python server is gone. The test still compares the name and every tool. |
| `release-please-config.json` has `"release-as": "0.1.0"` for the package `.`, and the manifest holds `"0.0.0"`. | With `bump-patch-for-minor-pre-major` a first `feat` from `0.0.0` gives `0.0.1`, not `0.1.0`. The docs of release-please say that `release-as` pins the next version and that it must be removed after the release. The follow-up issue #150 removes it (see below). |
| The version keys come from the current docs of release-please (`docs/manifest-releaser.md`, read 2026-11-15): `release-type`, `include-component-in-tag` (`false` gives the tag `v0.1.0`), `bump-minor-pre-major`, `bump-patch-for-minor-pre-major`, `release-as`. The action is `googleapis/release-please-action@v4`. Its outputs are `release_created` and `tag_name`. | The plan says not to trust memory. |
| `release.yml` job 1 has `contents: write` and `pull-requests: write`. Job 2 has `contents: write`. | The smallest permissions of the spec. The action docs also list `issues: write`. horus has no use for labels on issues, so the file does not give it. See "Run on main" below. |

**Red.** Before the change, on `main` at `594b970`:

```
$ go run ./cmd/horus version
horus: unknown command "version"
(usage text)
exit status 2
```

**Green.**

```
$ go run ./cmd/horus version
horus dev
$ go run -ldflags "-X github.com/hieplam/horus/internal/version.Version=1.2.3" ./cmd/horus version
horus 1.2.3
$ go test ./cmd/horus ./internal/mcpserver ./internal/archtest
ok
$ bash scripts/build-release.sh 0.0.0-ci .scratch/rel && ls .scratch/rel && (cd .scratch/rel && sha256sum -c checksums.txt)
build-release: 0.0.0-ci
checksums.txt  horus-darwin-arm64  horus-windows-amd64.exe
horus-windows-amd64.exe: OK
horus-darwin-arm64: OK
$ python tests/mcp_contract.py --server-cmd bin/horus.exe mcp
contract: 30 of 30 passed
```

**Ratchet.** `python tools/goal_check.py --phase 4 --no-ci`, rows passed: 1 → 2. The row `release` is now
`PASS  release - release files ok; 2 pinned tests pass; CI part skipped (--no-ci); real part not run`.
The plan expected 1 → 1, because it said that `release` passes only with CI. Without CI the row already
passes its file part and its two pinned tests, so the number is better than the plan. The Go test functions
went from 250 to 252 (the row `same-behaviour` needs 250 or more).

**Stub check.** A `version` command that prints a fixed text fails the `1.2.3` case of
`TestVersionPrintsTheVersionOfTheLinkerFlag`, because the test builds the program with the flag and runs it.
A server that gives a fixed version fails `TestServerInfoVersionIsTheVersionOfTheBuild`, which sets the variable
to `1.2.3`. A build script that writes an empty `checksums.txt` fails the CI step (3 files, 2 lines, `sha256sum -c`).

**Run on main (the setting of T10).** The action `release-please-action@v4` opens a pull request with the token
of the workflow. While the repository setting "Allow GitHub Actions to create and approve pull requests" is off,
GitHub refuses that call, so job `release-please` fails and the run of `release.yml` on `main` is red until T10.
The docs of the action name the setting and do not describe the failure. The result of the first run on `main`
after the merge of this pull request is in the comment of #146 and in the report of T1. The red run does not touch
the jobs of `ci.yml`. The file does not hide it: no `continue-on-error`.

## T2

Task: [#138](https://github.com/hieplam/horus/issues/138). The program holds the built-in workflows (`workflows/embed.go`,
`go:embed`) and copies them to `<user folder>/builtin-workflows` (`internal/settings/builtin.go`). The new rule for
the repo root, the user folder (`platform.UserDir`), `.env` and the output folder is in `internal/settings/settings.go`.

**Red** (before the change, built into the worktree, run from a temporary folder, `HORUS_ROOT` not set):

```
$ horus workflow list
FAIL: No workflows folder: <worktree>\bin\workflows
exit 1
```

**Green** (the same command, with `USERPROFILE`, `HOME` and `HORUS_HOME` in a scratch folder, so that the copy is not
written to the real home folder):

```
$ horus workflow list
it-illustration
  Text in, image out. The picture is 1536x1024. The model is FLUX 3 Image Klein (int8 build) running in ComfyUI. This is the first horus example.
  Models: 3 file(s), 11.02 GB (diffusion, text_encoder, vae). Installed: no (0 of 3 files) (size check).

Install: horus workflow install <name>    Check: horus workflow check <name>
exit 0
```

```
$ go test ./internal/settings ./internal/platform ./internal/catalog ./internal/archtest ./cmd/horus ./workflows
ok  	github.com/hieplam/horus/internal/settings
ok  	github.com/hieplam/horus/internal/platform
ok  	github.com/hieplam/horus/internal/catalog
ok  	github.com/hieplam/horus/internal/archtest
ok  	github.com/hieplam/horus/cmd/horus
ok  	github.com/hieplam/horus/workflows
$ python tools/goal_check.py --phase 4 --no-ci --only binary-only
PASS  binary-only - 6 pinned tests pass
goal: 1 of 1 rows passed
$ python tests/mcp_contract.py --server-cmd bin/horus.exe mcp
contract: 30 of 30 passed
$ python tests/workflow_parity.py
parity: 17 of 17 cases equal
$ python scripts/lint-public.py
lint-public: 0 finding(s)
```

**Ratchet.** `python tools/goal_check.py --phase 4 --no-ci`: the row `binary-only` goes from FAIL to PASS.
Rows passed 1 → 2 on the main of T0, and 2 → 3 after the rebase on the main with T1 (`release`, `binary-only`,
`same-behaviour`). Go test functions 250 → 263 (with T1). The answer "No workflows folder" outside a clone: 1 → 0.

**Stub check.** A `writeCopy` that writes nothing (no embedded workflows on disk) fails
`TestWorkflowListWithoutAClonePrintsTheBuiltInWorkflows` (exit 1),
`TestListWorkflowsOfTheMCPServerWithoutAClonePrintsTheBuiltInWorkflows` (`BACKEND_NOT_INSTALLED`) and
`TestHorusWorkflowsDirReplacesTheBuiltInWorkflows` ("the copy has no it-illustration"). An `IsClone` that accepts
each folder with `workflows/` fails `TestAFolderWithOnlyAWorkflowsFolderIsNotAClone` ("a folder with only a workflows
folder must not be a clone").

## T4

Task: [#140](https://github.com/hieplam/horus/issues/140). Files: `internal/provision/setup.go`,
`internal/provision/packages_test.go`, `internal/cli/setup.go`, `internal/backend/needs.go`,
`internal/backend/needs_test.go`, `internal/backend/portable.go`, `internal/platform/package.go`,
`cmd/horus/wiring.go`, `cmd/horus/setup_packages_test.go`, `cmd/horus/doctor_setup_test.go`.

**What changed.** `horus setup <workflow>` now plans the steps of the spec in this order: the package
(7-Zip with `winget` on Windows, Python 3.12 with `brew` on macOS), comfy-cli (macOS), ComfyUI, then the model files.
`-yes` with a package step and without `-with-packages` changes nothing and exits 1. Its text names the
package, the command, and the two ways to go on. Without a package manager, the step is a step for a person, and
`-yes` exits 1. Each `Step` has its kind, `Name` (what it installs), `Command` (a package step) and `Dir`. T5 needs
these fields for `details.steps`.

**Red** (before the change, on `594b970`):

```
$ go test ./internal/provision ./cmd/horus -run 'Package|WithPackages' -v
testing: warning: no tests to run
PASS
ok  	github.com/hieplam/horus/internal/provision	1.274s [no tests to run]
testing: warning: no tests to run
PASS
ok  	github.com/hieplam/horus/cmd/horus	0.238s [no tests to run]
```

**Green** (on the PC):

```
$ go test -count=1 ./internal/provision ./internal/cli ./internal/backend ./cmd/horus
ok  	github.com/hieplam/horus/internal/provision	13.842s
ok  	github.com/hieplam/horus/internal/cli	3.169s
ok  	github.com/hieplam/horus/internal/backend	47.478s
ok  	github.com/hieplam/horus/cmd/horus	10.095s
$ python tests/mcp_contract.py --server-cmd bin/horus.exe mcp
contract: 30 of 30 passed
$ python tests/workflow_parity.py
parity: 17 of 17 cases equal
$ python scripts/lint-public.py
lint-public: 0 finding(s)
```

`gofmt -l .` prints nothing. `GOOS=darwin GOARCH=arm64 go vet ./...` passes. The CI job `go darwin/arm64` of the
pull request runs the same tests on `macos-latest`.

**Stub check.**

- A plan without the package step (the condition of the package step made false) fails the order test:
  `packages_test.go:121: steps: backend, file diffusion, file text_encoder, file vae` (Windows) and
  `steps: comfy-cli, backend, ...` (macOS).
- A `-yes` that runs the package step without `-with-packages` (the refusal made false) leaves a line in the
  argument file of the fake `winget`, and the refusal test fails:
  `setup_packages_test.go:227: the package manager ran: "install --id 7zip.7zip -e --accept-source-agreements --accept-package-agreements\n"`.

**Ratchet.** The pinned tests of `full-setup` that pass, in the detail of the goal tool: 0 → 5 (the 5 tests of the
command). Before, the detail named all 7 pins. After, it names only the 2 pins of T5 and the count of error codes:

```
$ python tools/goal_check.py --phase 4 --no-ci --only full-setup --server-cmd <worktree>/bin/horus.exe mcp
FAIL  full-setup - no passing test: TestInstallWorkflowWithoutAcceptSetupAnswersSetupConfirmationRequired (not in internal/mcpserver, T5); TestInstallWorkflowWithAcceptSetupRunsTheSetupSteps (not in internal/mcpserver, T5); horus errors -json lists 21 codes, needs 22
```

Go test functions: 250 → 265.

## T6

Task: [#142](https://github.com/hieplam/horus/issues/142). Files: `internal/provision/register.go`, `internal/cli/mcptools.go`
and their tests, `docs/mcp.md`, `docs/setup.md`.

**Red** (before the change, program built into the worktree):

```
$ horus register -client codex
flag provided but not defined: -client
exit 2
```

**Green.** The real `codex` ran with `CODEX_HOME` in a scratch folder (`.scratch/codexhome` of the worktree). Before the
run, `codex mcp list` in that folder printed `No MCP servers configured yet.`, so `CODEX_HOME` is respected. The real
`codex mcp list` (real home) held no `horus` entry after the run. `HORUS_ROOT` was not set, so the repo root was the
worktree (a clone).

```
$ horus register -client codex
Added global MCP server 'horus'.
OK: registered for Codex. Start a new Codex session to see the tools.
exit 0
$ horus register -client codex
SKIP: Codex already knows a server named horus (codex mcp get horus).
exit 0
$ codex mcp get horus
horus
  enabled: true
  transport: stdio
  command: <worktree>\bin\horus.exe
  args: mcp
  env: HORUS_ROOT=*****
exit 0
$ horus register -client vim
horus register: unknown client "vim". The clients are: claude, codex.
exit 2
$ go test ./internal/provision ./internal/cli -run Register -v
--- PASS: TestRegisterClaudeRunsMCPAddWithTheArgumentsOfTheSpec
--- PASS: TestRegisterClaudeSkipsOnASecondRun
--- PASS: TestRegisterCodexRunsMCPAddWithTheArgumentsOfTheSpec
--- PASS: TestRegisterCodexSkipsWhenMCPGetExitsZero
--- PASS: TestRegisterPassesNoHorusRootWithoutARepoRoot
--- PASS: TestRegisterAnotherClientNameExitsTwo
(and the older Register tests, all PASS)
$ python tools/goal_check.py --phase 4 --no-ci --only register
PASS  register - 6 pinned tests pass; real part not run (needs --real on the PC after the release)
goal: 1 of 1 rows passed
```

**Decisions** (standing rules, no owner question):

- The `claude` fake and the `codex` fake are the same test program (`fakeClaudeMain`), because both clients take the same
  `mcp get` and `mcp add` shape.
- With a root that is set but has no `workflows/` folder (`-root <folder>`), `horus register` still fails with the old
  text. Only an empty root (no clone, no `-root`) passes no `HORUS_ROOT`, as the spec says.
- The Codex flag is `--env KEY=VALUE`, placed before the `--` (checked with `codex mcp add --help`).
- `README.md` is not changed: T8 owns it.

**Ratchet.** `python tools/goal_check.py --phase 4 --no-ci`: rows passed 3 (`release`, `binary-only`, `same-behaviour`) on the
main of this branch → 4 with `register`. (The plan says 4 → 5 because it counts the rows of T3 and T4 first. Those lanes are
not merged yet.) Clients that `horus register` serves: 1 → 2. Contract test: `contract: 30 of 30 passed`.

**Stub check.** A `register` that ignores `-client` runs `claude` and never gives the fake `codex` its arguments:
`TestRegisterClientCodexRunsCodex` (cli, `/bin/claude` is not the fake of `codex`) and
`TestRegisterCodexRunsMCPAddWithTheArgumentsOfTheSpec` fail.

## T3

Files: `install.sh`, `install.ps1`, the job `install` of `.github/workflows/ci.yml`. `scripts/lint-public.py` needed no change.

### What each script changes on the machine (for task T7, `horus uninstall`)

Windows (`install.ps1`):

| Item | Value |
|---|---|
| Program file | `%LOCALAPPDATA%\Programs\horus\horus.exe` (or `HORUS_INSTALL_DIR\horus.exe`). While an update runs, `horus.exe.old` exists for a moment. |
| PATH entry | The registry value `Path` of the key `HKCU\Environment` (type `REG_SZ` or `REG_EXPAND_SZ`, as it was). The script appends one entry at the end, separated by `;`: the install folder, with no trailing backslash. Written with `[Environment]::SetEnvironmentVariable('Path', ..., 'User')`. |
| When | Only when the install folder is not already an entry (compared without case and without a trailing backslash), and `HORUS_INSTALL_NO_PATH` is not `1`. |
| Uninstall | Remove only the entry that equals the install folder from that value. Keep every other entry. The program file is the running `horus.exe` itself, so it needs the rename trick that the installer uses. |

macOS (`install.sh`):

| Item | Value |
|---|---|
| Program file | `$HOME/.local/bin/horus` (or `$HORUS_INSTALL_DIR/horus`). |
| Profile file | `$HOME/.zprofile` when the basename of `$SHELL` is `zsh` (also when `SHELL` is not set), `$HOME/.bash_profile` for `bash`, `$HOME/.profile` for any other shell. |
| Profile line | One line, preceded by one empty line. The line is exactly `export PATH="<install folder>:$PATH" # added by the horus installer`. The marker text is `# added by the horus installer` (the end of the line). |
| When | Only when the install folder is not on `$PATH` of the script, the profile has no line with the marker, and `HORUS_INSTALL_NO_PATH` is not `1`. |
| Uninstall | Remove each line that contains the marker `# added by the horus installer` from the three profile files above. Keep every other line. The empty line before it may stay. |

Both scripts change nothing else. They write a temporary folder and delete it at the end. They print no environment variable.

### Decisions

| Date | Decision | Who | Why |
|---|---|---|---|
| 2026-11-15 | The profile of zsh is `.zprofile`, not `.zshrc`. | the T3 agent (design gap: the spec says "the shell profile") | The Terminal app of macOS starts a login shell, and a login shell reads `.zprofile`. The CI step starts `zsh -l` to test it. |
| 2026-11-15 | The CI step "a new shell" on Windows sets `$env:Path` from the registry (Machine and User values) before it runs `horus`. | the T3 agent | A child process of the runner inherits the PATH of the runner, not the registry. Reading the registry is what a new terminal does. A script that does not write the registry fails this step. |
| 2026-11-15 | `install.ps1` never calls `exit`. It throws on a failure. | the T3 agent | `iex` runs the script in the session of the user, and `exit` would close that window. A throw in `powershell -Command` gives exit code 1. |
| 2026-11-15 | The script downloads to a temporary folder and creates the install folder only after the SHA-256 matches. | the T3 agent | The plan needs an empty folder (or no folder) after a wrong checksum. |

### Local run on the PC (the CI jobs run the same steps on the runners)

The release assets came from `bash scripts/build-release.sh 0.0.0-ci`. The scripts ran with `HORUS_INSTALL_DIR` in a scratch folder under `.scratch` and `HORUS_INSTALL_NO_PATH=1` on Windows. A fake `uname` made `install.sh` run in Git Bash with a scratch `HOME`.

```
install.ps1 (pipe, base URL = local server):   Installed ...\inst1\horus.exe        exit=0
horus version                                  horus 0.0.0-ci
horus workflow list (run from %TEMP%)          it-illustration ...
wrong checksums.txt                            install.ps1: the SHA-256 of the download is not the SHA-256 in checksums.txt. Nothing is installed.
                                               expected: f4ff27... actual: 04ff27...   exit=1, install folder not created
second run on the first folder                 exit=0 (update)
install.sh (fake uname, scratch HOME, pipe)    Installed .../.local/bin/horus, profile .zprofile got one line with the marker, exit=0
install.sh second run                          exit=0, the marker is in the profile 1 time
install.sh wrong checksums.txt                 "Nothing is installed.", exit=1, folder not created
install.sh on MINGW64 x86_64 (no shim)         "this script supports macOS on Apple silicon (Darwin arm64) only ..." exit=1
python tools/goal_check.py --phase 4 --no-ci --only install-line
PASS  install-line - install scripts ok; CI part skipped (--no-ci); real part not run (needs --real ...)
```

### CI run 43687880114 (pull request #155): both jobs green

```
install windows-latest  horus 0.0.0-ci
install windows-latest  it-illustration                      (horus workflow list, run from the runner temp folder)
install windows-latest  install.ps1: the SHA-256 of the download is not the SHA-256 in checksums.txt. Nothing is installed.
install windows-latest  exit code: 1 / the folder is empty
install macos-latest    export PATH="$HOME/.local/bin:$PATH" # added by the horus installer   (the profile .zprofile)
install macos-latest    horus 0.0.0-ci                       (zsh -l, clean environment)
install macos-latest    it-illustration
install macos-latest    install.sh: the SHA-256 of the download is not the SHA-256 in checksums.txt. Nothing is installed.
install macos-latest    the folder is empty
```

Two CI test faults showed up and were fixed in this pull request: the runner already has `~/.local/bin` on its PATH (the macOS steps now run the script with a clean PATH), and the old wrong-checksum test changed the first character to `f`, which is no change when it already was `f` (it now changes `0` to `1` and any other character to `0`).

**Ratchet.** Goal rows passed 2 -> 3 once `python tools/goal_check.py --phase 4` reads these two green jobs (the part of `install-line` without `--real`).

## T5

Task: [#141](https://github.com/hieplam/horus/issues/141). Files: `internal/mcpserver/tool_install_workflow.go`,
`internal/mcpserver/setup.go`, `internal/mcpserver/tool_install_setup_test.go`, `internal/mcpserver/server_test.go`,
`internal/mcpserver/tool_setup_test.go`, `internal/herr/herr.go`, `internal/herr/herr_test.go`,
`tests/mcp_contract.py`, `docs/mcp.md`, `tools/goal_check.py`, `tools/test_goal_check.py`, `.github/workflows/ci.yml`.

**What changed.** `install_workflow` reads the plan of `horus setup` first. When the plan has a step that is not a
model file (a system package, comfy-cli, ComfyUI), the tool answers the new code `SETUP_CONFIRMATION_REQUIRED` with
`details.steps` until the call sends `accept_setup=true`. Then it asks the license consent when the plan downloads
model files. Then it runs the plan with `RunPlan`, with the package steps approved. A progress notification names each
setup step when it starts. A step for a person gives `BACKEND_NOT_INSTALLED` with a `next_step` that names the
package. On a machine with ComfyUI the plan has only model files, and the tool answers as before. The decisions are in
the "Decisions" table above (rows of the T5 agent).

**Red** (before the change, on `main` at `6dee6ca`, built into the worktree):

```
$ horus errors -json | python -c "import json,sys; print(len(json.load(sys.stdin)))"
21
$ python tools/goal_check.py --phase 4 --no-ci --only full-setup
FAIL  full-setup - no passing test: TestInstallWorkflowWithoutAcceptSetupAnswersSetupConfirmationRequired (not in internal/mcpserver, T5); TestInstallWorkflowWithAcceptSetupRunsTheSetupSteps (not in internal/mcpserver, T5); horus errors -json lists 21 codes, needs 22
```

**Green** (on the PC, after the rebase on `main` at `cc1a4e9`):

```
$ horus errors -json | python -c "import json,sys; print(len(json.load(sys.stdin)))"
22
$ go test -count=1 ./internal/mcpserver ./internal/herr
ok  	github.com/hieplam/horus/internal/mcpserver	0.476s
ok  	github.com/hieplam/horus/internal/herr	0.680s
$ python tests/mcp_contract.py --server-cmd bin/horus.exe mcp
PASS  server identifies as horus and has instructions
PASS  tools/list has exactly the 5 tools
PASS  every tool has an outputSchema with ok, result, error
PASS  uninstall_workflow is marked destructive, list_workflows read-only
PASS  list_workflows shows installed and partially_installed styles
PASS  describe_workflow gives subject rules, examples and 3 present files
PASS  list_workflows and describe_workflow carry no theme or look
PASS  describe_workflow hides the ComfyUI graph (no node class names)
PASS  unknown workflow -> WORKFLOW_NOT_FOUND with the known names, isError
PASS  generate_image writes a PNG at out_path and returns its path and seed
PASS  horus sends the subject to the model as written (0 characters added)
PASS  seed 7 reaches the sampler of the graph
PASS  generate_image sends progress notifications
PASS  existing out_path -> OUTPUT_EXISTS
PASS  width 1000 -> INVALID_ARGUMENT field=width, action_by=caller
PASS  too short subject -> INVALID_ARGUMENT field=subject with the rules
PASS  style without model files -> WORKFLOW_NOT_INSTALLED naming the file
PASS  GPU out of memory -> OUT_OF_MEMORY, retryable, action_by=caller
PASS  a job over the time limit -> GENERATION_TIMEOUT, and horus cancels it on the backend
PASS  install of an installed style -> ok, already_installed
PASS  install without accept_license -> LICENSE_CONFIRMATION_REQUIRED, action_by=human, with size
PASS  a machine with ComfyUI: accept_setup=true changes no answer of install_workflow
PASS  uninstall without confirm -> CONFIRMATION_REQUIRED, plan: 1 to delete, 2 shared kept; nothing deleted
PASS  uninstall with confirm deletes the 1 unshared file and keeps the 2 shared files
PASS  uninstall with remove_shared deletes the 2 shared files
PASS  after uninstall, list_workflows shows both styles not_installed
PASS  errors are the same envelope in structuredContent and in text
PASS  backend down and autostart off -> BACKEND_UNAVAILABLE, action_by=human
PASS  no ComfyUI and no package manager -> BACKEND_NOT_INSTALLED naming 7-Zip; nothing changed
PASS  no ComfyUI -> SETUP_CONFIRMATION_REQUIRED, action_by=human, with details.steps
PASS  no change on disk before the setup consent, also with accept_license=true
PASS  accept_setup=true without accept_license -> LICENSE_CONFIRMATION_REQUIRED; nothing changed
PASS  accept_setup=true runs winget with the arguments of the spec, then stops: 7z not found
PASS  install_workflow sends a progress message that names the setup step
PASS  it-illustration: manifest has no theme and no caption style file
PASS  it-illustration: the graph holds no style words
PASS  docs/mcp.md lists every error code
contract: 37 of 37 passed
$ python tools/goal_check.py --phase 4 --no-ci --only full-setup same-behaviour --server-cmd bin/horus.exe mcp
PASS  full-setup - 7 pinned tests pass; horus errors -json lists 22 codes; real part not run (needs --real on the PC after the release)
PASS  same-behaviour - contract: 37 of 37, 30 of 30 baseline checks pass; parity: 17 of 17; go test ./...: ok; archtest: ok, 0 allow list files; 290 Go test functions (baseline 250); 0 changed parity or golden files; CI part skipped (--no-ci); real part not run (needs --real on the PC after the release)
goal: 2 of 2 rows passed
note: the real part did not run for full-setup, same-behaviour (run with --real on the PC after the release)
```

`gofmt -l .` prints nothing. `go test ./...` passes. `python scripts/lint-public.py` prints `lint-public: 0 finding(s)`.
`python -m pytest tools/test_goal_check.py` prints `101 passed` (98 before: 3 new tests of the count of contract checks).

**Stub check.** Two stub programs, each built from this tree with one line changed in `installWithSetup`:

- Stub A runs the plan without the setup consent (`if false && !accept_setup`). The contract test prints
  `contract: 33 of 37 passed`. It fails "no ComfyUI -> SETUP_CONFIRMATION_REQUIRED ...", "no change on disk before the
  setup consent, also with accept_license=true", "accept_setup=true without accept_license -> ..." and the package
  step check (the fake `winget` ran three times). The Go test
  `TestInstallWorkflowWithoutAcceptSetupAnswersSetupConfirmationRequired` fails.
- Stub B ignores `accept_setup=true` (`if true || !accept_setup`). The contract test prints
  `contract: 34 of 37 passed`. It fails "accept_setup=true runs winget with the arguments of the spec, then stops: 7z
  not found" (`ran=[]`), the order check and the progress check. The Go tests
  `TestInstallWorkflowWithAcceptSetupRunsTheSetupSteps`, `TestInstallWorkflowAsksTheSetupConsentBeforeTheLicense` and
  `TestInstallWorkflowNamesTheSetupStepThatFailed` fail.

**Ratchet.** `python tools/goal_check.py --phase 4 --no-ci`, rows passed: 3 -> 4 on the main of T4 (`release`,
`binary-only`, `same-behaviour`, and now `full-setup`). After the rebase on the main with T3 and T6 the tool prints
`goal: 6 of 8 rows passed` (`install-line` of T3 and `register` of T6 also pass; `uninstall` and `readme` wait for T7, T8
and T9). Contract checks: 30 -> 37 (7 new checks, each of the 30 baseline names passes). Error codes: 21 -> 22.
Go test functions: 290 (baseline 250).

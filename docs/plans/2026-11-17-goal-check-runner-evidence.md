# Evidence: split the goal check into a runner and suites (2026-11-17)

## Goal

| Goal row | Oracle | Result |
|---|---|---|
| A new feature needs only a new suite file, `tools/suites/<name>.toml`. The runner does not change, and no Python code is needed when the general check types fit. | The test `NewSuite` of `tools/test_goal_runner.py` writes a suite `feature-x.toml` (a command, files, a real part) into a scratch suites folder. `--list` shows it, and `--suite feature-x` runs it. | Pass: 3 rows, the expected PASS and FAIL lines, and the note line. |
| A suite file with a mistake does not run, and the message names the row and the part. | The tests `BadSuite` (unknown check type, wrong parameter, missing parameter, row without a part, two rows with one name, real part without `needs`, missing title). | Pass: 9 tests. |
| The goal check gives the same output for each row of Phases 1 to 4 as before the change. | The output of `tools/goal_check.py --phase N --no-ci` before and after, on the same commit and with the same `horus` program. | 4 of 4 outputs are equal, byte for byte. |
| The unit tests of the four phases still pass. | `python -m unittest discover -s tools` | 127 tests pass: 104 of the phases (the 103 old tests and `OneNameOneItem`) and 23 of the runner. |

## The new layout

| File | What it holds |
|---|---|
| `tools/goal_check.py` | The command. It calls `goal_runner.main`. The old command lines still work. |
| `tools/goal_runner.py` | The runner. It reads a suite file, checks each part, runs the rows and prints the lines. |
| `tools/suites/<name>.toml` | One suite: `title`, `spec`, and the rows. Each row lists its parts as data. A part names a check type and its parameters. |
| `tools/suites/README.md` | The format, the check types, and how to add a check type. A test loads its example. |
| `tools/checks/parts.py` | The 12 general check types: `go_pinned_tests`, `go_tests`, `go_test_all`, `test_count`, `archtest`, `ci_green`, `command`, `files`, `no_changes`, `contract`, `parity`, `mcp_image`. |
| `tools/checks/base.py` | The general parts under the check types: commands, Go tests, pinned tests, CI jobs, the join of parts. |
| `tools/checks/horus.py` | The checks of this repo that more than one suite uses: the contract test, the parity test, archtest, a real image, ComfyUI, the `horus` program. |
| `tools/checks/phase1.py` to `phase4.py` | The checks that only one spec needs. A suite names them as `"phase3.real_tcc"`. |

The rows of Phases 1 to 4 were Python code before the change. Of the 32 rows, 5 now use only general
check types (for example `binary-only` and `out-of-memory`). 11 rows mix general check types with a
check of their spec (for example `release`: `go_pinned_tests`, `ci_green` and `phase4.release_files_part`).
16 rows name only a function of `tools/checks/phase<N>.py`, because their check exists for that spec only.

A check module uses a part of another module through that module (`r.run`, `horus.real_image`), never
with `from ... import`. Then a unit test can replace the part in one place. The test `OneNameOneItem`
fails when two modules define one name, because a test replaces a part by its name.

## Ratchet: before and after on the same tool

The first version of this change (Python suites, commit `3124242`) and the TOML version (this commit)
were both compared with the same baseline. Commit `763b1bd` (origin/main). `horus` on the PATH was `%USERPROFILE%\go\bin\horus.exe`, an old
build from 2026-11-13 (it has no `horus version`). The same program ran before and after the change.

| Suite | Before, `--no-ci` | After, `--no-ci` | Before, with CI | After, with CI |
|---|---|---|---|---|
| phase1 | 5 of 8 | 5 of 8 | 5 of 8 | 5 of 8 |
| phase2 | 4 of 8 | 4 of 8 | 4 of 8 | 4 of 8 |
| phase3 | 6 of 8 | 6 of 8 | 6 of 8 | 6 of 8 |
| phase4 | 6 of 8 | 6 of 8 | 6 of 8 | 6 of 8 |

`diff` of each pair of outputs: no difference. The TOML version: the 4 `--no-ci` outputs are equal to the baseline, byte for byte, with the same old build (a copy of it, first on the PATH).

## Why rows failed before the change

The rows `contract` (Phase 1), `same-behaviour` (Phases 2, 3 and 4) and `full-setup` (Phase 4) start
`horus mcp` and `horus errors -json` from the PATH when `--server-cmd` is not given. The old build on
the PATH answered 32 of 37 contract checks and listed 21 error codes. The code on `main` answers 37
and lists 22. With a fresh build of `main` (`--server-cmd <fresh>/horus.exe mcp --no-ci`):

| Suite | Old build on the PATH | Fresh build of `main` |
|---|---|---|
| phase1 | 5 of 8 | 6 of 8 (`contract` passes) |
| phase2 | 4 of 8 | 4 of 8 |
| phase3 | 6 of 8 | 6 of 8 |
| phase4 | 6 of 8 | 8 of 8 |

The rows that still fail do not depend on the program:

| Row | Why it fails | Kind |
|---|---|---|
| phase1 `real-image`, phase2 `backend` and `commands`, phase3 `mac-image` | They need `--real` on the GPU machine or on the Mac. | Expected without `--real` |
| phase1 `no-python` | Six Go files name `powershell` (the Windows install and the PATH entry of Phase 4). The Phase 1 row predates them. | The Phase 1 oracle is out of date |
| phase2 `layout` | `go list ./...` has 3 packages that `tools/phase2_packages.txt` does not list: `internal/backend/comfyclitest`, `internal/version` and `workflows` (added in Phases 3 and 4). | The Phase 2 oracle is out of date |
| phase2 and phase3 `same-behaviour` | `tests/mcp_contract.py` changed after the baseline of Phases 2 and 3. Task T5 of Phase 4 added 7 checks. | The oracle of the older phase is out of date |

This change does not touch these rows. Their result is the same before and after.

## Commands

```
python tools/goal_check.py --phase <N> [--no-ci]                                   # before and after
python tools/goal_check.py --phase <N> --no-ci --server-cmd <fresh>/horus.exe mcp  # fresh build
python -m unittest discover -s tools                                               # 118 tests
```

# Evidence: split the goal check into a runner and suites (2026-11-17)

## Goal

| Goal row | Oracle | Result |
|---|---|---|
| A new spec needs only a new file in `tools/suites/`. The runner does not change. | `python tools/goal_check.py --list` prints each suite. The test `CommandLine` of `tools/test_goal_runner.py` runs a suite by name. | 4 suites listed. 15 of 15 runner tests pass. |
| The goal check gives the same result for each row of Phases 1 to 4 as before the change. | The output of `tools/goal_check.py` before and after the change, for each phase, with and without CI, on the same commit and with the same `horus` program. | 8 of 8 outputs are equal, byte for byte. |
| The unit tests of the four phases still pass. | `python -m unittest discover -s tools` | 118 tests pass: the 103 old tests and the 15 new tests. |

## The new layout

| File | What it holds |
|---|---|
| `tools/goal_check.py` | The command. It calls `goal_runner.main`. The old command lines still work. |
| `tools/goal_runner.py` | The runner. It loads a suite, runs the rows and prints the lines. It also holds the parts that many rows use: run a command, Go tests, pinned Go tests, the CI jobs of HEAD, join the parts of a row, join a real part. |
| `tools/suites/common.py` | The checks of this repo that more than one suite uses: the contract test, the parity test, archtest, a real image, ComfyUI and the `horus` program. It is no suite. |
| `tools/suites/phase1.py` to `phase4.py` | One suite for each spec: `TITLE`, `SPEC`, `ROWS`, `CHECKS`, `REAL_WHERE`, and the checks that only that spec needs. |

A suite uses a part of another module through that module (`r.run`, `common.real_image`), never with
`from ... import`. Then a unit test can replace the part in one place, and every suite sees the replacement.

## Ratchet: before and after on the same tool

Commit `763b1bd` (origin/main). `horus` on the PATH was `%USERPROFILE%\go\bin\horus.exe`, an old
build from 2026-11-13 (it has no `horus version`). The same program ran before and after the change.

| Suite | Before, `--no-ci` | After, `--no-ci` | Before, with CI | After, with CI |
|---|---|---|---|---|
| phase1 | 5 of 8 | 5 of 8 | 5 of 8 | 5 of 8 |
| phase2 | 4 of 8 | 4 of 8 | 4 of 8 | 4 of 8 |
| phase3 | 6 of 8 | 6 of 8 | 6 of 8 | 6 of 8 |
| phase4 | 6 of 8 | 6 of 8 | 6 of 8 | 6 of 8 |

`diff` of each pair of outputs: no difference.

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

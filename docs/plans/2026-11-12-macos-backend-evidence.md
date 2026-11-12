# Evidence log: horus on macOS through comfy-cli (Phase 3)

Spec: [`../specs/2026-11-12-macos-backend.md`](../specs/2026-11-12-macos-backend.md).
Plan: [`2026-11-12-macos-backend.md`](2026-11-12-macos-backend.md).
Epic: [#98](https://github.com/hieplam/horus/issues/98).

Each task adds one section `## <task id>` with the literal output of its Green
commands.

## Decisions

| Date | Decision | Why | By |
|---|---|---|---|
| 2026-11-12 | The owner ratified the 8 goal rows, the task list and the standing rules of the spec. | The one ratification of this phase. | Owner |
| 2026-11-12 | The default install folder on macOS is `<home>/horus`, with ComfyUI in `<home>/horus/ComfyUI`. | It is outside the TCC folders, it has no space in its path, and a user can see it. | Owner |
| 2026-11-12 | The session on the Mac uses the account `hoacao33`. The owner merges its pull requests by hand. | The owner's merge rule lets an agent merge only a pull request of the owner. | Owner |
| 2026-11-12 | The real run on the Mac downloads the model files again (11 GB) into `~/horus`. `~/horus-lab` is not changed. | It proves the whole setup from nothing. | Owner |
| 2026-11-12 | Issues #82 and #91 are part of this phase (tasks T3 and T6). | They change `internal/doctor` and `docs/setup.md`, and this phase changes both. | Plan |
| 2026-11-12 | `HORUS_GENERATE_TIMEOUT` keeps its default of 1800 s. | The cold run of #53 took 1033 s, so 767 s are spare. | Plan |
| 2026-11-12 | `horus doctor` gets a WARN level that does not change the exit code. | The clone on the Mac is inside `~/Desktop`. A FAIL line for it could never pass, and the row `tcc-folders` asks for a warning. | Plan |
| 2026-11-12 | The comfy-cli backend writes the same state file as the Windows backend, and `Stop` reads it. | comfy-cli 1.25.0 keeps one background ComfyUI for each user (`comfy_cli/command/launch.py`, `ConfigManager().background`), and `comfy stop` stops that one. Without a record of its own, horus could stop a ComfyUI that a person started. | Plan |
| 2026-11-12 | `.mcp.json` starts the comfy MCP server as `horus comfy-mcp`, a new command that runs the `comfy-mcp` of the venv or of the PATH. The draft said "comfy-mcp as a command on the PATH". | A venv is not on the PATH, and `uv` is not installed on the PC. With `horus comfy-mcp` the PATH needs only `horus` on both systems, and the PC needs no new program. The row `one-config` holds: each entry names a command on the PATH, and the Windows-only paths go from 2 to 0. | Plan |
| 2026-11-12 | The comfy-cli backend finds `comfy` on the PATH, then in the venv of `horus install-comfy-mcp`. | That venv already holds comfy-cli 1.25.0, so a Mac needs no second install of it and no PATH entry for it. | Plan |
| 2026-11-12 | The verify task of the draft ("the Mac and the PC") is two tasks: T8 on the PC, then T9 on the Mac. T8 waits for the real run on the Mac (T7) and the docs (T6). | One session cannot run on two machines. The final commit is verified on both. | Plan |
| 2026-11-12 | The baseline of `same-behaviour` is commit `943d430` with 215 Go test functions (`go test -list . ./...`). | Measured on `main` before the first task. | Plan |
| 2026-11-12 | Without `--real`, a row that has a real part stands on its part without `--real`, as a row stands on its local part with `--no-ci`. Its detail says `real part not run`, and a `note:` line after the summary names those rows. `mac-image` has no part without `--real`, so it fails with `needs --real on macOS`. | The Greens of T2, T3 and T5 ask for `PASS` of the part without `--real`. "Done" still needs all 8 rows with `--real` on the Mac and on the PC (T8, T9), so no row proves less. | T0 |
| 2026-11-12 | The real part of `same-behaviour` (the Phase 2 rows `backend` and `commands` with `--real`) runs only on Windows. On macOS `--real` adds nothing to that row. The rows `mac-backend`, `mac-setup`, `mac-image` and `tcc-folders` with `--real` on Windows fail with `the real part needs macOS`. | T9 needs `goal: 8 of 8 rows passed` on the Mac. The PC part of `same-behaviour` is proven by T8 on the PC. | T0 |
| 2026-11-12 | A pinned Go test passes its row only when its name reports `--- PASS` in `go test -v -run` of its package, and when its body asserts something: a call of `t.Error`, `t.Errorf`, `t.Fatal`, `t.Fatalf`, `t.Fail`, `t.FailNow`, or `t` given to a helper. The two tests of `out-of-memory` must also hold the text `MPS backend out of memory` in their body. | An empty test body passes `go test`. A test with the pinned name of `out-of-memory` and a CUDA text proves nothing about MPS. | T0 |
| 2026-11-12 | The CI part of Phase 3 (`mac-backend` and `same-behaviour`) also fails when another job of the same CI run is red, cancelled or still running. A skipped or neutral job is no failure. The rows of Phase 1 and Phase 2 do not change. | A CI part that ignores a red job was a hole of Phase 2 (#81). | T0 |
| 2026-11-12 | On macOS with `--real`, the goal tool runs `horus start-comfyui` before a `horus doctor` when the server does not answer. Before the cold image of `mac-image` and before the start of `mac-backend`, it runs `horus stop-comfyui` when a server answers, and fails when a server still answers. On Windows the tool never starts or stops ComfyUI. | `horus doctor` fails when the server does not answer. `mac-image` needs a cold start. Each row must give the same result alone (`--only`) and in a full run. | T0 |
| 2026-11-12 | The real part of `mac-backend` runs start, stop and the refusal on the install of the settings (default `~/horus`). It does not install into a new folder. | Step 3 of T0 names start, stop and the refusal. The install from nothing is proven by `mac-setup` in T7 with the default folder. | T0 |
| 2026-11-12 | The real part of `tcc-folders` needs a line of `horus doctor` that starts with `WARN` and holds the absolute path of the repo root. T3 must print the absolute path. | The row asks for "a WARN line that names the path". | T0 |
| 2026-11-12 | The row `tcc-folders` pins 4 tests: the test of the default folder (T1) and 3 tests of the WARN line (T3: the pure function, the doctor service, the exit code of the handler). After T1, the row names these 3 tests as missing. The plan says "names only the WARN test". | The WARN line has three parts, and a stub of one of them must fail the row. | T0 |
| 2026-11-12 | The row `mac-backend` also pins the test of step 4 of T2 (`cmd/horus`: a generate call starts ComfyUI through the comfy-cli backend). The row `one-config` also pins the test of `horus comfy-mcp` (T5). | Without the second pin, an `.mcp.json` that names `horus comfy-mcp` passes when the command does not exist. | T0 |
| 2026-11-12 | The Green of T0 ran the goal tool with `--server-cmd <scratch>/horus.exe mcp`, a binary of the same commit from `go build -o`. The plan says `horus`. | The PC rule: a running `horus mcp` holds `horus.exe`, so `go install` is not allowed here. The binary is the same code. | T0 |
| 2026-11-12 | `pytest` is not in `tests/requirements.txt`. The test venv got it with `pip install pytest`. `python -m unittest discover -s tools` runs the same tests without it. | The plan's Green names `python -m pytest`. A change of `tests/requirements.txt` changes the CI install, and no row needs it. | T0 |

## Baseline

Measured on the PC (Windows) on `main` at `5614901` (the spec commit after `943d430`; it
changes only docs), before the first build task.

The goal tool, `python tools/goal_check.py --phase 3 --no-ci --server-cmd <scratch>/horus.exe mcp`:

```
FAIL  mac-backend - no passing test: TestComfyCLIIsTheBackendOfDarwin (not in internal/backend, T1); TestComfyCLIInstallRunsTheCommandOfTheSpec (not in internal/backend, T1); TestComfyCLIInstallSkipsWhenComfyUIIsInstalled (not in internal/backend, T1); TestComfyCLIInstallNamesBothFixesWhenComfyIsMissing (not in internal/backend, T1); TestComfyCLIStartRunsLaunchAndTheServerAnswers (not in internal/backend, T2); TestComfyCLIStartFailsWhenComfyUIIsNotInstalled (not in internal/backend, T2); TestComfyCLIStopRunsStopAndTheServerEnds (not in internal/backend, T2); TestComfyCLIStopRefusesAComfyUIThatHorusDidNotStart (not in internal/backend, T2); TestComfyCLIStopWithAStaleStateRunsNoStop (not in internal/backend, T2); TestGenerateStartsComfyUIThroughTheComfyCLIBackend (not in cmd/horus, T2)
FAIL  mac-setup - no passing test: TestDefaultComfyDirOfEachOS (not in internal/platform, T1); TestComfyDirDefaultIsTheFolderOfPlatform (not in internal/settings, T1); TestSetupYesWithTheFakeComfyThenASecondPlanHasZeroSteps (not in cmd/horus, T3)
FAIL  mac-image - needs --real on macOS
FAIL  tcc-folders - no passing test: TestDefaultComfyDirOfEachOS (not in internal/platform, T1); TestInTCCFolder (not in internal/platform, T3); TestTCCCheckWarnsForEachPathInATCCFolder (not in internal/doctor, T3); TestDoctorWarnLinesKeepExitCodeZero (not in internal/cli, T3)
FAIL  out-of-memory - no passing test: TestJobErrorOfTheMPSOutOfMemoryText (not in internal/comfy, T4); TestImageMPSOutOfMemoryThroughTheAdapter (not in cmd/horus, T4)
FAIL  one-config - .mcp.json: 2 Windows-only paths, the entry comfy-mcp is not `horus comfy-mcp`; no passing test: TestDoctorUsesHorusComfyUIURLWithoutTheFlag (not in internal/cli, T3); TestComfyMCPPassesTheStreamsComfyBinAndTheExitCode (not in cmd/horus, T5)
FAIL  one-setup-doc - 4 lines that the commands contradict; docs/setup.md:18, docs/setup.md:112, internal/cli/setup.go:38, internal/cli/workflow.go:89, docs/setup.md has no macOS section
PASS  same-behaviour - contract: local 30 of 30; parity: 17 of 17; go test ./...: ok; archtest: ok, 0 allow list files; 215 Go test functions (baseline 215); 0 changed contract files; CI part skipped (--no-ci); real part not run (needs --real on Windows)
goal: 1 of 8 rows passed
note: the real part did not run for same-behaviour (run with --real on the Mac and on the PC)
exit 1
```

| Number | Baseline | Evidence |
|---|---|---|
| Go test functions (`go test -list . ./...`) | 215 | the lines that match `^Test\w+$` in the list |
| Commands that answer "not supported" on macOS | 3: `horus install-comfyui`, `horus start-comfyui`, `horus stop-comfyui` | `backend.New` gives the `unsupported` backend on every OS except Windows (`internal/backend/backend.go:63-76`). Its `Install`, `Start` and `Stop` return `platform.ErrNotSupported`. The handler prints `horus <command>: not supported on this OS yet` and exits 2 (`internal/cli/comfyui_test.go:53`). |
| Windows-only paths in `.mcp.json` | 2 | `.mcp.json:5` (`command`: `${USERPROFILE}\.venvs\comfy-mcp\Scripts\comfy-mcp.exe`) and `.mcp.json:8` (`COMFY_BIN`: `...\Scripts\comfy.exe`) |
| Lines that the commands contradict (#91) | 4 | `docs/setup.md:18`, `docs/setup.md:112`, `internal/cli/setup.go:38`, `internal/cli/workflow.go:89` |
| Goal rows that pass | 1 of 8 | the output above |

### The pinned test names

Tasks T1 to T5 write Go tests with exactly these names. The goal tool runs them with
`go test -count=1 -v -run '^(<names>)$' ./<package>` and needs `--- PASS: <name>`. The
body of each test must assert something.

| Row | Package | Test name | Task |
|---|---|---|---|
| `mac-backend` | `internal/backend` | `TestComfyCLIIsTheBackendOfDarwin` | T1 |
| `mac-backend` | `internal/backend` | `TestComfyCLIInstallRunsTheCommandOfTheSpec` | T1 |
| `mac-backend` | `internal/backend` | `TestComfyCLIInstallSkipsWhenComfyUIIsInstalled` | T1 |
| `mac-backend` | `internal/backend` | `TestComfyCLIInstallNamesBothFixesWhenComfyIsMissing` | T1 |
| `mac-backend` | `internal/backend` | `TestComfyCLIStartRunsLaunchAndTheServerAnswers` | T2 |
| `mac-backend` | `internal/backend` | `TestComfyCLIStartFailsWhenComfyUIIsNotInstalled` | T2 |
| `mac-backend` | `internal/backend` | `TestComfyCLIStopRunsStopAndTheServerEnds` | T2 |
| `mac-backend` | `internal/backend` | `TestComfyCLIStopRefusesAComfyUIThatHorusDidNotStart` | T2 |
| `mac-backend` | `internal/backend` | `TestComfyCLIStopWithAStaleStateRunsNoStop` | T2 |
| `mac-backend` | `cmd/horus` | `TestGenerateStartsComfyUIThroughTheComfyCLIBackend` | T2 |
| `mac-setup`, `tcc-folders` | `internal/platform` | `TestDefaultComfyDirOfEachOS` | T1 |
| `mac-setup` | `internal/settings` | `TestComfyDirDefaultIsTheFolderOfPlatform` | T1 |
| `mac-setup` | `cmd/horus` | `TestSetupYesWithTheFakeComfyThenASecondPlanHasZeroSteps` | T3 |
| `tcc-folders` | `internal/platform` | `TestInTCCFolder` | T3 |
| `tcc-folders` | `internal/doctor` | `TestTCCCheckWarnsForEachPathInATCCFolder` | T3 |
| `tcc-folders` | `internal/cli` | `TestDoctorWarnLinesKeepExitCodeZero` | T3 |
| `out-of-memory` | `internal/comfy` | `TestJobErrorOfTheMPSOutOfMemoryText` (the body holds `MPS backend out of memory`) | T4 |
| `out-of-memory` | `cmd/horus` | `TestImageMPSOutOfMemoryThroughTheAdapter` (the body holds `MPS backend out of memory`) | T4 |
| `one-config` | `internal/cli` | `TestDoctorUsesHorusComfyUIURLWithoutTheFlag` (#82) | T3 |
| `one-config` | `cmd/horus` | `TestComfyMCPPassesTheStreamsComfyBinAndTheExitCode` | T5 |

The Red of T1 (`-run ComfyCLI`) and the Red of T2 (`-run 'ComfyCLI.*(Start|Stop)'`) still
hold with these names: no name of T1 matches `ComfyCLI.*(Start|Stop)`.

## T0

Red, on `main` at `5614901`:

```
python tools/goal_check.py --phase 3
usage: goal_check.py [-h] [--server-cmd SERVER_CMD [SERVER_CMD ...]] [--no-ci]
                     [--real] [--real-out REAL_OUT] [--phase {1,2}]
                     [--only ROW [ROW ...]]
goal_check.py: error: argument --phase: invalid choice: '3' (choose from 1, 2)
exit 2
```

Green 1: `python tools/goal_check.py --phase 3 --no-ci --server-cmd <scratch>/horus.exe mcp`
prints the 8 row lines of the section "Baseline", then `goal: 1 of 8 rows passed` (only
`same-behaviour` passes), and exits 1.

Green 2: `python -m pytest tools/test_goal_check.py`:

```
collected 52 items

tools\test_goal_check.py ............................................... [ 90%]
.....                                                                    [100%]

======================== 52 passed in 89.90s (0:01:29) ========================
```

Stub check: `tools/test_goal_check.py` feeds each row a stub and needs FAIL. The Go rows run the
real `go` program on a small module in a temporary folder.

| Row | Stubs that must FAIL | Input that must PASS |
|---|---|---|
| all Go rows | a package with no test files (#80), a missing package, a missing name, a name that is only a prefix, an empty body, a skipped test, a failing test | every pinned test, with a body that asserts |
| `mac-backend` | a red `go darwin/arm64` job; `--real` on Windows; a `stop-comfyui` that does not refuse; a `start-comfyui` that starts nothing | green CI; a fake horus and comfy that start, stop and refuse |
| `mac-setup` | no settings test; `HORUS_COMFYUI_DIR` set; a plan with 2 steps | `Plan: 0 steps` and 0 FAIL lines |
| `mac-image` | no `--real`; `--real` on Windows; `started_backend: false`; a ComfyUI that horus did not start | a cold image with `started_backend: true` |
| `tcc-folders` | only the default folder test (the state after T1); no WARN line that names the repo; a FAIL line | a WARN line with the repo path and exit 0 |
| `out-of-memory` | a pinned test without the MPS text | both tests with the MPS text |
| `one-config` | the old `.mcp.json`; no `.mcp.json`; a `comfy-mcp` entry that is not `horus comfy-mcp`; no test of `horus comfy-mcp`; a FAIL line of the doctor | the new `.mcp.json` and both tests |
| `one-setup-doc` | the 4 lines of #91; no macOS section; no `docs/setup.md` | the fixed page with a macOS section |
| `same-behaviour` | each of: contract 29 of 30, parity 16 of 17, a red `go test ./...`, archtest without `TestPlanted`, 214 Go test functions, a changed contract file, an allow list file, a red `contract macos-latest` job, a red `go linux/amd64` job (#81); `--real` on Windows with a failing Phase 2 row `backend` | all facts good |

Ratchet: goal rows with a committed check 0 → 8.

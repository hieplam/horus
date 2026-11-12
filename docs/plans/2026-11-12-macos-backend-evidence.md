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
| 2026-11-12 | The `next_step` of `OUT_OF_MEMORY` stays as it is. The time limit stays 1800 s. The fake ComfyUI gets the mode `OOMMPS` for the MPS text. | The text ("use a smaller width and height") is true on a Mac too, and the golden file does not change in Phase 3. The spec keeps 1800 s. A new fake mode leaves the mode `OOM` and its tests as they are. | T4 |
| 2026-11-12 | `backend.New(Place, Archive)` replaces `backend.New(*Portable)`. `Place` (folder, address, waits) is what every backend takes. `Archive` (`ArchivePath`, `SevenZip`, `KeepArchive`) only the Windows portable build reads. The two backend types are not exported (`portable`, `comfyCLI`). | Step 2 of T1: the constructor takes the values of both backends and the values of the Windows backend apart, and the interface of the package stays small. `Place` holds `dir`, `serverURL` and `Installed` once, so the two backends share them and copy nothing. | T1 |
| 2026-11-12 | The handler `cli.InstallComfyUI` gets a parameter `archive bool`. cmd/horus gives it `backend.TakesArchive()`. Without the archive, the three archive flags exit 2 with `horus install-comfyui: -<flag> is a flag of the Windows portable build: not supported on this OS yet`. | The handler must not read the OS (archtest), and a handler cannot import an adapter. `TakesArchive` and `New` read one function (`takesArchive`), so the flags and the backend cannot disagree. | T1 |
| 2026-11-12 | The default folder is the pure function `platform.DefaultComfyDir(home)`. Its test calls `defaultComfyDir(goos, home)` for each OS. The help texts show it with `filepath.ToSlash(platform.DefaultComfyDir("<home>"))`. | The help text on Windows stays `<home>/ComfyUI_windows_portable`, character for character. | T1 |
| 2026-11-12 | T1 changes the help texts that write `<home>/ComfyUI_windows_portable` (`install-comfyui`, `start-comfyui`, `stop-comfyui`, `sync-workflows`, `doctor`). It does not change `setup -h` and `workflow -h` (`then ~/ComfyUI_windows_portable`). The words "the portable folder" stay. | The Red of T6 (#91) needs those 2 lines. T6 owns the words of the help texts. | T1 |
| 2026-11-12 | The venv of `horus install-comfy-mcp` is one function, `settings.ComfyMCPVenv()`. `horus doctor`, `horus install-comfy-mcp` and the comfy-cli backend call it. `install-comfy-mcp` no longer has its own error for a missing home folder. | Three copies of `<home>/.venvs/comfy-mcp` become one. T5 (`horus comfy-mcp`) needs the same folder. | T1 |
| 2026-11-12 | `TestOtherOSAnswersNotSupported` now checks Linux and FreeBSD, not macOS. The macOS part moved to `TestComfyCLIIsTheBackendOfDarwin`: Start and Stop still answer "not supported", Install now works. | The behaviour of macOS changed by design (ratchet 3 → 2). No check was removed: each old assertion for macOS has a new one. | T1 |
| 2026-11-12 | The summary of `install-comfyui` in the command list of `horus` is now "install ComfyUI: the portable build on Windows, comfy-cli on macOS". `start-comfyui` and `stop-comfyui` keep "Windows only" until T2. | The old summary says "Windows only", which is false after T1. No golden file holds it. | T1 |

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

## T4

Red (on `main` at `003b840`): `git grep -n "MPS backend out of memory" -- '*.go'` printed nothing.
The code already maps the text (`internal/comfy/jobs.go:95`), so the Red is the missing proof.

Green:

```
$ git grep -n "MPS backend out of memory" -- '*.go' | wc -l
7
$ go test -count=1 ./internal/comfy ./cmd/horus
ok  	github.com/hieplam/horus/internal/comfy
ok  	github.com/hieplam/horus/cmd/horus
$ python tools/goal_check.py --phase 3 --no-ci --only out-of-memory
PASS  out-of-memory - 2 pinned tests pass
goal: 1 of 1 rows passed
```

Also green: `go test ./...` (all packages ok), `gofmt -l .` (no output), `python scripts/lint-public.py`
(`0 finding(s)`), `tests/mcp_contract.py` (`contract: 30 of 30 passed`, binary from `go build -o`).
The golden files and the contract test did not change. `internal/comfy/jobs.go` did not change.

Stub check: in a scratch edit of `internal/comfy/jobs.go`, the three text matches (`outofmemory`,
`out of memory`, `allocation on device`) were replaced by `false`. Then the new tests ran:

```
--- FAIL: TestJobErrorOfTheMPSOutOfMemoryText (0.00s)
    comfy_test.go:206: got GENERATION_FAILED retryable=true action_by=retry
--- FAIL: TestImageMPSOutOfMemoryThroughTheAdapter (0.03s)
    image_test.go:164: got {Code:GENERATION_FAILED Message:the backend failed while it ran the job Retryable:true ActionBy:retry ... backend_message:MPS backend out of memory (MPS allocated: 9.07 GB, ...
```

`jobs.go` was restored with `git checkout`. (`go vet` rejects `false || false`, so the first run used
`-vet=off` for the `internal/comfy` package.)

Ratchet: test cases with the MPS text 0 → 2 (`TestJobErrorOfTheMPSOutOfMemoryText`,
`TestImageMPSOutOfMemoryThroughTheAdapter`). Go test functions 215 → 217 with T4 alone.

Added to the fake ComfyUI: mode `OOMMPS` and the constant `comfytest.MPSOutOfMemoryText`. The mode `OOM` is unchanged.

Step 3, the `next_step` of `OUT_OF_MEMORY` (`internal/herr/herr.go:63`): "Call again. If it fails again, use a
smaller width and height." It names no thing that a Mac lacks. A Mac has unified memory, and a smaller
image also needs less of it. The text does not mislead, so no follow-up issue.

Step 4: the time limit stays 1800 s. `internal/settings/settings.go:158` and `internal/cli/run_illustration.go:155`
keep the default 1800 (the spec: the cold run on the Mac took 1033 s). No code changed.

## T1

Red (on `main` at `003b840`), `go test ./internal/backend -run ComfyCLI -v`:

```
testing: warning: no tests to run
PASS
ok  	github.com/hieplam/horus/internal/backend	2.434s [no tests to run]
```

Green, on the PC (Windows), after the rebase on `51c0fe3`:

```
$ go test -count=1 ./internal/backend ./internal/platform ./internal/settings ./internal/cli
ok  	github.com/hieplam/horus/internal/backend
ok  	github.com/hieplam/horus/internal/platform
ok  	github.com/hieplam/horus/internal/settings
ok  	github.com/hieplam/horus/internal/cli
$ go test -count=1 -v -run ComfyCLI ./internal/backend
--- PASS: TestComfyCLIIsTheBackendOfDarwin
--- PASS: TestComfyCLIInstallRunsTheCommandOfTheSpec
--- PASS: TestComfyCLIInstallFindsComfyInTheVenv
--- PASS: TestComfyCLIInstallSkipsWhenComfyUIIsInstalled
--- PASS: TestComfyCLIInstallNamesBothFixesWhenComfyIsMissing
--- PASS: TestComfyCLIInstallFailsWhenComfyFails
ok  	github.com/hieplam/horus/internal/backend
$ python tools/goal_check.py --phase 3 --no-ci --only tcc-folders --server-cmd <scratch>/horus.exe mcp
FAIL  tcc-folders - no passing test: TestInTCCFolder (not in internal/platform, T3); TestTCCCheckWarnsForEachPathInATCCFolder (not in internal/doctor, T3); TestDoctorWarnLinesKeepExitCodeZero (not in internal/cli, T3)
goal: 0 of 1 rows passed
$ python tests/mcp_contract.py --server-cmd <scratch>/horus.exe mcp
contract: 30 of 30 passed
$ python tests/workflow_parity.py --horus <scratch>/horus.exe
parity: 17 of 17 cases equal
```

The row `tcc-folders` names only the 3 tests of the WARN line (T3) as missing. The test of
the default folder (T1) passes (decision of T0: the row pins 4 tests). The 4 pinned tests
of T1 in `mac-backend` and the 2 in `mac-setup` are no longer named as missing.

Also green: `gofmt -l .` (no output), `go vet ./...` for windows, darwin/arm64 and linux,
`go test ./...` (every package `ok`), `python scripts/lint-public.py` (`0 finding(s)`).
`tests/` and the golden files did not change. The help of `install-comfyui` on Windows
still names `<home>/ComfyUI_windows_portable`. The CI job `go darwin/arm64` of the pull
request is the oracle of the macOS code: its result is in the pull request.

Stub check, each a scratch edit that was then reverted:

```
  # Install of the comfy-cli backend returns nil at once:
--- FAIL: TestComfyCLIInstallRunsTheCommandOfTheSpec (0.02s)
    comfycli_test.go:149: the fake comfy got "", want "--workspace C:\\...\\horus\\ComfyUI install --m-series --version 0.44.0\nstdin closed\n"
  # The default folder of macOS stays ComfyUI_windows_portable:
--- FAIL: TestDefaultComfyDirOfEachOS (0.00s)
    paths_test.go:59: darwin: defaultComfyDir = h\user\ComfyUI_windows_portable, want h\user\horus
    paths_test.go:59: linux: defaultComfyDir = h\user\ComfyUI_windows_portable, want h\user\horus
```

| Number | Before | After | Evidence |
|---|---|---|---|
| Commands that answer "not supported" on macOS | 3 | 2 (`start-comfyui`, `stop-comfyui`, until T2) | `newFor("darwin", ...)` gives `comfyCLI`, whose `Install` runs comfy (`TestComfyCLIIsTheBackendOfDarwin`, `TestComfyCLIInstallRunsTheCommandOfTheSpec`) |
| Default folders that fit macOS | 0 | 1 (`<home>/horus`) | `TestDefaultComfyDirOfEachOS`, `TestComfyDirDefaultIsTheFolderOfPlatform` |
| Go test functions (`go test -list . ./...`) | 217 (after T4) | 227 | 10 new tests, none removed |

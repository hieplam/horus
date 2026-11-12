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
| 2026-11-12 | A result of the doctor has a `Level` (`doctor.Pass`, `doctor.Warn`, `doctor.Fail`) in place of `OK bool`. The zero value is `Fail`. `doctor.Failed` counts only `Fail`. The handler maps the level to the word `PASS`, `WARN` or `FAIL`. | Step 1 of T3. A result that forgets its level does not pass. The service writes no output words. | T3 |
| 2026-11-12 | The test for "inside a TCC folder" is the pure function `inTCCFolder(goos, home, path)` of `internal/platform`. The exported `platform.InTCCFolder(home, path)` calls it with the OS of the build. The plan says "a pure function that takes the OS name". | The same pattern as `DefaultComfyDir` (decision of T1). No package outside `internal/platform` can name the OS (archtest). | T3 |
| 2026-11-12 | `doctor.Options` gets `InTCCFolder func(path string) (string, bool)`; nil means `platform.InTCCFolder` in `Home`. The doctor test gives it the macOS answer, so the test runs on Windows, Linux and macOS. | The check is a pure function, so it gets no interface. `Options.Getenv` has the same form. Without the field, the doctor test passes nothing on the PC. | T3 |
| 2026-11-12 | The TCC check gives one `WARN  macOS privacy folder - <what> <absolute path> is inside <folder> ...` line for each path, after the 7 checks. The paths: the repo root, the install folder, the models folder (`-models-dir`, else `HORUS_MODELS_DIR`), the text encoder folder (`-text-encoder-dir`, else `HORUS_TEXT_ENCODER_DIR`) and the output folder (`HORUS_OUTPUT_DIR`, else `<repo>/.scratch/outputs`). An empty folder is not checked. The same absolute path gives one line. | The spec names the 5 paths. The real part of `tcc-folders` needs the absolute path of the repo root in a WARN line (decision of T0), so the service calls `filepath.Abs`. An empty models folder is inside the install folder, which is checked. | T3 |
| 2026-11-12 | #82: without `-server-url`, the handler reads `settings.ComfyURL()` one time (`HORUS_COMFYUI_URL`, then `http://127.0.0.1:8188`) and gives the same address to the server check and to its line. The default of the flag is now empty. `TestDoctorModelCheckIsFullByDefault` now sets `HORUS_ROOT` to a temporary folder and `HORUS_COMFYUI_URL` to empty before it checks the default. | The behaviour of `start-comfyui` and `stop-comfyui` (decision 73 of Phase 2). Without the isolation, a `.env` or a variable of the machine changes the default that the test reads. Its assertion did not change. | T3 |
| 2026-11-12 | The setup test of `cmd/horus` drives the macOS backend through `backend.NewFor(goos, Place, Archive)`, a new export that gives the backend of the OS `goos` (`New` is `NewFor(runtime.GOOS, ...)`). `cmd/horus` gets the field `goos` in its test `seams`; empty means this OS. The test binary of `cmd/horus` is also the fake `comfy` (`TestMain`, `HORUS_FAKE_COMFY`). | `backend.New` picks by `runtime.GOOS`, and `cmd/horus` cannot name the unexported type `comfyCLI`. `NewFor` adds no behaviour and no setting: an environment variable would be a hidden option of the program. The test of T2 in `cmd/horus` (`TestGenerateStartsComfyUIThroughTheComfyCLIBackend`) can use the same export. | T3 |
| 2026-11-12 | Step 4 of T3 (no Windows path in the texts on macOS): the fix texts of the doctor name no Windows path (`ComfyUI/main.py` and `platform.ExeName("comfy")` follow the OS). The help line `-comfyui-dir ... then ~/ComfyUI_windows_portable` of `horus setup` (`internal/cli/setup.go:38`) and the words "the portable folder" stay. | T6 (#91) owns these help texts, and its Red needs the 2 lines of `setup.go` and `workflow.go` (decision of T1). | T3 |
| 2026-11-12 | The spec bullet "comfy-cli and comfy-mcp: the checks follow the new `.mcp.json`" is left to T5. | T5 makes the new `.mcp.json` and `horus comfy-mcp`. The plan steps of T3 do not name it, and the checks of today pass on the PC with the `.mcp.json` of today. | T3 |
| 2026-11-12 | The diff of `horus doctor` on the PC ran in the worktree of the task (no `.env`), with the binary of `origin/main` at `bd4409c` and the binary of the branch, both from `go build -o`. | The main checkout belongs to another session. The two runs use the same folder and the same flags. | T3 |
| 2026-11-12 | `Start` of the comfy-cli backend runs `comfy --workspace <dir>/ComfyUI --json launch --background -- --listen 127.0.0.1 --port <port>`. The global flag `--json` is the only flag that #53 did not have. `Stop` runs `comfy --workspace <dir>/ComfyUI stop` exactly as in #53. `--skip-prompt` is not added. The tests compare the full argument list. | The state file needs the process id, and only the envelope of `--json` holds it (`_emit_launch_success` of `comfy_cli/command/launch.py` in 1.25.0: `data.pid`). Without the flag, comfy-cli picks JSON or text by itself (a pipe gives JSON, `COMFY_OUTPUT=pretty` gives text), so horus could not rely on the format. comfy-cli asks nothing when its standard input is not a terminal (`tracking.prompt_tracking_consent`), and horus gives it a closed standard input, so `--skip-prompt` changes nothing. | T2 |
| 2026-11-12 | The state file keeps its fields (`tree`, `pid`, `url`, `started`). For comfy-cli, `tree` is `comfy-cli:<workspace>` and `pid` is the process id of the envelope (0 when comfy-cli gives none). | The Windows backend keeps its file byte for byte, and one `readState` serves both backends. `tree` names what `Stop` ends in both. | T2 |
| 2026-11-12 | `Stop` of the comfy-cli backend runs `comfy stop` only when the state file names the same address and `platform.Alive(pid)` is true, or the pid is 0. A dead pid runs no stop: the state file is deleted, and the text is "was not running any more", or the refusal when a server still answers. | The spec ("The stale state file"). The pid is the `comfy launch` process that runs ComfyUI, and it ends when ComfyUI ends (`launch_comfyui` exits with the server). A dead pid means that the background ComfyUI of comfy-cli is not the one that horus started. Limit: a pid of 0, or a pid that the OS gave to a new process after a restart, can still run a `comfy stop`. | T2 |
| 2026-11-12 | The shared code of both backends is in `internal/backend/started.go`: the state file, `alreadyUp`, `waitUp`, `stopStarted` (the refusal and the texts of `Stop`), `waitDown`, `answers` and `portOf`. Each backend gives only its own step: the portable build ends its tree, comfy-cli runs `comfy stop`. | Step 1 of T2: no copy between the backends. The texts of the portable build do not change (its tests pass without a change). | T2 |
| 2026-11-12 | `platform.Alive(pid)` is new. The 3 copies of `alive` in the tests of `internal/backend` and `internal/platform` are gone, and the tests call `platform.Alive`. | Whether a process runs is OS code, so it belongs in `internal/platform` (archtest). The backend needs it outside the tests now. | T2 |
| 2026-11-12 | The fake comfy is the package `internal/backend/comfyclitest` (layer `test` in `internal/archtest/layers.go`), not a function in a test file. `internal/backend` and `cmd/horus` both use it. On `launch` it starts a stand-in ComfyUI as a separate process, keeps its pid in a "background record" (one for each user, as comfy-cli does), and prints the envelope. On `stop` it ends that process. With `Proxy`, the stand-in sends each request to the fake ComfyUI of comfytest. | Step 2 and step 4 of T2 need the same fake in two packages. A copy is against the design rules. The package holds no code of comfy-cli. | T2 |
| 2026-11-12 | `TestGenerateStartsComfyUIThroughTheComfyCLIBackend` uses the test seam `seams.goos` of T3: `newImages(url)` is `imagesFor(url, seams{})`, and `imagesFor(url, seams{goos: "darwin"})` gives the image service on the comfy-cli backend (`backend.NewFor` of T3). The rule "goos empty = this OS" is one function, `backendFor`, for the setup service and the image service. No new export. | The pinned test of `cmd/horus` must pass on Windows too (the goal tool runs it on the PC), and `backend.New` gives the portable build there. The test goes through the MCP server, the image service, the ComfyUI adapter and the backend of the wiring. | T2 |
| 2026-11-12 | The fake comfy of T3 in `cmd/horus/doctor_setup_test.go` (its own `TestMain` and `fakeComfy`) is replaced by `comfyclitest`. `TestSetupYesWithTheFakeComfyThenASecondPlanHasZeroSteps` now wants the record line `... install --m-series --version 0.44.0` followed by `stdin closed`, and counts runs by the line `stdin ...`. | A package has one `TestMain`, and two fakes of comfy were a copy. The test asserts the same command and one run, and now also the closed standard input. | T2 |
| 2026-11-12 | The summaries of `start-comfyui` and `stop-comfyui` in the command list lose ", Windows only". | It is false after T2. No golden file holds it. | T2 |
| 2026-11-12 | `horus comfy-mcp` gives comfy-mcp the streams of horus as they are. `platform.Command` gets a field `Stdin`. cmd/horus passes `os.Stdin`, `os.Stdout` and `os.Stderr`, so Go passes the 3 handles to the child, and horus copies no byte. The service is `provision.RunComfyMCP` on its `Programs` interface. The handler is `cli.ComfyMCP`. | The plan step 1. A copy loop in horus is more code, and it can add a byte or hold one back. With the handles of the client, horus cannot write a byte between the bytes of comfy-mcp. | T5 |
| 2026-11-12 | How comfy-mcp ends: it reads the pipe of the MCP client itself, so it ends when the client closes that pipe or ends. Ctrl+C ends the context, and the context ends comfy-mcp (`exec.CommandContext`). No Job Object with "end on close". The limit that this design accepts: when a client kills `horus` and keeps the pipe open, comfy-mcp runs until the client closes the pipe. | The simplest design that leaves no process in the normal case. Measured on the PC (section T5): after the pipe closes, comfy-mcp and its 2 python processes are gone in less than 1.2 s, also after a kill of `horus`. A Job Object adds Windows code for a case in which the pipe closes anyway. | T5 |
| 2026-11-12 | `horus comfy-mcp` has the flag `-venv-path` (default `settings.ComfyMCPVenv()`). The arguments after the flags, or after `--`, go to comfy-mcp. | `TestHelpExitsZero` needs exit 0 for `-h` of each command, and `install-comfy-mcp` and `doctor` have the same flag. The test runs the command with a venv in a temporary folder. | T5 |
| 2026-11-12 | `horus comfy-mcp` sets `COMFY_BIN` only when `COMFY_BIN` is empty and the program `comfy` is next to comfy-mcp. Else it does not set it, and comfy-mcp runs the `comfy` of the PATH. | comfy-mcp 0.10.0 reads `os.environ.get("COMFY_BIN", "comfy")` (`comfy_mcp/server.py:100`). A `COMFY_BIN` that names a missing file breaks comfy-mcp, and the spec says "the comfy next to that program". An empty `COMFY_BIN` gives comfy-mcp an empty program name, so it counts as not set. | T5 |
| 2026-11-12 | Doctor check 4 (comfy-cli) without `COMFY_BIN` in the entry: the `comfy` next to comfy-mcp, else the `comfy` of the PATH, also for a comfy-mcp of the venv. Before, a comfy-mcp of the venv did not look at the PATH. The FAIL text says "not found next to comfy-mcp or on the PATH". | The check must find the program that `horus comfy-mcp` gives comfy-mcp (decision above). The spec: "comfy-mcp and comfy are in the venv or on the PATH". On the PC nothing changes: its venv holds `comfy.exe`. | T5 |
| 2026-11-12 | Doctor check 5 (`.mcp.json`): an entry whose command is `horus` must have the arguments `comfy-mcp`, else FAIL. A missing command `horus` gets the fix "put horus on the PATH". The PASS line adds the arguments (`command points at ...\horus.exe comfy-mcp`). An entry with the old Windows paths passes when the files exist. | `horus mcp` in the comfy-mcp entry starts the wrong server, and only the arguments show it. The old entry has no arguments, so its lines do not change (measured in section T5). | T5 |
| 2026-11-12 | The test of `cmd/horus` runs the test binary as `horus` (`HORUS_TEST_AS_HORUS`, which `TestMain` removes before `run`) and as the fake comfy-mcp (`HORUS_FAKE_COMFY_MCP=<exit code>`), in the one `TestMain` of `doctor_setup_test.go`, before `comfyclitest.Main`. | The pinned test must start `horus comfy-mcp` as a process: the exit code, and the standard output that holds only the bytes of comfy-mcp. Without the removal, the fake started by horus would run as horus too. | T5 |
| 2026-11-12 | The last Green command of the plan ("On the PC, with the new binary: `horus doctor` ... and `claude mcp list`") ran as: (a) `<scratch>/horus.exe doctor` from the worktree root, with `HORUS_COMFYUI_DIR=D:\ComfyUI_windows_portable` (the value of the `.env` of the main checkout; the worktree has no `.env`); (b) an MCP client of `tests/requirements.txt` that sends `initialize` and `tools/list` to `<scratch>/horus.exe comfy-mcp`. `claude mcp list` with the new entry needs the new `horus` on the PATH, so the verify task T8 (#108) runs it after `go install`. | A task must not replace the installed `horus.exe` (a running `horus mcp` holds it). (a) proves the doctor half of the row, (b) proves that the new entry starts the real comfy MCP server. The command `horus` of the entry resolves to the installed binary, which is enough for the check "the command exists". | T5 |
| 2026-11-12 | The Phase 1 row `no-python` fails on `main` before T5, for comments that name PowerShell in 4 Go files. Its `.mcp.json` part passes with the new file. Follow-up #117, outside the epic. | T5 changes none of the 4 files. No row of Phase 3 reads `no-python`. | T5 |
| 2026-11-12 | `docs/setup.md` and `docs/mcp.md` are not changed by T5. The lines of `docs/setup.md` that the new `.mcp.json` makes false (24-25, 156, 160, 253, 255) are named in a comment on #106 (T6). | T6 owns the one setup page and `horus comfy-mcp` in `docs/mcp.md` (plan, T6 steps 1 and 4). | T5 |
| 2026-11-12 | `TestComfyCLIIsTheBackendOfDarwin` no longer asserts that `Start` and `Stop` answer "not supported". It asserts that they do not: `Start` of an empty folder gives the error of a missing ComfyUI, and `Stop` says "Nothing to stop.". The address is a free port (it was 8190). | The behaviour changed by design (ratchet 2 → 0). The new tests of T2 prove `Start` and `Stop`. | T2 |
| 2026-11-12 | The third Red search of T6 (`git grep -n "not supported on this OS yet" -- docs/setup.md`) printed nothing on `main`, because the phrase breaks over lines 20 and 21. Line 20 of `main` names macOS ("On macOS and Linux they exit with "not supported on this"). After T6 the search prints one line, and that paragraph names Linux only. | The plan expected a line. The other two searches and the goal tool give the 4 lines of #91, so the Red holds. | T6 |
| 2026-11-12 | `docs/setup.md` swaps steps 1 and 2: step 1 is `horus install-comfy-mcp`, step 2 is `horus install-comfyui` and `horus start-comfyui`. Step 0 and steps 3 to 6 keep their numbers. | On macOS the backend finds `comfy` on the PATH, then in the venv of `horus install-comfy-mcp` (decision of the plan). So that venv must exist before the install of ComfyUI. `horus install-comfy-mcp` prints "docs/setup.md, step 5" for the agent step, and step 5 keeps its number. | T6 |
| 2026-11-12 | The help texts of `-comfyui-dir` use one form on every command: "the ComfyUI install folder" + `dirHelp` (`HORUS_COMFYUI_DIR, then <home>/...`). `start-comfyui` says "with ComfyUI/ in it" in place of "with ComfyUI/ and python_embeded/". On Windows the default still shows `<home>/ComfyUI_windows_portable`. On macOS it shows `<home>/horus` (`platform.DefaultComfyDir`, tested by `TestDefaultComfyDirOfEachOS`). The help of `-seven-zip` keeps `7z.exe` with "(Windows only)" on macOS. | Issue #91, fix 3. "portable" and `python_embeded` name the Windows build, which a Mac does not have. No golden file and no test holds these help texts (`git grep` of "portable folder" in `tests/` and `internal/`). No command changes its behaviour. | T6 |
| 2026-11-12 | T6 also changes `.env.example`, which is not in the plan's file list: the comment of `HORUS_COMFYUI_DIR` names the default of Windows and of macOS. | Step 3 of `docs/setup.md` sends the user into that file, and its old line named only the Windows folder. | T6 |
| 2026-11-12 | The macOS prerequisites name `brew install python@3.12 go uv`, the command of the Mac test run (#53). The page does not say that `uv` is needed. | comfy-cli 1.25.0 lists `uv>=0.11.15` in its `Requires-Dist` (the `METADATA` file of the venv on the PC), so the venv has a `uv`. Whether `comfy install` runs with only that `uv` is not proven. #53 is the one run that worked. T7 (#107) runs it. | T6 |
| 2026-11-12 | The times on the page: Windows 217 s to 470 s (`workflows/it-illustration/manifest.json`, `timing`), macOS 1033 s cold and 469 s warm (#53). | The measured numbers of each machine. | T6 |
| 2026-11-12 | The page says "Do not clone the repo into `~/horus`". A `horus doctor` check for that case is follow-up #119, outside the epic. | `~/horus` is the default install folder on macOS, and `horus` is the name of the repo, so `git clone` in the home folder makes the same folder. The page now names it, so no goal row waits for #119. | T6 |
| 2026-11-12 | T7 also took the line `HORUS_MODELS_DIR` out of `.env` (it named `~/horus-lab/ComfyUI/models`). The plan names only `HORUS_COMFYUI_DIR`. | With that line the plan finds the files of `~/horus-lab` and downloads nothing, against the decision "the real run downloads the model files again into `~/horus`". `~/horus-lab` was not changed. | T7 |
| 2026-11-12 | Step 4 ran `horus doctor -workflow it-illustration` two times: right after the setup (1 FAIL: `ComfyUI server - no answer`), then after `horus start-comfyui`. The Green holds for the second run. | `horus setup -yes` does not start ComfyUI, and the doctor fails when the server does not answer (decision of T0). The goal tool starts it the same way. | T7 |
| 2026-11-12 | The goal tool ran with `--real-out <scratch>/real-out`. The PNG is in `workflows/it-illustration/examples/mac-conveyor-belt.png`. | The default folder is inside the repo. | T7 |
| 2026-11-12 | The comfy of the backend was `~/.local/bin/comfy` (comfy-cli 1.25.0 of `uv tool`), found on the PATH before the venv. The doctor line `comfy-cli` names the `comfy` of the venv (also 1.25.0), the one that `horus comfy-mcp` gives comfy-mcp. The T6 sentence "the `comfy` of the venv installs ComfyUI with no `uv` from Homebrew" stays unproven: this Mac has both. | The rule of the plan (PATH, then the venv). The two programs are the same version, so no row changes. | T7 |
| 2026-11-12 | The 2 manual steps of this run: the token line of `.env` (already there) and `export PATH="$HOME/go/bin:$PATH"` in `~/.zshrc` (`docs/setup.md`, step 0). | The ratchet of `mac-setup` (3 → 2). `HORUS_COMFYUI_DIR` is no longer a step. | T7 |
| 2026-11-12 | No sentence of the page says that a macOS step was tested with the code of Phase 3. The page gives the numbers of #53 as "the Mac test run". | #53 ran comfy-cli by hand, not `horus install-comfyui`. The real run of `horus` on the Mac is T7 (#107). | T6 |
| 2026-11-12 | Issue #4 gets the decisions D42 to D53 of Phase 3, the ticked boxes of Phase 3 and a new "Resume here". The old body is saved before the edit, and the edit changes no other part of it (checked with `diff`). | Step 5 of T6. | T6 |
| 2026-11-12 | Step 1 of T8: a running `horus mcp` (process 23880) held `%USERPROFILE%\go\bin\horus.exe`. The old file became `horus.exe.old-20261112`, then `go install ./cmd/horus` ran. The renamed file stays. | The PC rule: rename, do not stop the process. | T8 |
| 2026-11-12 | The owner's ComfyUI on port 8188 did not run during T8 (no answer at 18:20, no listener at any later reading). With only the owner's setting (`HORUS_COMFYUI_DIR=D:\ComfyUI_windows_portable`), `horus doctor` gives 1 FAIL line: `ComfyUI server - no answer at http://127.0.0.1:8188`. So the real parts of `one-config` and `commands` ran with the owner's `HORUS_COMFYUI_DIR` and `HORUS_COMFYUI_URL=http://127.0.0.1:8189`, a second ComfyUI that `horus start-comfyui` started from the folder of junctions. | The task must not start the owner's ComfyUI. The doctor checks the server at `HORUS_COMFYUI_URL` (#82, part of the row `one-config`). Every other check reads the owner's settings as they are. The row still needs 0 FAIL lines from the same 7 checks, as in Phase 2 (T10, run 2) with the owner's ComfyUI. | T8 |
| 2026-11-12 | `HORUS_ROOT` of every real run is the worktree of T8 at `0f9192b` (`origin/main`), not the main checkout of Phase 2. | The main checkout belongs to another session, and its branch has the old `.mcp.json`. The rows must read the files of `main`. | T8 |
| 2026-11-12 | The GPU was quiet enough for the image run: the only GPU process was the client window of a game (`LeagueClientUxRender.exe`), at 87 MiB and 25 %. One run, and it passed. | The same state as the start of run 3 of Phase 2 (T10): the client window only, 483 MiB, 28 %. | T8 |
| 2026-11-12 | `claude mcp list` shows `horus` connected (the user-scope entry, the new binary) and `comfy-mcp: horus comfy-mcp - Pending approval`. T8 does not approve it. An MCP client started both entries of `.mcp.json` as written (`horus comfy-mcp`, `horus mcp`, found on the PATH) and got `initialize` and `tools/list` from both. | Claude Code asks a person to approve a project server one time for each folder. The approval changes the owner's Claude configuration, which a task does not do. The oracle of `one-config` is `horus doctor` and the Go tests. The MCP client proves that the entry starts the real comfy MCP server with the installed binary. | T8 |
| 2026-11-12 | The review of T7 (PR #121) finds no Green without literal output and no line that contradicts a goal row. One note: `gh pr view 121 --json mergedBy` names `hoacao33`, and the spec says that the owner merges by hand. Whether a person merged it with that account is not verified. No issue. | No goal row depends on who pressed merge. The coordinator gets the note. | T8 |
| 2026-11-12 | The PNG of the PC (seed 7) shows 4 envelopes. The PNG of Phase 2 (T10, seed 7) showed 5. No issue. | `same-behaviour` compares the answers of the contract and parity tests, not pixels. The cause is not verified. | T8 |
| 2026-11-12 | `changed_contract_files` of the goal tool counts a warning line of `git diff` (`LF will be replaced by CRLF`) as a changed file (stub probe B of `same-behaviour`: "2 changed contract files" for 1 file). No issue. | The warning comes only with a changed file, so it can add to a FAIL but never makes a PASS. No goal row is at risk. | T8 |
| 2026-11-12 | T9 ran in the main checkout on the branch of its pull request, at `866006e` (the merge of T8), with a clean tree. The goal tool ran with `--real-out <scratch>/real-out`. The PNG of T9 is not added to the repo. | The rows read the files of `main` (the branch adds only this log). `workflows/it-illustration/examples/mac-conveyor-belt.png` of T7 already shows the Mac image. | T9 |
| 2026-11-12 | The PNG of T9 (seed 7) shows the same picture as the PNG of T7 (seed 7), with the same gap: each box has its own belt. The two files are not equal byte for byte (SHA-256 `56ffb0e5...` and `dbd3bfd3...`). No issue. | `mac-image` asks for a PNG that shows the subject, not for equal bytes. The cause of the byte difference is not verified. | T9 |
| 2026-11-12 | The plan says that the pull request of T9 waits for the owner's merge. The owner asked the agent to merge it after CI is green. | The owner's request in the session of T9. | Owner |
| 2026-11-13 | The `Backend` interface now has 4 methods (`Install`, `Start`, `Stop`, `Installed`). D37 of #4 listed 5, with `Layout`. `Layout` is no method of it, and its type is `layout`, inside `internal/backend`. | Design rule 2 (deep modules): no caller outside the package used `Layout`. The agent acceptance (T10) found it. Issue #124. | Plan |

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

## T3

Red (on `main` at `bd4409c`, the test of #82 before the change), `go test -count=1 -run TestDoctorUsesHorusComfyUIURLWithoutTheFlag ./internal/cli`:

```
--- FAIL: TestDoctorUsesHorusComfyUIURLWithoutTheFlag (0.01s)
    doctor_test.go:205: HORUS_COMFYUI_URL=http://127.0.0.1:63682/: the doctor asks "http://127.0.0.1:8188", its line names "http://127.0.0.1:8188"; want http://127.0.0.1:63682
FAIL
FAIL	github.com/hieplam/horus/internal/cli	2.532s
```

Green, on the PC (Windows):

```
$ go test -count=1 ./internal/doctor ./internal/cli ./internal/platform ./cmd/horus
ok  	github.com/hieplam/horus/internal/doctor
ok  	github.com/hieplam/horus/internal/cli
ok  	github.com/hieplam/horus/internal/platform
ok  	github.com/hieplam/horus/cmd/horus
$ go test -count=1 -v -run '^(<the 5 pinned names of T3>)$' <their packages>
--- PASS: TestInTCCFolder (0.00s)
--- PASS: TestTCCCheckWarnsForEachPathInATCCFolder (0.03s)
--- PASS: TestDoctorWarnLinesKeepExitCodeZero (0.00s)
--- PASS: TestDoctorUsesHorusComfyUIURLWithoutTheFlag (0.01s)
--- PASS: TestSetupYesWithTheFakeComfyThenASecondPlanHasZeroSteps (0.30s)
$ python tools/goal_check.py --phase 3 --no-ci --only tcc-folders --server-cmd <scratch>/horus.exe mcp
PASS  tcc-folders - 4 pinned tests pass; real part not run (needs --real on macOS)
goal: 1 of 1 rows passed
$ python tools/goal_check.py --phase 3 --no-ci --only one-config mac-setup --server-cmd <scratch>/horus.exe mcp
PASS  mac-setup - 3 pinned tests pass; real part not run (needs --real on macOS)
FAIL  one-config - .mcp.json: 2 Windows-only paths, the entry comfy-mcp is not `horus comfy-mcp`; no passing test: TestComfyMCPPassesTheStreamsComfyBinAndTheExitCode (not in cmd/horus, T5)
goal: 1 of 2 rows passed
$ python tests/mcp_contract.py --server-cmd <scratch>/horus.exe mcp
contract: 30 of 30 passed
$ python tests/workflow_parity.py --horus <scratch>/horus.exe
parity: 17 of 17 cases equal
```

`one-config` no longer names `TestDoctorUsesHorusComfyUIURLWithoutTheFlag` as missing. Its other two parts belong to T5.

`horus doctor` on the PC: the binary of `origin/main` (`bd4409c`) and the binary of the branch ran with the same flags in the same folder.
The three runs were `horus doctor`, `horus doctor -workflow it-illustration -quick` and
`horus doctor -server-url http://127.0.0.1:8188`. The output and the exit code of each pair were saved to files, and
`diff` printed nothing for each of the 3 pairs. (`HORUS_COMFYUI_URL` is not set on the PC.)

Also green: `gofmt -l .` (no output), `go vet ./...` for windows, darwin/arm64 and linux, `go test ./...` (every
package `ok`, archtest included), `python scripts/lint-public.py` (`0 finding(s)`). `tests/`, the golden files and
`.mcp.json` did not change. The CI job `go darwin/arm64` of the pull request is the oracle of the macOS code: its
result is in the pull request.

Stub check, each a scratch edit that was then restored with `git checkout`:

```
  # the TCC function always answers "outside" (inTCCFolder starts with `if true { return "", false }`):
--- FAIL: TestInTCCFolder (0.00s)
    paths_test.go:95: darwin "/Users/you/Desktop": got "" false, want "\\Users\\you\\Desktop"
  # the doctor gives no TCC result (Check returns rs without tccWarnings):
--- FAIL: TestTCCCheckWarnsForEachPathInATCCFolder (0.03s)
    doctor_test.go:325: the doctor asked for 0 paths, want 5: []
    doctor_test.go:339: 7 results, 0 failed, want 11 and 0:
  # a WARN counts as failed (Failed counts `r.Level != Pass`):
--- FAIL: TestDoctorWarnLinesKeepExitCodeZero (0.00s)
    doctor_test.go:90: exit 1, want 0:
  # a fixed default address (the handler sets http://127.0.0.1:8188 in place of settings.ComfyURL()):
--- FAIL: TestDoctorUsesHorusComfyUIURLWithoutTheFlag (0.01s)
    doctor_test.go:236: HORUS_COMFYUI_URL=http://127.0.0.1:53494/: the doctor asks "http://127.0.0.1:8188", its line names "http://127.0.0.1:8188"; want http://127.0.0.1:53494
  # Install of the comfy-cli backend returns nil at once:
--- FAIL: TestSetupYesWithTheFakeComfyThenASecondPlanHasZeroSteps (0.04s)
    doctor_setup_test.go:240: -yes: exit 1:
        Setup of workflow 'demo':
        MISSING   ComfyUI in <temp>\comfy
```

| Number | Before | After | Evidence |
|---|---|---|---|
| Levels of a doctor line | 2 (`PASS`, `FAIL`) | 3 (`PASS`, `WARN`, `FAIL`) | `doctor.Level`, `TestDoctorWarnLinesKeepExitCodeZero` |
| Open issue #82 | open | closed by the pull request | `TestDoctorUsesHorusComfyUIURLWithoutTheFlag` |
| Pinned tests of T3 that pass | 0 of 5 | 5 of 5 | the `-v` run above |
| Go test functions (`go test -list . ./...`) | 227 (after T1) | 233 | 6 new tests, none removed |

## T2

Red (on `main` at `bd4409c`), `go test ./internal/backend -run 'ComfyCLI.*(Start|Stop)' -v`:

```
testing: warning: no tests to run
PASS
ok  	github.com/hieplam/horus/internal/backend	2.419s [no tests to run]
```

Green, on the PC (Windows):

```
$ go test -count=1 ./internal/backend ./cmd/horus
ok  	github.com/hieplam/horus/internal/backend	34.605s
ok  	github.com/hieplam/horus/cmd/horus	2.256s
$ go test -count=1 -v -run 'ComfyCLI.*(Start|Stop)' ./internal/backend
--- PASS: TestComfyCLIStartRunsLaunchAndTheServerAnswers
--- PASS: TestComfyCLIStartFailsWhenComfyUIIsNotInstalled
--- PASS: TestComfyCLIStartFailsWhenLaunchFails
--- PASS: TestComfyCLIStopRunsStopAndTheServerEnds
--- PASS: TestComfyCLIStopRefusesAComfyUIThatHorusDidNotStart
--- PASS: TestComfyCLIStopWithAStaleStateRunsNoStop
--- PASS: TestComfyCLIStartAndStopWithoutAProcessID
ok  	github.com/hieplam/horus/internal/backend	14.365s
$ go test -count=1 -v -run TestGenerateStartsComfyUIThroughTheComfyCLIBackend ./cmd/horus
--- PASS: TestGenerateStartsComfyUIThroughTheComfyCLIBackend
ok  	github.com/hieplam/horus/cmd/horus	0.589s
$ python tools/goal_check.py --phase 3 --no-ci --only mac-backend --server-cmd <scratch>/horus.exe mcp
PASS  mac-backend - 10 pinned tests pass; CI part skipped (--no-ci); real part not run (needs --real on macOS)
goal: 1 of 1 rows passed
$ python tests/mcp_contract.py --server-cmd <scratch>/horus.exe mcp
contract: 30 of 30 passed
$ python tests/workflow_parity.py --horus <scratch>/horus.exe
parity: 17 of 17 cases equal
```

Also green: `gofmt -l .` (no output), `go vet ./...` for windows, darwin/arm64 and linux,
`go test ./...` (every package `ok`), `python scripts/lint-public.py` (`0 finding(s)`).
`tests/` and the golden files did not change. The tests of the portable backend
(`start_test.go`, `stop_test.go`) pass with no change of an assertion: only their helper
`alive` became `platform.Alive`. The CI job `go darwin/arm64` of the pull request is the
oracle of the macOS code: its result is in the pull request.

The commands that horus sends, as the fake comfy records them (the tests compare the full lines):

```
--workspace <dir>/ComfyUI --json launch --background -- --listen 127.0.0.1 --port <port>
--workspace <dir>/ComfyUI stop
```

Stub check, each a scratch edit that was then reverted with `git checkout`:

```
  # Start of the comfy-cli backend returns nil at once:
--- FAIL: TestComfyCLIStartRunsLaunchAndTheServerAnswers (0.02s)
    comfycli_test.go:239: the fake comfy got "", want "--workspace C:\\...\\horus\\ComfyUI --json launch --background -- --listen 127.0.0.1 --port 63182\nstdin closed\n"
    comfycli_test.go:242: the server does not answer after Start
--- FAIL: TestGenerateStartsComfyUIThroughTheComfyCLIBackend (0.03s)
    comfycli_test.go:75: got map[error:map[action_by:human code:BACKEND_UNAVAILABLE ...
  # Stop runs comfy stop first, every time:
--- FAIL: TestComfyCLIStopRefusesAComfyUIThatHorusDidNotStart (1.55s)
    comfycli_test.go:351: Stop without a state file = <nil>, want the refusal
    comfycli_test.go:360: the fake comfy got a stop: "--workspace C:\\...\\horus\\ComfyUI launch --background -- --listen 127.0.0.1 --port 63205\nstdin closed\n-...
    comfycli_test.go:363: the person's ComfyUI no longer answers
--- FAIL: TestComfyCLIStopWithAStaleStateRunsNoStop (1.58s)
    comfycli_test.go:400: the fake comfy got a stop: "--version\nstdin closed\n--workspace C:\\...\\horus\\ComfyUI stop\nstdin closed\n...
```

| Number | Before | After | Evidence |
|---|---|---|---|
| Commands that answer "not supported" on macOS | 2 (`start-comfyui`, `stop-comfyui`) | 0 | `comfyCLI.Start` runs `comfy launch` and `comfyCLI.Stop` runs `comfy stop` (the 5 pinned tests of T2 in `internal/backend`). Linux still answers "not supported" (`TestOtherOSAnswersNotSupported`). |
| Go test functions (`go test -list . ./...`) | 233 (after T1 and T3) | 243 | 10 new tests, none removed |

What only the Mac can confirm (task T7), because the tests use the fake comfy:

1. `comfy --json launch --background` of comfy-cli 1.25.0 prints one envelope line on the standard output, with `data.pid`.
2. That pid is the `comfy launch` process that runs ComfyUI, and it ends after `comfy stop`, within the 15 s of the wait of `Stop`.
3. The background ComfyUI keeps running after `horus start-comfyui` ends, also when the terminal closes.
4. The global flag `--json` before `launch` changes nothing else of the launch.
5. `comfy stop` exits 0 after it stopped the background ComfyUI.

## T5

Red, on `main` at `250302f`, with the binary of `main` from `go build -o <scratch>/red/horus.exe`:

```
$ git grep -n "USERPROFILE" -- .mcp.json
.mcp.json:5:      "command": "${USERPROFILE}\\.venvs\\comfy-mcp\\Scripts\\comfy-mcp.exe",
.mcp.json:8:        "COMFY_BIN": "${USERPROFILE}\\.venvs\\comfy-mcp\\Scripts\\comfy.exe",
$ <scratch>/red/horus.exe comfy-mcp
horus: unknown command "comfy-mcp"
exit 2
```

Green, on the PC (Windows), after the rebase on `88ea73a` (T2):

```
$ git grep -n "USERPROFILE" -- .mcp.json
(no output, exit 1)
$ go test -count=1 ./internal/cli ./internal/provision ./internal/doctor ./cmd/horus
ok  	github.com/hieplam/horus/internal/cli
ok  	github.com/hieplam/horus/internal/provision
ok  	github.com/hieplam/horus/internal/doctor
ok  	github.com/hieplam/horus/cmd/horus
$ python tools/goal_check.py --phase 3 --no-ci --only one-config --server-cmd <scratch>/t5/horus.exe mcp
PASS  one-config - .mcp.json ok; 2 pinned tests pass; real part not run (needs --real on Windows and on macOS)
goal: 1 of 1 rows passed
$ python tests/mcp_contract.py --server-cmd <scratch>/t5/horus.exe mcp
contract: 30 of 30 passed
$ python tests/workflow_parity.py --horus <scratch>/t5/horus.exe
parity: 17 of 17 cases equal
$ python scripts/lint-public.py
lint-public: 0 finding(s)
```

The Phase 1 row that reads `.mcp.json` (`no-python`) checks that the horus entry is `horus mcp`.
That part holds. The row fails on `main` for another reason (follow-up #117, decision above):

```
$ python tools/goal_check.py --no-ci --only no-python
FAIL  no-python - 0 lines left; internal/catalog/workflowdir.go contains powershell; internal/cli/workflow.go contains powershell; internal/cli/workflow_test.go contains powershell; internal/provision/provision_test.go contains powershell
```

Also green: `gofmt -l .` (no output), `go vet ./...` for windows, darwin/arm64 and linux, `go test ./...`
(every package `ok`, archtest included). `tests/` and the golden files did not change. The CI job
`go darwin/arm64` of the pull request is the oracle of the macOS code: its result is in the pull request.

The last Green command, on the PC with the new binary (decision above):

(a) `<scratch>/t5/horus.exe doctor` from the worktree root, with `HORUS_COMFYUI_DIR=D:\ComfyUI_windows_portable`:

```
PASS  ComfyUI folder - D:\ComfyUI_windows_portable
PASS  ComfyUI server - answers at http://127.0.0.1:8188 (ComfyUI 0.44.0)
PASS  comfy-mcp environment - %USERPROFILE%\.venvs\comfy-mcp\Scripts\comfy-mcp.exe
PASS  comfy-cli - %USERPROFILE%\.venvs\comfy-mcp\Scripts\comfy.exe
PASS  .mcp.json - command points at %USERPROFILE%\go\bin\horus.exe comfy-mcp
PASS  horus on PATH - %USERPROFILE%\go\bin\horus.exe
PASS  .mcp.json horus - command points at %USERPROFILE%\go\bin\horus.exe
All checks passed.
exit 0
```

The same binary on the old `.mcp.json` of `main` (a scratch folder with that file and `HORUS_ROOT` set to it):

```
PASS  .mcp.json - command points at %USERPROFILE%\.venvs\comfy-mcp\Scripts\comfy-mcp.exe, COMFY_BIN at %USERPROFILE%\.venvs\comfy-mcp\Scripts\comfy.exe
All checks passed.
exit 0
```

(b) An MCP client (`mcp` of `tests/requirements.txt`) starts `<scratch>/t5/horus.exe comfy-mcp` and sends
only `initialize` and `tools/list`:

```
initialize: server comfy-mcp , protocol 2025-11-25
tools/list: 39 tools
first 5: server_info, auth_status, auth_login, run_workflow, generate_image
```

The end of the child, on the PC. A script starts `horus comfy-mcp`, sends `initialize`, reads the answer,
and lists the processes under `horus` (`Get-CimInstance Win32_Process`):

```
A: the client closes the standard input
  horus pid 23056, answer from server 'comfy-mcp', children: ['comfy-mcp.exe', 'python.exe', 'python.exe']
  horus exit code 0; children left after the exit: []; all gone after 1.2 s
B: the client kills horus and keeps the pipe open
  horus pid 7308, answer from server 'comfy-mcp', children: ['comfy-mcp.exe', 'python.exe', 'python.exe']
  3 s after the kill: children alive: ['comfy-mcp.exe', 'python.exe', 'python.exe']
  after the client closes the pipe: all gone after 1.2 s
```

B is the limit of the design (decision above). That Claude Code closes the pipe of a server that it
kills is not verified here.

(c) `claude mcp list` with the new entry needs the new `horus` on the PATH. The verify task T8 (#108)
runs it after `go install` (decision above).

Stub check, each a scratch edit that was then restored with `git checkout`:

```
  # RunComfyMCP returns 0 at once and starts nothing (`if true { return 0, nil }`):
--- FAIL: TestComfyMCPPassesTheStreamsComfyBinAndTheExitCode (0.12s)
    comfymcp_test.go:88: exit 0, want 7 (the exit code of comfy-mcp); stderr:
    comfymcp_test.go:91: stdout has 0 bytes, want the 262201 bytes of the input; the first bytes: ""
    comfymcp_test.go:95: stderr lacks "COMFY_BIN=C:\\...\\venv\\Scripts\\comfy.exe\n":
    comfymcp_test.go:102: COMFY_BIN set: exit 0, stdout "", stderr:
    comfymcp_test.go:110: no comfy-mcp: exit 0, stdout "", stderr "", want exit 1 and "horus comfy-mcp: comfy-mcp was not found in ..."
  # the doctor does not read the comfy-mcp entry (check 5 always passes):
--- FAIL: TestTheMcpJSONOfTheRepoPasses (0.01s)
--- FAIL: TestMcpJSONExpandsVariablesAndChecksComfyBin (0.01s)
--- FAIL: TestOneMissingPieceGivesOneFailLine (0.06s)
    --- FAIL: TestOneMissingPieceGivesOneFailLine/entry_of_.mcp.json_points_at_nothing (0.01s)
```

| Number | Before | After | Evidence |
|---|---|---|---|
| Windows-only paths in `.mcp.json` | 2 | 0 | the search above, and `one-config` of the goal tool |
| Pinned tests of `one-config` that pass | 1 of 2 | 2 of 2 | `2 pinned tests pass` |
| Go test functions (`go test -list . ./...`) | 243 (after T2) | 249 | 6 new tests, none removed |

## T6

Red, on `main` at `b7b71cf`, before any change:

```
$ git grep -n "Two commands are\|does steps 1 and 4" -- docs/setup.md
docs/setup.md:18:**macOS and Linux.** `horus` builds and runs there too. Two commands are
docs/setup.md:112:**The short way.** `horus setup it-illustration` does steps 1 and 4 in one
$ git grep -n "then ~/ComfyUI_windows_portable" -- internal/cli
internal/cli/setup.go:38:	fs.StringVar(&place.ComfyUIDir, "comfyui-dir", "", "the ComfyUI portable folder (default: HORUS_COMFYUI_DIR, then ~/ComfyUI_windows_portable)")
internal/cli/workflow.go:89:	fs.StringVar(&place.ComfyUIDir, "comfyui-dir", "", "the ComfyUI portable folder (default: HORUS_COMFYUI_DIR, then ~/ComfyUI_windows_portable)")
$ git grep -n "not supported on this OS yet" -- docs/setup.md
(no output: the phrase breaks over lines 20 and 21; line 20 names macOS, decision above)
$ python tools/goal_check.py --phase 3 --no-ci --only one-setup-doc
FAIL  one-setup-doc - 4 lines that the commands contradict; docs/setup.md:18, docs/setup.md:112, internal/cli/setup.go:38, internal/cli/workflow.go:89, docs/setup.md has no macOS section
goal: 0 of 1 rows passed
exit 1
```

Green, on the PC (Windows), on the branch of T6 (based on `b7b71cf`), with a binary of the branch
from `go build -o <scratch>/t6/horus.exe ./cmd/horus` and the test venv of `tests/requirements.txt`:

```
$ git grep -n "Two commands are\|does steps 1 and 4" -- docs/setup.md
(no output, exit 1)
$ git grep -n "then ~/ComfyUI_windows_portable" -- internal/cli
(no output, exit 1)
$ git grep -n "not supported on this OS yet" -- docs/setup.md
docs/setup.md:22:commands exit with "not supported on this OS yet" there: `horus
$ python tools/goal_check.py --phase 3 --no-ci --only one-setup-doc
PASS  one-setup-doc - 0 lines that the commands contradict; docs/setup.md has a macOS section
goal: 1 of 1 rows passed
$ python scripts/lint-public.py
lint-public: 0 finding(s)
$ gofmt -l .
(no output)
$ go test -count=1 ./...
ok  	github.com/hieplam/horus/cmd/horus
ok  	github.com/hieplam/horus/internal/archtest
ok  	github.com/hieplam/horus/internal/backend
?   	github.com/hieplam/horus/internal/backend/comfyclitest	[no test files]
ok  	github.com/hieplam/horus/internal/catalog
ok  	github.com/hieplam/horus/internal/cli
ok  	github.com/hieplam/horus/internal/comfy
?   	github.com/hieplam/horus/internal/comfy/comfytest	[no test files]
ok  	github.com/hieplam/horus/internal/devtools
ok  	github.com/hieplam/horus/internal/doctor
ok  	github.com/hieplam/horus/internal/download
ok  	github.com/hieplam/horus/internal/generate
ok  	github.com/hieplam/horus/internal/graph
ok  	github.com/hieplam/horus/internal/herr
ok  	github.com/hieplam/horus/internal/mcpserver
ok  	github.com/hieplam/horus/internal/modelfiles
ok  	github.com/hieplam/horus/internal/platform
ok  	github.com/hieplam/horus/internal/provision
ok  	github.com/hieplam/horus/internal/settings
$ python tests/mcp_contract.py --server-cmd <scratch>/t6/horus.exe mcp
contract: 30 of 30 passed
$ python tests/workflow_parity.py --horus <scratch>/t6/horus.exe
parity: 17 of 17 cases equal
$ GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go vet ./internal/cli/
(no output, exit 0)
```

The help line of `-comfyui-dir` after T6, on Windows. `horus setup -h`, `horus workflow -h`,
`horus doctor -h`, `horus stop-comfyui -h` and `horus sync-workflows -h` print the same words:

```
  -comfyui-dir string
    	the ComfyUI install folder (default: HORUS_COMFYUI_DIR, then <home>/ComfyUI_windows_portable)
```

On macOS the same line ends with `then <home>/horus)`. This PC cannot run the macOS binary. The
text comes from `platform.DefaultComfyDir("<home>")`, and its test `TestDefaultComfyDirOfEachOS`
checks `<home>/horus` for `darwin`. The CI job `go darwin/arm64` runs the tests of `internal/cli`
on macOS.

Each sentence of the page about a command was checked against `horus -h`, `<command> -h` of the
binary of `b7b71cf`, and the code:

- `internal/backend/comfycli.go`: the install, start and stop commands, the log path, the text "comfy-cli (the program comfy) was not found".
- `internal/cli/comfyui.go`: the archive flags exit 2 on macOS.
- `internal/platform/platform.go`: the default folder, the TCC folders, `python3.12`.
- `internal/doctor/doctor.go`: the WARN line.
- `internal/provision/comfymcp.go`: `horus comfy-mcp` (the venv, then the PATH, `PYTHONUTF8`, `COMFY_BIN`). The binary printed `horus comfy-mcp: comfy-mcp was not found in <scratch>\novenv\Scripts or on the PATH. Fix: horus install-comfy-mcp` for an empty `-venv-path` folder, exit 1.

The sentences about macOS that only a real Mac confirms are for T7 (#107):

- `horus install-comfyui` installs ComfyUI in `~/horus/ComfyUI` through comfy-cli, in about 2 minutes and 2.0 GB.
- `horus start-comfyui` and `horus stop-comfyui` start and stop it, and the log is `ComfyUI/user/comfyui_8188.log`.
- `horus install-comfyui` finds `comfy` in the venv of `horus install-comfy-mcp`, and that `comfy` installs ComfyUI with no `uv` from Homebrew.
- `horus doctor` prints the WARN line for a clone in `~/Desktop`.
- The times: 1033 s cold and 469 s warm, and 5 minutes for the 11 GB download.

| Number | Before | After | Evidence |
|---|---|---|---|
| Sentences of the setup docs and help texts that the commands contradict (#91) | 4 | 0 | the two searches and `one-setup-doc` above |
| macOS sections in `docs/setup.md` | 0 | 2 (`Windows and macOS: what differs`, `The macOS privacy folders`) | `one-setup-doc` above |
| Lines of `docs/setup.md` that the new `.mcp.json` made false (comment of T5 on #106) | 5 | 0 | the lines 24-25, 156, 160, 253 and 255 of `main` are gone or rewritten for `horus comfy-mcp` |
| Go test functions (`go test -list . ./...`) | 249 | 249 | T6 changes no test (`same-behaviour`: 249 Go test functions) |

## T7

On the Mac (Apple M4, 16 GB, macOS 26.5.1), on `main` at `77374a8`, `horus` from `go install
./cmd/horus`, the goal tool in a Python 3.12 venv of `tests/requirements.txt`. Paths: `~` is the
home folder, `<scratch>` a scratch folder outside the repo. `~/horus` did not exist before step 1.

Red, before step 2 (so also before step 3), after `~/go/bin` was put on the PATH:

```
$ horus doctor
FAIL  ComfyUI folder - ComfyUI/main.py not found in ~/horus. Fix: horus install-comfyui, or pass -comfyui-dir.
FAIL  ComfyUI server - no answer at http://127.0.0.1:8188. Fix: horus start-comfyui
FAIL  comfy-mcp environment - comfy-mcp not found in ~/.venvs/comfy-mcp/bin or on the PATH. Fix: horus install-comfy-mcp
FAIL  comfy-cli - comfy-mcp is missing, so comfy-cli cannot be found. Fix: horus install-comfy-mcp
PASS  .mcp.json - command points at ~/go/bin/horus comfy-mcp
PASS  horus on PATH - ~/go/bin/horus
PASS  .mcp.json horus - command points at ~/go/bin/horus
WARN  macOS privacy folder - the repo root ~/Desktop/Private/horus is inside ~/Desktop, a folder that macOS protects. ComfyUI and comfy-mcp can fail to read it. Fix: move it out of ~/Desktop, or grant access in System Settings > Privacy & Security.
WARN  macOS privacy folder - the output folder ~/Desktop/Private/horus/.scratch/outputs is inside ~/Desktop, a folder that macOS protects. ComfyUI and comfy-mcp can fail to read it. Fix: move it out of ~/Desktop, or grant access in System Settings > Privacy & Security.
4 check(s) failed.
exit 1
```

Step 2: `horus install-comfy-mcp` (18 s): `OK: comfy-mcp is installed in ~/.venvs/comfy-mcp`.
The backend used `~/.local/bin/comfy` (comfy-cli 1.25.0 of `uv tool`, on the PATH), proven by the
process list after the start:

```
92657 ~/.local/share/uv/tools/comfy-cli/bin/python ~/.local/bin/comfy --workspace=~/horus/ComfyUI launch -- --listen 127.0.0.1 --port 8188 --enable-manager
92659 ~/horus/ComfyUI/.venv/bin/python main.py --listen 127.0.0.1 --port 8188 --enable-manager --enable-manager
```

Step 3, the plan:

```
$ horus setup it-illustration
Setup of workflow 'it-illustration':
MISSING   ComfyUI in ~/horus
MISSING   diffusion flux3-image-klein-int8-convrot.safetensors (5.83 GB) ~/horus/ComfyUI/models/diffusion_models/flux3-image-klein-int8-convrot.safetensors
MISSING   text_encoder qwen3vl_4b_int8_convrot.safetensors (4.86 GB) ~/horus/ComfyUI/models/text_encoders/qwen3vl_4b_int8_convrot.safetensors
MISSING   vae flux2-vae.safetensors (0.34 GB) ~/horus/ComfyUI/models/vae/flux2-vae.safetensors
Plan: 4 step(s):
  1. install ComfyUI in ~/horus
  2. download diffusion flux3-image-klein-int8-convrot.safetensors (5.83 GB) to ~/horus/ComfyUI/models/diffusion_models/flux3-image-klein-int8-convrot.safetensors
  3. download text_encoder qwen3vl_4b_int8_convrot.safetensors (4.86 GB) to ~/horus/ComfyUI/models/text_encoders/qwen3vl_4b_int8_convrot.safetensors
  4. download vae flux2-vae.safetensors (0.34 GB) to ~/horus/ComfyUI/models/vae/flux2-vae.safetensors
Nothing changed. Run horus setup it-illustration -yes to run the plan.
```

Step 3, `horus setup it-illustration -yes`: from 10:36:33Z to 10:43:05Z, `real 391.87` s for the
install of ComfyUI and the 11 GB download. 331 lines; the lines before the end are the pip log of
comfy-cli (`comfy install --m-series --version 0.44.0`, torch nightly for MPS). The end:

```
ComfyUI is installed at: ~/horus/ComfyUI
OK: ComfyUI v0.44.0 is installed in ~/horus
Next: horus start-comfyui -comfyui-dir '~/horus'
License: FLUX Kommunity License (gated: accept it on the repo page first)
Installing 3 model file(s) for workflow 'it-illustration'.
-- diffusion -> ~/horus/ComfyUI/models/diffusion_models/flux3-image-klein-int8-convrot.safetensors
Downloading flux3-image-klein-int8-convrot.safetensors (5.43 GiB) ...
Checking SHA-256 of flux3-image-klein-int8-convrot.safetensors (this takes a while for big files) ...
OK flux3-image-klein-int8-convrot.safetensors verified: 5827874779 bytes, SHA-256 847d8933548afd93df4af2cac2498e70891bf28d9dae06d695f536992e168fe2
-- text_encoder -> ~/horus/ComfyUI/models/text_encoders/qwen3vl_4b_int8_convrot.safetensors
Downloading qwen3vl_4b_int8_convrot.safetensors (4.52 GiB) ...
Checking SHA-256 of qwen3vl_4b_int8_convrot.safetensors (this takes a while for big files) ...
OK qwen3vl_4b_int8_convrot.safetensors verified: 4858559240 bytes, SHA-256 32de113cbf8282b02cbff0794b637b9d6676da09314a1db5a30bcea9bf4ee01c
-- vae -> ~/horus/ComfyUI/models/vae/flux2-vae.safetensors
Downloading flux2-vae.safetensors (0.31 GiB) ...
Checking SHA-256 of flux2-vae.safetensors (this takes a while for big files) ...
OK flux2-vae.safetensors verified: 336211292 bytes, SHA-256 868fe7b343cc8f3a19dbcfcafbc3d5f888802be3f89bd81b65b3621a066ce8f3

Checking workflow 'it-illustration': 3 model file(s), size only.
OK        diffusion ~/horus/ComfyUI/models/diffusion_models/flux3-image-klein-int8-convrot.safetensors (5827874779 bytes)
OK        text_encoder ~/horus/ComfyUI/models/text_encoders/qwen3vl_4b_int8_convrot.safetensors (4858559240 bytes)
OK        vae ~/horus/ComfyUI/models/vae/flux2-vae.safetensors (336211292 bytes)
OK        models.json lists all 3 model file(s) that workflow.json loads
SKIP      server checks: no answer at http://127.0.0.1:8188 (start ComfyUI with horus start-comfyui)
OK: this machine is set up for workflow 'it-illustration'.
real 391.87
user 53.96
sys 46.33
exit=0
```

`~/horus` after the setup: 12 GB, of which 10 GB are model files.

Step 4 (Green), after `horus start-comfyui` (47 s, `OK: ComfyUI answers at http://127.0.0.1:8188`):

```
$ horus doctor -workflow it-illustration
PASS  ComfyUI folder - ~/horus
PASS  ComfyUI server - answers at http://127.0.0.1:8188 (ComfyUI 0.44.0)
PASS  comfy-mcp environment - ~/.venvs/comfy-mcp/bin/comfy-mcp
PASS  comfy-cli - ~/.venvs/comfy-mcp/bin/comfy
PASS  .mcp.json - command points at ~/go/bin/horus comfy-mcp
PASS  horus on PATH - ~/go/bin/horus
PASS  .mcp.json horus - command points at ~/go/bin/horus
WARN  macOS privacy folder - the repo root ~/Desktop/Private/horus is inside ~/Desktop, a folder that macOS protects. ComfyUI and comfy-mcp can fail to read it. Fix: move it out of ~/Desktop, or grant access in System Settings > Privacy & Security.
WARN  macOS privacy folder - the output folder ~/Desktop/Private/horus/.scratch/outputs is inside ~/Desktop, a folder that macOS protects. ComfyUI and comfy-mcp can fail to read it. Fix: move it out of ~/Desktop, or grant access in System Settings > Privacy & Security.
      Checking workflow 'it-illustration': 3 model file(s).
      OK        diffusion ~/horus/ComfyUI/models/diffusion_models/flux3-image-klein-int8-convrot.safetensors (5827874779 bytes, SHA-256 verified)
      OK        text_encoder ~/horus/ComfyUI/models/text_encoders/qwen3vl_4b_int8_convrot.safetensors (4858559240 bytes, SHA-256 verified)
      OK        vae ~/horus/ComfyUI/models/vae/flux2-vae.safetensors (336211292 bytes, SHA-256 verified)
      OK        models.json lists all 3 model file(s) that workflow.json loads
      OK        the server has all 10 node classes of workflow.json
      OK        the server lists flux3-image-klein-int8-convrot.safetensors
      OK        the server lists qwen3vl_4b_int8_convrot.safetensors
      OK        the server lists flux2-vae.safetensors
      OK: workflow 'it-illustration' is ready.
PASS  models for 'it-illustration' - all files present and verified (SHA-256 checked)
All checks passed.
exit=0
$ horus setup it-illustration
Setup of workflow 'it-illustration':
PRESENT   ComfyUI in ~/horus
PRESENT   diffusion flux3-image-klein-int8-convrot.safetensors (5.83 GB) ~/horus/ComfyUI/models/diffusion_models/flux3-image-klein-int8-convrot.safetensors
PRESENT   text_encoder qwen3vl_4b_int8_convrot.safetensors (4.86 GB) ~/horus/ComfyUI/models/text_encoders/qwen3vl_4b_int8_convrot.safetensors
PRESENT   vae flux2-vae.safetensors (0.34 GB) ~/horus/ComfyUI/models/vae/flux2-vae.safetensors
Plan: 0 steps. Nothing is missing.
```

Step 5 (Green), from 10:44:28Z to 10:52:24Z:

```
$ python tools/goal_check.py --phase 3 --real --real-out <scratch>/real-out --only mac-backend mac-setup mac-image tcc-folders one-config
PASS  mac-backend - 10 pinned tests pass; CI: 1 jobs green (go darwin/arm64); real: start, stop, and a refusal of a ComfyUI started by hand
PASS  mac-setup - 3 pinned tests pass; real: horus doctor: 0 FAIL lines; Plan: 0 steps; no HORUS_COMFYUI_DIR
PASS  mac-image - 1536x1024 <scratch>/real-out/real-image-seed7-20261112-174506.png (seed 7, 434 s); started_backend: true
PASS  tcc-folders - 4 pinned tests pass; real: horus doctor: exit 0, 0 FAIL lines, a WARN line names ~/Desktop/Private/horus
PASS  one-config - .mcp.json ok; 2 pinned tests pass; real: horus doctor on darwin: 0 FAIL lines
goal: 5 of 5 rows passed
real 476.10
user 9.28
sys 6.54
exit=0
```

The PNG is `workflows/it-illustration/examples/mac-conveyor-belt.png` (1536x1024). Judged against
the subject: a mint-green server box on the left and a sky-blue one on the right, sun-yellow
envelopes, lilac conveyor belts. One gap: each server has its own belt, the subject asks for one
belt from the left box to the right box. Small marks on the boxes look like text. Both are limits
of the model (the same kind as in the workflow README), not of horus.

Step 6 (Green):

```
$ horus stop-comfyui
Background ComfyUI is stopped. (127.0.0.1:8188)
{"schema": "envelope/1", "type": "envelope", "ok": true, "command": "stop", "version": "1.25.0", "where": null, "data": {"stopped": true, "host": "127.0.0.1", "port": 8188, "pid": 93437}, "error": null, "changed": true}
OK: stopped the ComfyUI that horus started at http://127.0.0.1:8188 (process 93437).
exit 0
$ comfy --workspace ~/horus/ComfyUI launch --background -- --listen 127.0.0.1 --port 8188
ComfyUI is successfully launched in the background.
To see the GUI go to: http://127.0.0.1:8188
{"schema": "envelope/1", "type": "envelope", "ok": true, "command": "launch", "version": "1.25.0", "where": null, "data": {"background": true, "listen": "127.0.0.1", "port": 8188, "url": "http://127.0.0.1:8188", "pid": 94850}, "error": null, "changed": true}
$ horus stop-comfyui
horus stop-comfyui: horus did not start the ComfyUI that answers at http://127.0.0.1:8188 (~/horus/horus-comfyui.state.json holds no ComfyUI that horus started), so it does not stop it. Stop it where it was started.
exit 1
$ curl -s -o /dev/null -w "%{http_code}
" http://127.0.0.1:8188/system_stats
200
$ comfy stop
Background ComfyUI is stopped. (127.0.0.1:8188)
{"schema": "envelope/1", "type": "envelope", "ok": true, "command": "stop", "version": "1.25.0", "where": null, "data": {"stopped": true, "host": "127.0.0.1", "port": 8188, "pid": 94850}, "error": null, "changed": true}
exit 0
$ curl (after comfy stop)
000
curl exit 7
```

The refusal exits 1 and the server still answers (200) after it. `comfy stop` stops it.

Stub check 1: the binary of `51c0fe3` (the parent of T1, `go build -o <scratch>/horus-pre-t1`)
fails step 3 at the install and creates no folder:

```
$ horus-pre-t1 setup it-illustration -yes -comfyui-dir <scratch>/stub-dir
...
Plan: 4 step(s):
...
Running the plan.
FAIL: not supported on this OS yet
exit 1
```

Stub check 2: a ComfyUI started by hand (`comfy --workspace ~/horus/ComfyUI launch --background
...`), then `generate_image` through `horus mcp` with the subject and seed of the goal tool
(its `real_call`, 574 s):

```
ok: True started_backend: False size: (1536, 1024)
row mac-image would fail: True
```

Step 7: no defect. Two notes, no issue: the doctor line `comfy-cli` and the backend can name two
different `comfy` programs (decision above); `comfy launch` passes `--enable-manager` two times
to `main.py`, which ComfyUI accepts.

| Number | Before | After | Evidence |
|---|---|---|---|
| Images through MCP on macOS | 0 | 1 | `mac-image` above, `started_backend: true` |
| Manual steps of #53 | 3 | 2 | the token and the PATH (decision above); no `HORUS_COMFYUI_DIR` (`mac-setup` above) |
| Rows of Phase 3 with `--real` on the Mac | 0 of 5 | 5 of 5 | step 5 |

## T8

On the PC (Windows 11, NVIDIA GPU), on `main` at `0f9192b` (the merge of T7), in a worktree of
`origin/main` with no `.env`. The goal tool ran in a Python 3.12 venv of `tests/requirements.txt`.
`<scratch>` is a scratch folder outside the repo.

Red: not applicable: this task builds nothing. Each stub probe below is a Red of its own.

### Step 1: the binary

```
$ git rev-parse HEAD
0f9192b52ca8aec1c574a41e80cb1f2d266bfbd3
$ git status --porcelain
(no output)
$ Rename-Item %USERPROFILE%\go\bin\horus.exe horus.exe.old-20261112     (horus mcp, process 23880, holds it)
$ go install ./cmd/horus
exit 0
$ go version -m %USERPROFILE%\go\bin\horus.exe
	build	vcs.revision=0f9192b52ca8aec1c574a41e80cb1f2d266bfbd3
	build	vcs.time=2026-11-12T11:06:58Z
	build	vcs.modified=false
```

The binary before was `defd6ab`. CI of `0f9192b` (`gh run view 43206325839 --json jobs`): every job
`success` (`contract` and `workflow-parity` on `windows-latest` and `macos-latest`, `go windows/amd64`,
`go darwin/arm64`, `go linux/amd64`).

### Step 2: every row without a real machine, with the CI part

18:21:29 to 18:23:15:

```
$ python tools/goal_check.py --phase 3
PASS  mac-backend - 10 pinned tests pass; CI: 1 jobs green (go darwin/arm64); real part not run (needs --real on macOS)
PASS  mac-setup - 3 pinned tests pass; real part not run (needs --real on macOS)
FAIL  mac-image - needs --real on macOS
PASS  tcc-folders - 4 pinned tests pass; real part not run (needs --real on macOS)
PASS  out-of-memory - 2 pinned tests pass
PASS  one-config - .mcp.json ok; 2 pinned tests pass; real part not run (needs --real on Windows and on macOS)
PASS  one-setup-doc - 0 lines that the commands contradict; docs/setup.md has a macOS section
PASS  same-behaviour - contract: local 30 of 30; parity: 17 of 17; go test ./...: ok; archtest: ok, 0 allow list files; 249 Go test functions (baseline 215); 0 changed contract files; CI: 6 jobs green (contract windows-latest, contract macos-latest, workflow-parity windows-latest, workflow-parity macos-latest, go windows/amd64, go darwin/arm64); real part not run (needs --real on Windows)
goal: 7 of 8 rows passed
note: the real part did not run for mac-backend, mac-setup, tcc-folders, one-config, same-behaviour (run with --real on the Mac and on the PC)
exit 1
```

`out-of-memory`, `one-setup-doc` and `same-behaviour` print PASS. `mac-image` and the real parts of
the other rows name `needs --real`. The rows of the Mac pass with `--real` on the Mac in T7, and
again in T9.

### Step 3: the real rows on the PC

The owner's ComfyUI on port 8188 did not run (decision above). It got no request that changes
anything, and nothing started or stopped it.

```
$ curl http://127.0.0.1:8188/system_stats          (18:20)
no answer: Unable to connect to the remote server
```

**The row `backend` of Phase 2**, with the method of Phase 2 (T7): a second install folder
`<scratch>\comfy8189` with a junction `python_embeded` to the owner's install, a folder `ComfyUI`
with 22 junctions (each folder of the owner's `ComfyUI` except `user`, `input`, `output`, `temp`), 4
empty folders of its own and 36 copied small files. Settings: `HORUS_ROOT` = the worktree,
`HORUS_COMFYUI_DIR=<scratch>\comfy8189`, `HORUS_COMFYUI_URL=http://127.0.0.1:8189`, `HORUS_AUTOSTART=1`.

GPU before the run (`nvidia-smi --query-gpu=memory.used,utilization.gpu --format=csv,noheader`,
and `--query-compute-apps`):

```
18:20     87 MiB, 25 %
18:23:37  87 MiB, 25 %   GPU process: LeagueClientUxRender.exe (the client window of a game)
18:23:42  87 MiB, 25 %   ports 8188 and 8189: no listener
```

The run, 18:23:56 to 18:29:23:

```
$ python tools/goal_check.py --phase 2 --real --only backend --real-out <scratch>\real
PASS  backend - 1536x1024 <scratch>\real\real-image-seed7-20261112-182427.png (seed 7, 296 s); started through the backend
goal: 1 of 1 rows passed
exit 0
nvidia-smi after: 125 MiB, 6 %; ports 8188 and 8189: no listener
```

The image: on the left a mint-green server box, on the right a sky-blue server box, and between
them one purple conveyor belt with 4 yellow-orange envelopes. It shows the subject (`REAL_SUBJECT`
of `tools/goal_check.py`). The envelopes are more amber than sun-yellow, and the belt is a deeper
purple than lilac, as in Phase 2. Unlike the Mac image of T7, the belt goes from one box to the
other. Small marks on the label of the belt frame look like text, a limit of the model.

**The row `one-config`** with only the owner's setting first (decision above):

```
$ python tools/goal_check.py --phase 3 --real --only one-config      (HORUS_COMFYUI_DIR=D:\ComfyUI_windows_portable)
FAIL  one-config - .mcp.json ok; 2 pinned tests pass; real: horus doctor on win32: exit 1, 1 FAIL lines
goal: 0 of 1 rows passed
$ horus doctor
FAIL  ComfyUI server - no answer at http://127.0.0.1:8188. Fix: horus start-comfyui
(the other 6 lines PASS)
```

Then the second ComfyUI on port 8189, started through the backend:

```
$ horus start-comfyui      (HORUS_COMFYUI_DIR=<scratch>\comfy8189, HORUS_COMFYUI_URL=http://127.0.0.1:8189; 18:30:00)
Started ComfyUI (process 23000). Waiting for http://127.0.0.1:8189 ...
OK: ComfyUI answers at http://127.0.0.1:8189
exit 0
```

With `HORUS_COMFYUI_DIR=D:\ComfyUI_windows_portable` and `HORUS_COMFYUI_URL=http://127.0.0.1:8189`:

```
$ python tools/goal_check.py --phase 3 --real --only one-config
PASS  one-config - .mcp.json ok; 2 pinned tests pass; real: horus doctor on win32: 0 FAIL lines
goal: 1 of 1 rows passed
exit 0
$ horus doctor
PASS  ComfyUI folder - D:\ComfyUI_windows_portable
PASS  ComfyUI server - answers at http://127.0.0.1:8189 (ComfyUI 0.44.0)
PASS  comfy-mcp environment - %USERPROFILE%\.venvs\comfy-mcp\Scripts\comfy-mcp.exe
PASS  comfy-cli - %USERPROFILE%\.venvs\comfy-mcp\Scripts\comfy.exe
PASS  .mcp.json - command points at %USERPROFILE%\go\bin\horus.exe comfy-mcp
PASS  horus on PATH - %USERPROFILE%\go\bin\horus.exe
PASS  .mcp.json horus - command points at %USERPROFILE%\go\bin\horus.exe
All checks passed.
exit 0
$ python tools/goal_check.py --phase 2 --real --only commands
PASS  commands - horus doctor: 0 FAIL lines; horus setup it-illustration: 0 steps, nothing is missing; 5 of 5 named files present and unchanged
goal: 1 of 1 rows passed
exit 0
$ horus setup it-illustration
Setup of workflow 'it-illustration':
PRESENT   ComfyUI in D:\ComfyUI_windows_portable
PRESENT   diffusion flux3-image-klein-int8-convrot.safetensors (5.83 GB) %USERPROFILE%\ComfyUI-models\diffusion_models\flux3-image-klein-int8-convrot.safetensors
PRESENT   text_encoder qwen3vl_4b_int8_convrot.safetensors (4.86 GB) %USERPROFILE%\ComfyUI-models\text_encoders\qwen3vl_4b_int8_convrot.safetensors
PRESENT   vae flux2-vae.safetensors (0.34 GB) %USERPROFILE%\ComfyUI-models\vae\flux2-vae.safetensors
Plan: 0 steps. Nothing is missing.
exit 0
```

**The new `.mcp.json` in Claude Code** (left to T8 by T5), from the worktree root:

```
$ claude mcp list
horus: %USERPROFILE%\go\bin\horus.exe mcp - ✔ Connected
comfy-mcp: horus comfy-mcp - ⏸ Pending approval (run `claude` to approve)
[Conflicting scopes]
├ Server "horus" is defined in multiple scopes with different endpoints: user (%USERPROFILE%\go\bin\horus.exe mcp), project (horus mcp).
```

(The other lines are servers of other projects.) T8 does not approve the project server (decision
above). An MCP client (`mcp` of `tests/requirements.txt`) started each entry of `.mcp.json` as
written, with the PATH lookup:

```
comfy-mcp: command horus comfy-mcp -> %USERPROFILE%\go\bin\horus.EXE
  initialize: server comfy-mcp, protocol 2025-11-25
  tools/list: 39 tools
horus: command horus mcp -> %USERPROFILE%\go\bin\horus.EXE
  initialize: server horus, protocol 2025-11-25
  tools/list: 5 tools
exit 0
```

**After the runs** (18:31:56), with the settings of the start:

```
$ horus stop-comfyui      (HORUS_COMFYUI_DIR=<scratch>\comfy8189, HORUS_COMFYUI_URL=http://127.0.0.1:8189)
OK: stopped the ComfyUI that horus started at http://127.0.0.1:8189 (process 23000).
exit 0
listeners on 8188 and 8189: 0
processes with comfy8189 in the command line: 0
nvidia-smi: 125 MiB, 27 %
```

Cleanup: the 23 junctions (1 + 22) were deleted one at a time with `(Get-Item <junction>).Delete()`,
which removes only the link. Then:

```
$ cmd /c dir /AL /S /B <scratch>\comfy8189
File Not Found
```

Then the 44 files were deleted one by one, and the 8 folders, each after it was empty, with a
non-recursive delete. No recursive delete ran. After the cleanup:

```
Test-Path D:\ComfyUI_windows_portable\ComfyUI\main.py               True
Test-Path D:\ComfyUI_windows_portable\python_embeded\python.exe     True
entries in D:\ComfyUI_windows_portable\ComfyUI\models               26
flux3-image-klein-int8-convrot.safetensors   5827874779 bytes
qwen3vl_4b_int8_convrot.safetensors          4858559240 bytes
flux2-vae.safetensors                         336211292 bytes
Test-Path <scratch>\comfy8189                                       False
```

The byte sizes are equal to `workflows/it-illustration/models.json`.

### Step 4: stub probes of the goal tool

Each probe ran in a second worktree of `origin/main` in `<scratch>`, not in the tree of the pull
request. Each change was restored with `git checkout -- .`, and `git status --porcelain` printed
nothing at the end. The worktree was removed with `git worktree remove`. The rows that run Go tests
ran with `--no-ci`, so that the stub, not a missing CI run, gives the FAIL.

```
  # mac-backend: Start of the comfy-cli backend returns nil at once (comfycli.go:114)
FAIL  mac-backend - no passing test: TestComfyCLIIsTheBackendOfDarwin (did not pass in internal/backend: FAIL, T1); TestComfyCLIStartRunsLaunchAndTheServerAnswers (did not pass in internal/backend: FAIL, T2); TestComfyCLIStartFailsWhenComfyUIIsNotInstalled (did not pass in internal/backend: FAIL, T2); TestComfyCLIStopRunsStopAndTheServerEnds (did not pass in internal/backend: FAIL, T2); TestGenerateStartsComfyUIThroughTheComfyCLIBackend (did not pass in cmd/horus: FAIL, T2)
  # mac-setup: defaultComfyDir gives ComfyUI_windows_portable on every OS (platform.go:130, `if true {`)
FAIL  mac-setup - no passing test: TestDefaultComfyDirOfEachOS (did not pass in internal/platform: FAIL, T1)
  # mac-image: no --real
FAIL  mac-image - needs --real on macOS
  # mac-image: --real on Windows
FAIL  mac-image - needs --real on macOS; the real part needs macOS (this machine: win32)
  # tcc-folders: inTCCFolder always answers "outside" (platform.go:152, `if true || ...`)
FAIL  tcc-folders - no passing test: TestInTCCFolder (did not pass in internal/platform: FAIL, T3)
  # out-of-memory A: the memory error gives GENERATION_FAILED (jobs.go:97)
FAIL  out-of-memory - no passing test: TestJobErrorOfTheMPSOutOfMemoryText (did not pass in internal/comfy: FAIL, T4); TestImageMPSOutOfMemoryThroughTheAdapter (did not pass in cmd/horus: FAIL, T4)
  # out-of-memory B: the pinned test of internal/comfy holds "CUDA out of memory", not the MPS text (the test itself passes: ok internal/comfy)
FAIL  out-of-memory - no passing test: TestJobErrorOfTheMPSOutOfMemoryText (has no text 'MPS backend out of memory', T4)
  # one-config A: the .mcp.json of 250302f (before T5)
FAIL  one-config - .mcp.json: 2 Windows-only paths, the entry comfy-mcp is not `horus comfy-mcp`; 2 pinned tests pass
  # one-config B: RunComfyMCP returns 0 at once and starts nothing (comfymcp.go:153)
FAIL  one-config - .mcp.json ok; no passing test: TestComfyMCPPassesTheStreamsComfyBinAndTheExitCode (did not pass in cmd/horus: FAIL, T5)
  # one-setup-doc A: docs/setup.md without the word macOS in its 2 headings
FAIL  one-setup-doc - 0 lines that the commands contradict; docs/setup.md has no macOS section
  # one-setup-doc B: docs/setup.md and internal/cli/setup.go of b7b71cf (before T6)
FAIL  one-setup-doc - 3 lines that the commands contradict; docs/setup.md:18, docs/setup.md:112, internal/cli/setup.go:38, docs/setup.md has no macOS section
  # same-behaviour A: 4 test files removed (download, devtools, comfy, herr: 37 Go test functions)
FAIL  same-behaviour - contract: local 30 of 30; parity: 17 of 17; go test ./...: ok; archtest: ok, 0 allow list files; 212 Go test functions (baseline 215); 0 changed contract files; CI part skipped (--no-ci)
  # same-behaviour B: one comment line added to tests/workflow_parity.py
FAIL  same-behaviour - contract: local 30 of 30; parity: 17 of 17; go test ./...: ok; archtest: ok, 0 allow list files; 249 Go test functions (baseline 215); 2 changed contract files: M	tests/workflow_parity.py, warning: in the working copy of 'tests/workflow_parity.py', LF will be replaced by CRLF the next time Git touches it; CI part skipped (--no-ci)
  # same-behaviour C: internal/archtest is one file `package archtest` (no rules, no tests)
FAIL  same-behaviour - contract: local 30 of 30; parity: 17 of 17; go test ./...: ok; archtest: no passing test: TestRules, TestPlanted, TestProblems, 0 allow list files; 246 Go test functions (baseline 215); 0 changed contract files; CI part skipped (--no-ci)
```

Each run above ended with `goal: 0 of 1 rows passed` and exit 1. A first try of probe A of
`same-behaviour` removed `internal/generate/generate_test.go`, and the build of the tests failed
(`fake_test.go:212:12: undefined: contains`). That FAIL came from the build, not from the count, so
the probe was run again with test files that no other file needs.

The CI parts and the real part of `same-behaviour`, judged by the functions of the tool on job
lists and results made by hand (no network, no GPU):

```
all green           same-behaviour: (True, 'CI: 6 jobs green (contract windows-latest, contract macos-latest, workflow-parity windows-latest, workflow-parity macos-latest, go windows/amd64, go darwin/arm64)')
red go darwin/arm64 mac-backend:    (False, 'CI: not green: go darwin/arm64 (failure)')
red go linux/amd64  mac-backend:    (False, 'CI: another job is not green: go linux/amd64 (failure)')
parity macos running same-behaviour: (False, 'CI: not green: workflow-parity macos-latest (unknown)')
real part of same-behaviour, backend fails: (False, 'backend: FAIL generate_image failed: BACKEND_UNAVAILABLE (stub); commands: PASS stub pass')
real part of mac-backend on Windows: (False, 'the real part needs macOS (this machine: win32)')
real part of tcc-folders on Windows: (False, 'the real part needs macOS (this machine: win32)')
```

No probe passed, so no issue for the tool. The real parts of the Mac rows (`mac-image` with a
ComfyUI that a person started, `mac-backend` without a refusal) are probed by the tests of the tool
(`tools/test_goal_check.py`, the stub table of T0) and by stub check 2 of T7 on the Mac.

The tests of the tool on `0f9192b`:

```
$ python -m unittest discover -s tools
Ran 52 tests in 66.724s

OK
```

### Step 5: the check of the epic tool on every closed task

```
$ node epic.mjs -R hieplam/horus check <n> --dry-run --evidence docs/plans/2026-11-12-macos-backend-evidence.md --heading "## T<k>"
PASS #100 via PR #111 (https://github.com/hieplam/horus/pull/111)
PASS #101 via PR #113 (https://github.com/hieplam/horus/pull/113)
PASS #102 via PR #116 (https://github.com/hieplam/horus/pull/116)
PASS #103 via PR #115 (https://github.com/hieplam/horus/pull/115)
PASS #104 via PR #112 (https://github.com/hieplam/horus/pull/112)
PASS #105 via PR #118 (https://github.com/hieplam/horus/pull/118)
PASS #106 via PR #120 (https://github.com/hieplam/horus/pull/120)
PASS #107 via PR #121 (https://github.com/hieplam/horus/pull/121)
```

The 8 issues are closed. The sub-issues of #98 that are open: #108 (this task), #109 and #110.

### The review of T7 (PR #121)

Each Green of plan section T7 has its literal output in `## T7`:

| Green of T7 | Literal output in `## T7` |
|---|---|
| Red: `horus doctor` prints `FAIL  ComfyUI folder` for `~/horus` | yes, the first line of the Red |
| Step 4: 0 FAIL lines and `Plan: 0 steps. Nothing is missing.` | yes, `All checks passed.` and the second plan |
| Step 5: `goal: 5 of 5 rows passed`, a 1536x1024 PNG, `started_backend: true` | yes |
| Step 6: the refusal, exit 1, the server still answers | yes, `exit 1` and `200` |
| Stub check: the backend before T1 fails step 3; an image from a ComfyUI that a person started fails | yes, `FAIL: not supported on this OS yet` and `started_backend: False` |
| Ratchet: images on macOS 0 → 1, manual steps 3 → 2 | yes, the table |

No line contradicts a goal row. `tcc-folders` asks for a WARN line that names the path: the lines
name `~/Desktop/Private/horus`. `mac-setup` asks for no `HORUS_COMFYUI_DIR`: decision of T7 took it
out, with `HORUS_MODELS_DIR`. `mac-image` asks for a picture that shows the subject: the agent
opened `workflows/it-illustration/examples/mac-conveyor-belt.png` again for this review. Two boxes in
the colours of the subject, amber envelopes on lilac belts, and each box has its own belt. The
README of the workflow names that gap. The note about who merged PR #121 is in the decisions.

### Ratchets

The table of the spec, before → after, on `main` at `0f9192b`:

| Row | Ratchet | Before | After | Command |
|---|---|---|---|---|
| `mac-backend` | Commands that answer "not supported" on macOS | 3 | 0 | step 2 `mac-backend` (10 pinned tests: `Install`, `Start` and `Stop` of the comfy-cli backend), and the real part on the Mac in T7 |
| `mac-setup` | Manual steps of #53 | 3 | 2 | the Mac only: T7, step 5 and the decision of T7 (not measured on the PC) |
| `mac-image` | Images through MCP on macOS | 0 | 1 | the Mac only: T7, step 5 (not measured on the PC) |
| `tcc-folders` | Default folders that fit macOS | 0 | 1 | step 2 `tcc-folders` (`TestDefaultComfyDirOfEachOS`: `<home>/horus` for `darwin`) |
| `out-of-memory` | Test cases with the MPS text | 0 | 2 | `git grep -l "MPS backend out of memory" -- '*_test.go'`: `cmd/horus/image_test.go`, `internal/comfy/comfy_test.go`; step 2 `2 pinned tests pass` |
| `one-config` | Windows-only paths in `.mcp.json` | 2 | 0 | `git grep -n "USERPROFILE" -- .mcp.json` (no output, exit 1); step 2 `.mcp.json ok` |
| `one-setup-doc` | Sentences that the commands contradict | 4 | 0 | step 2 `0 lines that the commands contradict` |
| `same-behaviour` | Go test functions, never below the baseline | 215 | 249 | `go test -list . ./...`, lines `^Test\w+$`: 249; step 2 |
| all | Rows of the goal tool that pass on the PC | 1 of 8 (baseline) | 7 of 8 without `--real`; `one-config` and the PC part of `same-behaviour` (`backend`, `commands`) pass with `--real` | steps 2 and 3 |
| all | Probes that pass on a stub | not measured | 0 of 20 (14 runs of the tool, 6 judgements of made-up input) | step 4 |

## T9

On the Mac (Apple M4, 16 GB, macOS 26.5.1), on `main` at `866006e` (the merge of T8, a later
commit than the one T8 verified), in the main checkout with a clean tree. The goal tool ran in a
Python 3.12 venv of `tests/requirements.txt`. Paths: `~` is the home folder, `<scratch>` a scratch
folder outside the repo.

Red: not applicable: this task builds nothing. The stub check below is the Red of the row
`mac-image`.

### Step 1: the binary and a cold start

```
$ git pull
Already up to date.
$ git rev-parse HEAD
866006eec6b39cc4597e4ea194453aa271407f17
$ go install ./cmd/horus
exit 0
$ go version -m ~/go/bin/horus
	build	vcs.revision=866006eec6b39cc4597e4ea194453aa271407f17
	build	vcs.time=2026-11-12T11:43:01Z
	build	vcs.modified=false
$ horus stop-comfyui
No ComfyUI that horus started runs at http://127.0.0.1:8188. Nothing to stop.
exit 0
$ curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8188/system_stats
000
curl exit 7
```

No ComfyUI answered before the run, so the image run was cold.

### Step 2: every row with --real (Green)

14:54:47Z to 15:04:08Z:

```
$ python tools/goal_check.py --phase 3 --real --real-out <scratch>/real-out
PASS  mac-backend - 10 pinned tests pass; CI: 1 jobs green (go darwin/arm64); real: start, stop, and a refusal of a ComfyUI started by hand
PASS  mac-setup - 3 pinned tests pass; real: horus doctor: 0 FAIL lines; Plan: 0 steps; no HORUS_COMFYUI_DIR
PASS  mac-image - 1536x1024 <scratch>/real-out/real-image-seed7-20261112-215525.png (seed 7, 469 s); started_backend: true
PASS  tcc-folders - 4 pinned tests pass; real: horus doctor: exit 0, 0 FAIL lines, a WARN line names ~/Desktop/Private/horus
PASS  out-of-memory - 2 pinned tests pass
PASS  one-config - .mcp.json ok; 2 pinned tests pass; real: horus doctor on darwin: 0 FAIL lines
PASS  one-setup-doc - 0 lines that the commands contradict; docs/setup.md has a macOS section
PASS  same-behaviour - contract: local 30 of 30; parity: 17 of 17; go test ./...: ok; archtest: ok, 0 allow list files; 249 Go test functions (baseline 215); 0 changed contract files; CI: 6 jobs green (contract windows-latest, contract macos-latest, workflow-parity windows-latest, workflow-parity macos-latest, go windows/amd64, go darwin/arm64); real: the PC part (Phase 2 rows backend and commands) runs with --real on Windows
goal: 8 of 8 rows passed
real 560.86
user 12.78
sys 11.21
exit=0
```

The cold time of the image: 469 s (start of ComfyUI through the comfy-cli backend, model load and
render), `started_backend: true`. T7 measured 434 s. The PC part of `same-behaviour` passed on the
PC in T8 (step 3: `backend` and `commands`).

### Step 3: the PNG

`<scratch>/real-out/real-image-seed7-20261112-215525.png`, 1536x1024, 1856062 bytes, SHA-256
`56ffb0e585e7168d86700ce99ea9466be36936490050ace1bb0a9b97b1be3064`.

Judged against the subject (`REAL_SUBJECT` of `tools/goal_check.py`): a mint-green server box on
the left, a sky-blue server box on the right, 4 amber envelopes (2 on each belt) on lilac conveyor
belts. The same gap as in T7: each box has its own belt, and the subject asks for one belt from the
left box to the right box. Small marks on the boxes look like text. Both are limits of the model,
named in the workflow README, not of horus. The picture is the same as the T7 PNG
(`workflows/it-illustration/examples/mac-conveyor-belt.png`, the same seed); the bytes differ
(decision above).

After the run, ComfyUI was stopped again:

```
$ horus stop-comfyui
Background ComfyUI is stopped. (127.0.0.1:8188)
OK: stopped the ComfyUI that horus started at http://127.0.0.1:8188 (process 34873).
exit 0
$ curl (after the stop)
000
curl exit 7
```

### Stub check: no autostart, ComfyUI stopped

15:04:25Z:

```
$ HORUS_AUTOSTART=0 python tools/goal_check.py --phase 3 --real --real-out <scratch>/stub-out --only mac-image
FAIL  mac-image - generate_image failed: BACKEND_UNAVAILABLE: ComfyUI does not answer at http://127.0.0.1:8188, and HORUS_AUTOSTART is off
goal: 0 of 1 rows passed
exit 1
$ curl (after the stub check)
000
curl exit 7
```

The row fails with `BACKEND_UNAVAILABLE`, writes no PNG (`<scratch>/stub-out` holds only the
stderr log), and nothing started ComfyUI.

### Ratchet

| Row | Ratchet | Before | After | Command |
|---|---|---|---|---|
| all | Goal rows that pass on `main`, on the Mac with `--real` | 0 | 8 | step 2: `goal: 8 of 8 rows passed` at `866006e` |
| `mac-image` | Cold time of the image on the Mac | 434 s (T7) | 469 s | step 2 |
| `mac-image` | Stub runs that pass | not measured | 0 of 1 | stub check |

## T10

On the Mac (Apple M4, 16 GB, macOS 26.5.1), in the main checkout with a clean tree. Paths: `~`
is the home folder, `<scratch>` a scratch folder outside the repo. Part A and part B ran as new
agent sessions: part A a subagent that got only `README.md`, `docs/setup.md` and the pages that the
setup page links to; part B `claude -p`. Part C was a read of the code by a subagent, checked by
hand.

Red: not applicable: this task builds nothing.

### Decisions of T10

| Decision | Why |
|---|---|
| A step that the page names in its prose, or that a command names in its output, is not "a step that the page does not name", also when it is outside the code block. Such a finding becomes a follow-up (#129), not a defect. | The goal row `one-setup-doc` says "a step that the page does not name". The restart of step 4 is in the bullet below the block (`docs/setup.md:305`). |
| Part A runs in a tool shell that reads no shell profile, and it may not write in the repo. The `export PATH` of each call and an output path outside the repo are not unnamed steps. Step 3 (`.env`) was skipped: no download was needed. | They come from the test harness, not from the page. Step 0 of the page names the profile line and a new terminal. |
| `Backend.Layout()` has no caller outside `internal/backend` (#124). It is a follow-up for the owner, not a defect of this phase. | `Layout` is part of the locked decision D37 of #4, and the spec says "The `Backend` interface does not change". A fix reopens D37. |
| Part B ran a second time after the first try timed out. | The first try ran while the Mac swapped (27.8 GB of 28.7 GB of swap used by other programs); the server log shows each prompt took 30 min. horus answered `GENERATION_TIMEOUT` with `retryable: true`, as designed. The second try ran with the memory free. |

### Part A, run 1 (on `8cdd194`)

A second install with `HORUS_COMFYUI_DIR=~/horus-accept`, `HORUS_COMFYUI_URL=http://127.0.0.1:8189`
and `HORUS_MODELS_DIR=~/horus/ComfyUI/models`. `horus install-comfyui` (74 s) and `horus
start-comfyui` (64 s) worked as the page says. Then:

```
$ horus workflow install it-illustration
SKIP flux3-image-klein-int8-convrot.safetensors : already present and verified.
SKIP qwen3vl_4b_int8_convrot.safetensors : already present and verified.
SKIP flux2-vae.safetensors : already present and verified.
NOT SEEN  the server does not list flux3-image-klein-int8-convrot.safetensors. Check extra_model_paths.yaml, then restart ComfyUI.
NOT SEEN  the server does not list qwen3vl_4b_int8_convrot.safetensors. Check extra_model_paths.yaml, then restart ComfyUI.
NOT SEEN  the server does not list flux2-vae.safetensors. Check extra_model_paths.yaml, then restart ComfyUI.
FAIL: The files are installed, but 3 check(s) failed.
$ ls ~/horus-accept/ComfyUI/extra_model_paths.yaml
ls: ...: No such file or directory
```

`docs/setup.md:305` says "When a model folder is outside ComfyUI, the command adds it to
`extra_model_paths.yaml`." The agent wrote the file by hand: 1 step that the page does not name.
Cause: `Service.Install` mapped only the folders of files that it downloaded. Defect **#125**,
fixed by #126 (`a5bc2f5`). The image of run 1 (1536x1024, 1152.8 s while the Mac swapped) showed
its subject.

### Part A, run 2 (on `a5bc2f5`, Green)

A new agent, the same settings. 16:59Z to 17:13Z.

```
$ horus install-comfyui                         exit 0, about 71 s
OK: ComfyUI v0.44.0 is installed in ~/horus-accept
$ horus start-comfyui                           exit 0, about 47 s
OK: ComfyUI answers at http://127.0.0.1:8189
$ horus workflow install it-illustration        exit 1
SKIP flux3-image-klein-int8-convrot.safetensors : already present and verified.   (x3, no download)
extra_model_paths.yaml: added diffusion_models -> ~/horus/ComfyUI/models/diffusion_models
extra_model_paths.yaml: added text_encoders -> ~/horus/ComfyUI/models/text_encoders
extra_model_paths.yaml: added vae -> ~/horus/ComfyUI/models/vae
Restart ComfyUI so that it reads the new paths (horus start-comfyui).
NOT SEEN  the server does not list flux3-image-klein-int8-convrot.safetensors. Check extra_model_paths.yaml, then restart ComfyUI.   (x3)
FAIL: The files are installed, but 3 check(s) failed. See the lines above.
$ horus stop-comfyui && horus start-comfyui     (the restart of docs/setup.md:305)
OK: ComfyUI answers at http://127.0.0.1:8189
$ horus sync-workflows                          exit 0
$ horus doctor -workflow it-illustration        exit 0, 5.5 s
PASS  models for 'it-illustration' - all files present and verified (SHA-256 checked)
All checks passed.
$ horus run-illustration -subject "..." -out <scratch>/out/robot-tree.png
{"ok":true,...,"width":1536,"height":1024,"execution_seconds":394.6,...}
$ horus stop-comfyui
OK: stopped the ComfyUI that horus started at http://127.0.0.1:8189 (process 65787).
$ curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:8189/system_stats
000
$ rm -rf ~/horus-accept
```

`extra_model_paths.yaml`, as horus wrote it:

```
horus_diffusion_models:
    base_path: ~/horus/ComfyUI/models
    diffusion_models: diffusion_models

horus_text_encoders:
    base_path: ~/horus/ComfyUI/models
    text_encoders: text_encoders

horus_vae:
    base_path: ~/horus/ComfyUI/models
    vae: vae
```

The PNG: `<scratch>/out/robot-tree.png`, 1536x1024, 1274675 bytes, SHA-256
`e9bee0ede56d5bc20d26b39fdc2ba088c157e1175a065f27628935cb8e1a03b3`. Subject: an orange robot with
round white eyes waters a small tree with blue leaves in a white pot with a yellow watering can,
on a cream background, no text. The picture shows each part, with no text.

What the agent found, and the verdict:

| Finding of the agent | Verdict |
|---|---|
| The page has no walkthrough for a second install. | Not a step: the brief gave the 3 settings, and both commands are on the page. |
| The restart between the commands of step 4. | Named by the page (`docs/setup.md:305`) and by the `NOT SEEN` lines. Follow-up **#129** (the block and the line `(horus start-comfyui)`, which does nothing while the server answers). |
| `docs/setup.md:307`: "that needs a few minutes, because it hashes 11 GB". `horus doctor -workflow` took 5.5 s. | A sentence that a command contradicts. Defect **#127**, fixed by #128. `horus workflow check it-illustration` measured again by hand: `real 5.21`, `user 3.36` for 10.2 GB; the code keeps no hash cache. |

Steps that the page does not name: 0. Sentences that a command contradicts, after #128: 0.

### Part B, a new agent through MCP (Green)

The config `<scratch>/partB/mcp.json` starts `~/go/bin/horus mcp` with `HORUS_ROOT` on the repo and
`HORUS_OUTPUT_DIR` on `<scratch>/partB/out`. The binary was built at `a5bc2f5`
(`vcs.modified=false`). No ComfyUI answered on port 8188 before the run.

```
$ claude -p --strict-mcp-config --mcp-config <scratch>/partB/mcp.json \
    --allowedTools "mcp__horus__list_workflows,mcp__horus__describe_workflow,mcp__horus__generate_image" \
    --output-format json "You have an image tool through MCP. Find the workflows, read the one for tech illustrations, then make one image of a subject that you choose yourself. ..."
16:52:11Z to 16:59:24Z, exit 0, is_error false, 6 turns, permission_denials []
```

The envelope of `generate_image`:

```
{"ok":true,"result":{"workflow":"it-illustration","path":"<scratch>/partB/out/it-illustration/20261112T165229Z-3190444361.png","bytes":1116869,"width":1536,"height":1024,"seed":3190444361,"seconds":406.4,"backend_seconds":398.1,"started_backend":true,"job_id":"6e9246e0-96fe-4fb5-b35a-285db8f4cffe"},"error":null}
```

The PNG: 1536x1024, 1116869 bytes, SHA-256
`59139d1cfb91662057b162b95794e6bc31f00969e80436eab43cafbe6710944d`. Subject of the agent: "a database
cache layer: a laptop sends requests through a yellow cache box with a lightning bolt to a green
database cylinder". The picture shows a laptop with a bar chart, arrows to a yellow box with a
lightning bolt, an arrow to a green database cylinder, and no text. It shows its subject.
`horus stop-comfyui` then stopped the ComfyUI that `generate_image` started.

The first try (15:50Z to 16:51Z, on `8cdd194`) returned two times
`{"ok":false,...,"error":{"code":"GENERATION_TIMEOUT","message":"the job did not finish in 1800 s","retryable":true,...}}`
while `vm.swapusage` showed `used = 27846.81M` of `28672.00M` (decision above).

### Part C, the design

Each package read file by file against the design rules of `AGENTS.md` and
`docs/specs/2026-11-11-restructure.md`. The search for OS tests (`runtime.GOOS`, `//go:build`,
`_windows.go`, `_darwin.go`, `"windows"`, `"darwin"`, `\\`, `.exe`) found each real OS branch in
`internal/platform` or `internal/backend` (and the rule text of `internal/archtest`).

| Package | Result |
|---|---|
| `internal/backend` | No defect. `started.go` holds the state file, `alreadyUp`, `waitUp`, `stopStarted`, `waitDown`, `answers` and `portOf` for both backends; each backend adds only its own step. `New`, `NewFor` and `TakesArchive` are the only reads of `runtime.GOOS`. Follow-up **#124** (`Layout()` has no outside caller; locked by D37). |
| `internal/backend/comfyclitest` | No defect. Imports only `internal/platform` and the standard library. |
| `internal/platform` | No defect. Every `runtime.GOOS` and build tag of the product is here; `DefaultComfyDir`, `InTCCFolder`, `ExeName`, `VenvBin` are pure functions without an interface. |
| `internal/doctor` | No defect. `Server`, `Install` and `Programs` are small interfaces that the service declares; `expand`, `resolve`, `isHorus` are pure. `isHorus` strips `.exe` on every OS, which is harmless and no OS test. |
| `internal/provision` | No defect. Its `Backend` interface is `Installed` and `Install` only; the decisions (`fileCheck`, `graphCheck`, `serverCheck`, `removeDecision`, `mappings`, `steps`) are pure functions. The defect of part A (#125) was in `Install`, not in the design. |
| `cmd/horus` (`main.go`, `wiring.go`) | No defect. `main.go` imports handlers and base packages; `wiring.go` is the one place that imports every layer. `seams.goos` is the test seam of decisions 41 and 51. |

`go test ./internal/archtest` passes on `main`.

### Stub check

`docs/setup.md` at `943d430` says: "These commands are Windows-only in this phase. On macOS and
Linux, install and start ComfyUI by hand." The `horus` of that commit, on the Mac, with the
settings of part A:

```
$ go build -o <scratch>/horus943 ./cmd/horus    (in a worktree at 943d430)
$ HORUS_COMFYUI_DIR=<scratch>/stub-accept HORUS_COMFYUI_URL=http://127.0.0.1:8189 <scratch>/horus943 install-comfyui
horus install-comfyui: not supported on this OS yet
exit=2
$ ls <scratch>/stub-accept
ls: ...: No such file or directory
```

Part A on the page of `943d430` stops at the install of ComfyUI.

### Issues of the acceptance

| Issue | Kind | State |
|---|---|---|
| #125 `workflow install` maps no folder for present files | defect (goal row `one-setup-doc`) | closed by #126 |
| #127 the hash time on the setup page and in `doctor -h` | defect (goal row `one-setup-doc`) | closed by #128 |
| #124 `Backend.Layout()` has no outside caller | follow-up (reopens D37) | open, for the owner |
| #129 the restart of step 4 outside the block, and the line `(horus start-comfyui)` | follow-up | open |

### Ratchet

| Ratchet | Before | After | Command |
|---|---|---|---|
| Open defects of the acceptance | 2 (#125, #127) | 0 | `gh issue view 125`, `gh issue view 127`: `CLOSED` |
| Steps of part A that the page does not name | 1 (run 1) | 0 (run 2) | part A |
| Sentences of part A that a command contradicts | 1 | 0 | #128 |
| Part B `generate_image` with `ok: true` | 0 of 1 (timeout, the Mac swapping) | 1 of 1 | part B |

## Issue 124

`Layout` leaves the exported interface `Backend` of `internal/backend`. The type is now `layout`.
No behaviour changes. Run on the PC (Windows), on the branch `refactor/124-backend-layout`.

Goal: the interface `Backend` has only the methods that callers outside the package use.

Red (on `origin/main` `4bda133`, before the change):

    $ git show origin/main:internal/backend/backend.go | grep -n "Layout() Layout"
    47:	Layout() Layout
    $ git grep -n "Layout()" origin/main -- '*.go' | grep -v "origin/main:internal/backend/"
    (no output: no caller outside the package)

Green (after the change):

    $ gofmt -l .
    (no output)
    $ go vet ./...            (also GOOS=darwin GOARCH=arm64 and GOOS=linux)
    (no output)
    $ go doc ./internal/backend Backend
    type Backend interface {
        Install(ctx context.Context, out io.Writer) error
        Start(ctx context.Context, out io.Writer) error
        Stop(ctx context.Context, out io.Writer) error
        Installed() (dir string, ok bool)
    }
    $ go test ./...
    ok for every package with tests (cmd/horus, archtest, backend, catalog, cli, comfy, devtools,
    doctor, download, generate, graph, herr, mcpserver, modelfiles, platform, provision, settings)
    $ python scripts/lint-public.py
    lint-public: 0 finding(s)
    $ python tests/mcp_contract.py --server-cmd <scratch>/horus.exe mcp
    contract: 30 of 30 passed
    $ python tools/goal_check.py --phase 3 --no-ci --server-cmd <scratch>/horus.exe mcp
    goal: 7 of 8 rows passed      (mac-image: needs --real on macOS, as before)

Stub check: not applicable: the change removes an export.

Ratchet: exported methods of `Backend` that no outside caller uses: 1 -> 0.
Go test functions: 250 (`go test -list . ./...`), unchanged: the change renames, and deletes no test.

# Evidence log: restructure horus into three layers (Phase 2)

Plan: [`2026-11-11-restructure.md`](2026-11-11-restructure.md). Spec:
[`../specs/2026-11-11-restructure.md`](../specs/2026-11-11-restructure.md).
Each task adds one section `## <task id>` with the literal output of its Green
commands.

## Decisions

| Date | Decision | Why | By |
|---|---|---|---|
| 2026-11-11 | The owner ratified the 8 goal rows, the design rules and the standing rules of the spec. | The one ratification of this phase. | Owner |
| 2026-11-11 | `horus mcp` calls the `provision` service in its own process. D12 and D16 of #4 (a `horus workflow` child process) end. | The owner wants each part behind an interface that a test can replace. A child process with a text line is not such an interface. | Owner |
| 2026-11-11 | The row `process-tree` proves `Backend.Stop` and the detached start. It no longer proves the end of a download child. | After the decision above, the server has no download child. | Plan |
| 2026-11-11 | PR #49 (fixes of #39 to #42) and PR #51 merged before the first task. #40 and #41 are not tasks of this phase. | The fixes existed, and the move tasks touch the same files. | Plan |
| 2026-11-11 | T0 chose the allow list file names: `internal/archtest/allow_os-boundary.txt`, `internal/archtest/allow_direction.txt`, `internal/archtest/allow_token.txt`. One entry on each line. A blank line or a line that starts with `#` is not an entry. Task T1 uses these names. | The goal tool must read the lists before T1 exists. | Plan (T0) |
| 2026-11-11 | The row `layout` counts the packages of `go list ./...` that are not in `tools/phase2_packages.txt` (12 today). It also prints the number of listed packages that do not exist yet (11 today). The row passes only when both numbers are 0. | The ratchet "12 to 0" counts the extra packages. A missing package must also fail the row. | Plan (T0) |
| 2026-11-11 | The row `same-behaviour` accepts a renamed golden file when git reports the rename as 100 percent equal (`R100`). Any other change under the contract paths fails. | A golden file of `internal/server/testdata` moves with its package. The content does not change. | Plan (T0) |
| 2026-11-11 | The row `process-tree` needs a passing test whose name has `Tree` in it, in `internal/platform`. The row counts each occurrence of `taskkill` in Go files (8 today, in 6 lines). | The plan names "the tree test" but no test name. | Plan (T0) |
| 2026-11-11 | The rows `backend` and `commands` pass only with `--real`. Without it they print that the Go tests are fine and that the owner's PC is needed. `commands` passes when `horus doctor` has 0 FAIL lines and the plan of `horus setup it-illustration` says "nothing is missing". | Their oracle runs on the owner's PC, like the real-image row of Phase 1. | Plan (T0) |
| 2026-11-11 | The baseline commit of `same-behaviour` is `3e27da4`. The baseline number of Go test functions is 157. | Measured at T0 (see the Baseline section). | Plan (T0) |
| 2026-11-11 | The rule `token` flags a Go string whose value is `HF_TOKEN`, or that starts with `HF_TOKEN=` (a line of a `.env` file). It does not flag the error code `HF_TOKEN_INVALID`, a comment, or a longer text. The exempt places are `internal/download` and the two files of the refusal: `internal/settings/settings.go` and `internal/settings/settings_test.go`. The allow list has 4 files. | The `next_step` text of `HF_TOKEN_INVALID` in `internal/herr` names `HF_TOKEN`, and `internal/herr/testdata/errors.golden.json` holds that text. A golden file must not change, so a plain text match could never reach 0. Code that reads the token names the variable in a string of its own. | Agent (T1) |
| 2026-11-11 | The rule `os-boundary` flags a file when a part of its name after the first `_` is an OS name of Go or `unix`, when a `//go:build` line names an OS, when it uses `runtime.GOOS`, or when it imports `syscall` or `golang.org/x/sys/...`. Test files count. Nothing under `internal/platform` or `internal/backend` is flagged. There is no planted file for an old `// +build` line: `gofmt` adds a `//go:build` line to such a file, and CI fails a file that `gofmt -l` lists. The code still reads both kinds of line. | The spec names the four cases. The baseline of 19 files counts `_unix` files and test files. | Agent (T1) |
| 2026-11-11 | The layers of the Phase 1 packages in `internal/archtest/layers.go`: `internal/server` and `internal/scripts/runillustration` are handlers. `internal/models`, `checksetup`, `installcomfymcp`, `newworkflow`, `register`, `syncworkflows` and `workflow` are services. `downloadmodel`, `installcomfyui` and `startcomfyui` are adapters. A package that is not in the table fails the rule `direction`. | Each one gets the layer of the package that its code moves to (spec table, plan T1 step 2). `runillustration` moves to `internal/cli` (plan T5 step 5). | Agent (T1) |
| 2026-11-11 | `internal/archtest` and `internal/comfy/comfytest` are in the layer "test". Only a test file can import them. The spec sets no import limit for them, so archtest sets none. | The spec says only that a test file can import `comfytest`. | Agent (T1) |
| 2026-11-11 | An entry of the allow list `direction` is a pair `<package> -> <import>` for the whole package, test files included. The ban of a service is on the exact import paths `net/http` and `os/exec`. | The plan writes the entry as `internal/generate -> internal/comfy` (T5 Red). The spec names the two paths. | Agent (T1) |
| 2026-11-11 | A test file of a service follows the rules of the service, as the spec says. So T5 step 3 has one option only: a test of `internal/generate` uses a fake of the interface. A test that needs the wire format of the real `comfy.Client` goes to `internal/comfy` or `cmd/horus`. Today `internal/generate/generate_test.go` imports `internal/comfy`. | The spec wins over the plan. The other option of the plan keeps the entry `internal/generate -> internal/comfy` on the list. | Agent (T1) |
| 2026-11-11 | The Red of T1 printed `directory not found` and `[setup failed]` (Go 1.27.3), not "no Go files" or "cannot find package". | It is the same fact: the package does not exist. | Agent (T1) |
| 2026-11-11 | `tools/goal_check.py` does not change in T1. It reads the three lists in the format of T0 and prints their lengths. | The format of T0 fits the lists of T1. | Agent (T1) |
| 2026-11-11 | The interface of `internal/platform` is 9 functions: `StartTree`, `EndTree`, `NoConsole`, `FreeSpace`, `IsRooted`, `VenvBin`, `ExeName`, `PythonLauncher` and `BlockDelete`. The id of a tree is a string: the name of the Job Object on Windows, the process group number on Unix. A caller stores the id and gives it to `EndTree`. | Design rule 2: a caller sees no SysProcAttr, handle or signal. A string id can go to a file, so a later process (T7 `Backend.Stop`) can end the tree. | Agent (T2) |
| 2026-11-11 | On Windows, `StartTree` copies a handle of the Job Object into the root process. | Windows removes the name of a job when its last handle closes, also while processes of the job run. Measured: with the copy in the starter instead of the root, check 2 of the tree test fails with `2 of 2 processes alive 5 s after EndTree`. Limit: when the root ends and its children still run, the name is gone, and `EndTree` reaches nothing. For ComfyUI the root is the server, so this limit does not apply. | Agent (T2) |
| 2026-11-11 | On Windows, the root starts suspended (`CREATE_SUSPENDED`). `StartTree` puts it in the job, then resumes its thread. The standard library `syscall` with kernel32 procedures is enough; no new dependency. | The root cannot start a child before it is in the job. | Agent (T2) |
| 2026-11-11 | A tree on Windows starts with `CREATE_NO_WINDOW` and `CREATE_NEW_PROCESS_GROUP`. `horus start-comfyui` used `DETACHED_PROCESS` before. | With `DETACHED_PROCESS` the root has no console, so each console program that ComfyUI starts opens a new window. With `CREATE_NO_WINDOW` the tree shares one hidden console. The tree still gets no Ctrl+C and still lives after its starter. | Agent (T2) |
| 2026-11-11 | The `horus workflow` child of `internal/models` now starts with `StartTree`. A cancel ends the tree with `EndTree`. On Windows: `TerminateJobObject` instead of `taskkill /T /F`. On Unix: a new session instead of a new process group, and SIGTERM, a wait of at most 3 s (`EndWait`), then SIGKILL, instead of SIGKILL at once. | The plan removes `taskkill`. The process tree has one implementation. `TestCancelKillsTheProcessTree` of `internal/models` and the contract test still pass. The child is a Go program, and SIGTERM ends it at once. | Agent (T2) |
| 2026-11-11 | `StartTree` does not use `CREATE_BREAKAWAY_FROM_JOB`. The tree is a nested job in the job of its starter, if the starter is in one. | The old code did not break away either. A break-away fails when the outer job does not allow it. The tree test passes on this PC (started from a Claude Code session) and in CI on `windows-latest`. | Agent (T2) |
| 2026-11-11 | `BlockDelete` (the test helper of `TestRemoveThatCannotDeleteGivesDeleteFailed`) is in `internal/platform`, not in a new package `internal/platform/platformtest`. | A new package raised the row `layout` from 12 to 13 packages off target (measured). The ratchet must not move the wrong way. | Agent (T2) |
| 2026-11-11 | The default ComfyUI folder stays in `internal/settings` (`ComfyDir`). `internal/platform` gets only the venv paths. | `settings.ComfyDir` has no code for one OS today. T7 moves the layout of the install to `internal/backend`. | Agent (T2) |
| 2026-11-11 | The free space function of `installcomfyui` now calls `platform.FreeSpace` on the root of the drive. On macOS and Linux it gives the error of `statfs`, not "not supported on this OS yet". | The command refuses on those systems before it reads the free space, so a user sees no change. The two copies of the free space code become one, as the plan says. | Agent (T2) |
| 2026-11-11 | Check 1 of the tree test also sends SIGKILL to the process group of the ended starter on Unix, as a terminal does when it closes. A process of the tree in that group fails check 1. On Windows, check 1 has no such part. | The plan's stub check says that a start that is not detached fails check 1. Without the signal, a child on Unix lives after its parent ends also when it is not detached. | Agent (T2) |
| 2026-11-11 | `horus download-model` keeps its flag handling (`Main` in `command.go`) in `internal/download`. `cmd/horus` calls it. `internal/cli` is not created in T4. | A handler can import services and base packages only. Before T6 the only callee is the adapter `internal/download`, so a file in `internal/cli` would add an entry to `allow_direction.txt`. The list must not grow. The 13 tests of the old package call `Main` and move unchanged. T6 creates `internal/provision` and `internal/cli`, and can move the flag handling then. | Agent (T4) |
| 2026-11-11 | `internal/download` exports the constant `DefaultTokenVar`. `internal/scripts/workflow/install.go` no longer sets `TokenEnvVar`, because the downloader already defaults to the same variable. `workflow_test.go` clears the variable by `download.DefaultTokenVar`. | The token rule flags a string that names the variable. A constant of the download package names it once, in the one allowed place. The lookup (environment first, then the `.env` file) did not change. | Agent (T4) |
| 2026-11-11 | The entry `internal/scripts/workflow -> internal/scripts/downloadmodel` of `allow_direction.txt` is renamed to `internal/scripts/workflow -> internal/download`. The line of `layers.go` for the old package is deleted, because `internal/download` is already in the target table. | A rename is not a new entry. T6 removes it. | Agent (T4) |
| 2026-11-11 | `tools/goal_check.py`: the Phase 1 row `downloader` now tests `./internal/download`. | The old path does not exist. It still needs 9 passed tests (13 pass). | Agent (T4) |
| 2026-11-11 | `internal/generate` declares the interface `ComfyUI` with 9 methods: `IsUp`, `EnsureUp`, `ObjectInfo`, `Submit`, `History`, `QueueState`, `Cancel`, `View`, `Free`. `*comfy.Client` implements it. The low-level `GetJSON` and `PostJSON` of the client are not in it. The service keeps the steps: the argument checks, the graph conversion, the graph inputs, the node count and model file checks, the poll every 2 s, the time limits, the progress text, the cancel on a timeout or a cancelled call, the choice of the first image, and the PNG file. The adapter keeps the wire format: the JSON of `/history` and `/queue`, the `client_id` of a job, and the mapping of a rejected graph and of a failed job to a horus error. | Design rule 2. Before, the service read `comfy.Status` and called `comfy.JobError` itself, so it knew the shape of `/history`. Now `History` returns the job, or the mapped error of a failed job. The `client_id` was a value that the service only passed through, so the adapter makes it (`Submit(ctx, api)`). `IsUp` and `EnsureUp` stay two calls, so the service can report "starting the image backend" before a start, and T7 can replace `EnsureUp` with the backend. `QueueState` keeps its documented text form (`running`, `pending:<n>`, `absent`) and its tests. | Agent (T5) |
| 2026-11-11 | The types that the service and the adapter both need are in `internal/graph`: `API`, `Node`, `ObjectInfo` (moved), and `Job` and `Image` (new, `job.go`). `graph` also exports the helpers that the adapter uses: `PyDumps`, `PyStr`, `IsModelFile` and `Truthy` (moved from `comfy.go`). | An adapter imports base packages only, so it cannot name a type that the service declares. A Go interface needs the same named types on both sides. `graph` is the base package of the ComfyUI graph and its job. | Agent (T5) |
| 2026-11-11 | `generate.Service` also has `Running` (for `backend_running` of `list_workflows`) and `FreeMemory` (before `uninstall_workflow` deletes files). `internal/mcpserver` declares the interface `Images` (`Generate`, `Running`, `FreeMemory`), and `cmd/horus` gives it a `*generate.Service`. `internal/mcpserver` imports no adapter. | `list_workflows` and `uninstall_workflow` called `comfy.New("")` because they need the state of ComfyUI and the unload of its models. The handler gets them through a service, not through the adapter. The entry `internal/mcpserver -> internal/comfy` is gone. For T6: `FreeMemory` belongs to the remove step of `provision`, and T6 can move it there. For T7: `Running` can ask the backend. | Agent (T5) |
| 2026-11-11 | `comfy.New("")` reads `HORUS_COMFYUI_URL` at each request, not at the time it is made. | Before, every tool call made a new client and read the setting then. `cmd/horus` now makes one client at the start. The client keeps the old behaviour. `TestEmptyURLReadsTheSettingAtEachRequest` proves it. | Agent (T5) |
| 2026-11-11 | T5 creates `internal/cli` with `cli.go` (the package comment) and `run_illustration.go`. `cli.RunIllustration` gets a function `imagesAt(url)` from `cmd/horus`, because the flag `-url` picks the ComfyUI. Exported names start with `Illustration` (`IllustrationOptions`, `IllustrationResult`) so that later commands in the same package do not collide. | T4 did not create the package (see the T4 row above). `run-illustration` calls the service `generate`, so its handler adds no entry. | Agent (T5) |
| 2026-11-11 | Tests that need the wire format moved to `cmd/horus`, the wiring: `TestGenerateImageTool` and `TestGenerateImageToolErrors` (from `internal/mcpserver`, through `mcpserver.New(newImages(""))`), and 7 tests of `run-illustration` (`run_illustration_test.go`, through `cli.RunIllustration` with `imagesAt`). 3 new tests there prove that the parts fit on the timeout, the cancel and out of memory. The 12 tests of `internal/generate` keep their names and cases, on a fake of `ComfyUI` (`fake_test.go`). `internal/mcpserver` gets 2 handler tests on a fake `Images`. `TestFlagErrors` moved to `internal/cli`. | A test file follows the rules of its package (T1 decision). `cmd/horus` can import every package. Each behaviour that a test proved stays proved: the parts that the adapter now does (the mapping of a failed job, the backend time, a cancelled call) got tests in `internal/comfy`. | Agent (T5) |
| 2026-11-11 | `tools/goal_check.py`: the Phase 1 row `graph` now tests `./internal/graph`. The Phase 1 row `setup-commands` finds `runillustration` in `internal/cli` (`SETUP_PACKAGES`). | The old paths do not exist. The rows prove the same things. | Agent (T5) |
| 2026-11-11 | Issue #75: horus itself does not send a progress notification late. The fix is in the tests only. A test that reads collected notifications waits for them on a channel with a time limit. It proves "none without a token" with a third call that has a token, because the client runs its handlers in the order of arrival. | Server: `session.NotifyProgress` calls `handleNotify`, which calls `Connection.Notify`, and that call writes the message before it returns. The tool handler returns after that, and only then the SDK writes the result. So the notification is on the wire before the result. Client: `readIncoming` gives a response straight to the waiting `CallTool`, and gives a notification to a queue that `handleAsync` runs in a new goroutine (`jsonrpc2/conn.go:625-690` of go-sdk v1.8.0). So `CallTool` can return before the handler runs. A sleep or a retry would not fix this. | Agent (#75) |
| 2026-11-11 | `internal/modelfiles` is a `Store` with 8 methods: `ComfyUI`, `Dirs` (the folder order), `Own`, `Mapped`, `Map` (`extra_model_paths.yaml` with one backup), `Size`, `SHA256` and `Delete` (also a read-only file). An empty field reads the setting at each call, and the yaml file is read at each call, as each `horus workflow` child did. `internal/provision` declares the interface `ModelFiles` with the same methods. | Design rule 4. The interface uses only built-in types, so the adapter implements it without a shared type. | Agent (T6) |
| 2026-11-11 | The refusal rules of a remove (another size: REFUSED; another workflow lists the file: KEPT_SHARED unless Force) are pure functions of `internal/provision` (`removeDecision`), with the checks of a file (`fileCheck`, `graphCheck`, `serverCheck`) and the choice of the yaml folders (`mappings`). `modelfiles.Delete` only deletes. | The task says that the decisions are the knowledge of the service. "Remove with the refusal rules" of the plan is read as: the adapter deletes, the service refuses. | Agent (T6) |
| 2026-11-11 | The reader of the workflows folder with the rules of workflow.ps1 (keys without case, the casts of the script, the order of the keys) moved from `internal/scripts/workflow/files.go` to `internal/catalog/workflowdir.go` (`Folders`, `FindModelList`, `ReadModelList`, `ReadGraphRefs`). It is a second reader of models.json beside `catalog.Models`, with its own error texts. | The service then does its input and output only through its interfaces or the base package `catalog`, as `generate` reads the workflows through `catalog`. The two readers keep their own errors, because the parity golden files and the MCP contract hold both. | Agent (T6) |
| 2026-11-11 | `internal/provision` declares `Downloader`, `Fetch`, `DownloadError` and `DownloadKind` (6 kinds, the same as `download.Kind`). The glue type `downloader` in `cmd/horus/wiring.go` calls `download.Download` and copies the kind of a `*download.Error` into a `*provision.DownloadError`. | A service cannot import an adapter, and an adapter cannot name a type of a service. No base package fits a download error. The glue is 20 lines of the wiring, and `cmd/horus/workflow_test.go` proves it for HTTP 401, HTTP 403, a full disk and a file conflict. | Agent (T6) |
| 2026-11-11 | `internal/provision` declares `ComfyUI` with one method, `ObjectInfo`. `*comfy.Client` implements it. Two small differences from the old check: the time limit of `/object_info` is 60 s (the limit of the client), not 30 s; and an answer that is JSON but not an object counts as a server with no node class and no file, also when the answer holds model names. An answer that is not JSON gives the old result (each node class NO_NODE, each file NOT_SEEN). | One client of ComfyUI, no second HTTP code. A real ComfyUI always answers an object, and no test or golden file reaches the two cases. | Agent (T6) |
| 2026-11-11 | The service returns a typed `Result`. Its JSON fields are the `HORUS_JSON` line, unchanged. Fields with `json:"-"` (`Want`, `Count`, `Users`, `Part`, `Err`, `Checked`, `CheckFiles`, `Quick`, `Mappings`) carry what the handler needs for the text lines and what the MCP tools need for the error codes. The lines of the downloader go live to `Watch.Out`; `Watch.Started`, `Watch.Downloading` and `Watch.Failed` give the other live lines of an install. `internal/cli/workflow.go` writes every line. | The service knows no output text (spec, "Service"). The parity test proves the text: `parity: 17 of 17 cases equal`. | Agent (T6) |
| 2026-11-11 | A cancel stops an install after the download that runs: no next download, no change to `extra_model_paths.yaml`, and the error is the error of the context. The `.part` file stays. In `horus workflow install`, Ctrl+C now ends with the line `FAIL: context canceled`. Before, each remaining file was tried and failed at once, and the last line was `FAIL: <n> of <m> file(s) are not ready.` | The spec decision: a cancel ends the download through the context. Without the stop, the service tries every remaining file after the cancel, and a present file gets a full SHA-256 check first (minutes for 10 GB). This is the only difference that a command-line user can see, and only after Ctrl+C. | Agent (T6) |
| 2026-11-11 | The error code of a failed download comes from the kind of the error (`HF_TOKEN_INVALID` for 401, `LICENSE_NOT_ACCEPTED` for 403, `DISK_FULL`, `FILE_CONFLICT`, else `DOWNLOAD_FAILED`), not from the text `error: 401` in any line. The kind is the status of the last attempt (the rule of `internal/download`). The details (`file`, `path`, `detail`, and the license for 403) are as before. | The task asks for the mapping on typed values. A run whose earlier attempt got 401 and whose last attempt got another error now gives `DOWNLOAD_FAILED`. | Agent (T6) |
| 2026-11-11 | When the list of the workflows fails in a tool, the error is still `INTERNAL` "horus workflow list failed". Its detail `output_tail` is now `["FAIL: <error>"]`, the line that the child printed. The error "horus workflow ... printed no result" no longer exists: there is no child and no line to parse. | The server starts no child (spec). | Agent (T6) |
| 2026-11-11 | The server resolves a relative `HORUS_MODELS_DIR` or `HORUS_TEXT_ENCODER_DIR` against its own working folder. The child of `internal/models` ran in `settings.Root()`. | The settings are absolute paths in `docs/setup.md` and in both test tools. The in-process service cannot change the working folder of the server. | Agent (T6) |
| 2026-11-11 | `FreeMemory` stays in the tool `uninstall_workflow` (through `Images`), not in `provision.Remove`. | `horus workflow remove -yes` does not unload the models today. In the service, it would change the command. | Agent (T6) |
| 2026-11-11 | `horus download-model` keeps its flag handling (`Main`) in `internal/download`. | Its flag `-token-env-var` names the token variable, with the default `HF_TOKEN`. In `internal/cli` that string breaks the rule `token`, and a variable name that passes through `provision` needs an exported option of the downloader: the loophole that this task closes. A method of `provision` that only passes the call to the downloader hides nothing (design rule 1). | Agent (T6) |
| 2026-11-11 | `internal/download` exports no name of the token variable: `DefaultTokenVar` became `defaultTokenVar`, and the option `TokenEnvVar` became the field `tokenVar`, which only `Main` (the flag `-token-env-var`) and the tests of the package set. The rule `token` of archtest also flags a selector of the download package whose name has "token" in it, outside `internal/download`. Two planted files prove it: `cmd/horus/token_name.go`, and `cmd/horus/token_alias.go` with an import alias. | Before, another package could read the token with `os.Getenv(download.DefaultTokenVar)`, and no string named `HF_TOKEN`. The tests outside the package need no token: the fake servers are not huggingface.co, so the downloader sends no token, and `Root` points at a temporary folder. | Agent (T6) |
| 2026-11-11 | The tests moved. The 6 command tests of `internal/scripts/workflow/workflow_test.go` and `TestModelToolsOnASandbox` went to `cmd/horus/workflow_test.go`, because they need the real model files adapter and the real downloader. `TestDownloadFailuresKeepTheTextThatModelsPyReads` became `TestDownloadFailuresKeepTheirTextAndTheirKind` (it also checks the kind). `TestInstallToolGivesTheCodeOfAFailedDownload` is new: 401, 403, a full disk and a conflict through `install_workflow`. `TestYAMLFolders` went to `internal/modelfiles`. `TestNumbersAsTheScriptPrintsThem` became `TestGB2AsTheScriptPrintsIt` (`internal/cli`) and `TestRoundGBAsTheScriptRoundsIt` (`internal/provision`). Of `internal/models`: `TestInstallMapsEachStatusToItsCode` and `TestRemoveMapsEachStatusToItsCode` test the mapping in `internal/mcpserver` on typed items; `TestListStatusAndInstallState` became `TestRowState` and `TestListCountsTheFilesOnDisk`; `TestPartSize` went to `internal/provision`; `TestCancelKillsTheProcessTree` became `TestInstallStopsWhenTheContextEnds` (service) and `TestInstallReportsProgressAndACancelStopsTheDownload` (`internal/mcpserver`, the test of the plan). `TestRunScriptReadsTheLinesAndTheResult` and `TestNoResultLineIsInternal` are gone: the child process and the parser of its line do not exist. `TestSizeText` stays in `internal/mcpserver`. | A test follows the rules of its package: the tests of `provision` and `mcpserver` use fakes of the adapters (`memFiles`, `diskFiles`, `slowDownloader`). The Go test functions went from 174 to 185. | Agent (T6) |
| 2026-11-11 | The tests of `internal/mcpserver` that call no model tool give `New` a nil `Setup`. | They prove that the image tools and the envelope need no setup service. | Agent (T6) |
| 2026-11-11 | `internal/backend` declares `Backend` with 5 methods: `Install(ctx, out)`, `Start(ctx, out)`, `Stop(ctx, out)` (each returns an error and writes its text lines to `out`), `Installed() (dir, ok)` and `Layout() Layout` (`Dir`, `ComfyUI`, `Python`, `Logs`). No pid, no job name and no archive is in a signature. | The result of the Mac test run (#53, PASS) is the requirement of the second backend: `comfy --workspace <dir>/ComfyUI install --m-series --version 0.44.0`, `... launch --background -- --listen 127.0.0.1 --port 8188`, `... stop`, `main.py` and `models` in `<dir>/ComfyUI`, Python in `<dir>/ComfyUI/.venv/bin/python`, the log `<dir>/ComfyUI/user/comfyui_8188.log`. Each fact fits one method or one field of `Layout`, so the comfy-cli backend of Phase 3 needs no change of the interface. The archive, 7-Zip and the state file are fields and files of the Windows backend only. | Agent (T7) |
| 2026-11-11 | The Windows backend `Portable` compiles on every OS. `backend.New` gives `*Portable` on Windows and else a value that embeds it and answers `platform.ErrNotSupported` ("not supported on this OS yet", a new variable of `internal/platform`) for `Install`, `Start` and `Stop`. `Installed` and `Layout` work on every OS. | The tests of the old packages ran the portable code with a fake `python.exe` and a fake `7z.exe` on Windows, macOS and Linux, and they still do. On macOS, `horus workflow install` still finds a ComfyUI in `HORUS_COMFYUI_DIR` (the parity test runs on `macos-latest`), because the layout `<dir>/ComfyUI/main.py` is the same for both backends. | Agent (T7) |
| 2026-11-11 | `Start` writes `horus-comfyui.state.json` next to the two log files: the tree id of `platform.StartTree`, the pid of the root, the address and the time. `Stop` reads it. Without a state file for the same address, `Stop` ends nothing: when a server answers there, it refuses with exit 1 ("horus did not start the ComfyUI that answers at ..."); when nothing answers, it says "Nothing to stop" with exit 0. With a state file, it calls `platform.EndTree`, deletes the file, and waits up to 15 s until the server no longer answers. A state file whose tree has ended (ComfyUI ended by itself, the PC restarted) gives "was not running any more", or, when another server answers at the address, an error that horus did not start it. `Start` deletes the file when ComfyUI stops during the wait. | The state must live after the end of the starting process (the MCP server). On Windows the tree id is the random name of a Job Object, and Windows removes the name with the last process of the job, so an old id can reach no other process. Limit: on Unix the id is a process group number, which the system can give again after a restart. The Unix backend answers "not supported" in Phase 2, and the comfy-cli backend of Phase 3 stops ComfyUI with `comfy stop`. | Agent (T7) |
| 2026-11-11 | `internal/generate` declares `Backend` with one method, `Start(ctx, out)`. The service decides the autostart: when `IsUp` is false it reports "starting the image backend", reads `HORUS_AUTOSTART`, calls `Start` one time, and checks `IsUp` again. A failed start is `BACKEND_NOT_INSTALLED` when the error has a method `NotInstalled() bool` that returns true (the error of `backend.Start` for a folder without `python_embeded\python.exe` or `ComfyUI\main.py`), else `BACKEND_UNAVAILABLE`. `start_output_tail` holds the last 8 lines of the text of the start and its error. The interface `ComfyUI` loses `EnsureUp` and gets `Address()` for the text of the errors. `internal/comfy` starts no program: `os/exec`, `os` and `internal/platform` are gone from it. | The task: the adapter does not start programs, the service or the wiring decides. The code is a typed test of the error, not the old text match on "was not found". Three small differences, none in a test or a golden file: the lines of the tail no longer start with `horus start-comfyui: `; the limit of 420 s of the child process is gone, so the 300 s wait of `Start` is the limit; a call that is cancelled during the start gives the error of the context, as every other step of the service does. | Agent (T7) |
| 2026-11-11 | `internal/provision` declares `Backend` with one method, `Installed() (dir, ok)`, and the install check of `horus workflow install` uses it. `ModelFiles.ComfyUI` and `modelfiles.Store.ComfyUI` are gone. `provision` does not declare `Install` yet. | One source of truth: only `internal/backend` decides if ComfyUI is installed. The install folder comes from one place for both adapters: the field of the flag `-comfyui-dir`, else `settings.ComfyDir()`. `modelfiles` still joins `<dir>/ComfyUI/models` and `<dir>/ComfyUI/extra_model_paths.yaml`: that is the layout of ComfyUI itself, the same in both backends (#53). In T7 no code of `provision` calls `Install`. T8 (`horus setup`) adds it to the interface when its plan needs it (design rule 1). | Agent (T7) |
| 2026-11-11 | The handlers of `install-comfyui`, `start-comfyui` and the new `stop-comfyui` are in `internal/cli/comfyui.go`. `internal/cli` declares `Backend` (`Install`, `Start`, `Stop`) and `BackendPlace` (the flags), and `cmd/horus` gives `backendAt`, as it gives `setupAt` and `imagesAt`. Exit codes: 0 done, 1 a failed step (a refusal of `stop-comfyui` too), 2 a bad flag or "not supported on this OS yet". | A method of `provision` that only passes the call to the backend hides nothing (design rule 1, the T6 decision on `download-model`). The handler imports no adapter: the import rules hold. | Agent (T7) |
| 2026-11-11 | The flag `-server-url` of `horus start-comfyui` now defaults to `HORUS_COMFYUI_URL` (then `http://127.0.0.1:8188`), not to a fixed `http://127.0.0.1:8188`. `stop-comfyui` has the same default. | The autostart always gave the child `-server-url` with the setting, so the port of an autostart followed `HORUS_COMFYUI_URL`, and it still does. The command alone did not. Without the change, `HORUS_COMFYUI_URL=...:8189 horus stop-comfyui` and `start-comfyui` would act on different ports. Without the variable, the output is the same as before. | Agent (T7) |
| 2026-11-11 | The tests moved. The 14 install tests and 5 start tests of the old packages are in `internal/backend` (`install_test.go`, `start_test.go`; the install rig is `installRig`). The 2 `TestNotSupportedOffWindows` and 2 `TestBadFlagExits2` became `TestComfyUINotSupportedOffWindows` and `TestComfyUIBadFlagExits2` in `internal/cli`, each for the three commands. Of `internal/comfy`: `TestEnsureUpAutostartOff` is the message check in `TestBackendDownAutostartOff`; `TestEnsureUpWhenUpStartsNothing`, `TestEnsureUpStartFails` and `TestEnsureUpStartsTheServer` became `TestUpBackendStartsNothing`, `TestStartOfAComfyUIThatIsNotInstalled`, `TestFailedStartKeepsTheLastEightLines` and `TestDownBackendStartsOneTime` (`internal/generate`, on a fake backend); `TestStartCommandIsHorusStartComfyUI` became `TestPortFollowsTheSetting` (`internal/backend`); `TestHelperStart` is gone (there is no child). The install check of `TestDirsInTheOrderOfTheSpec` (`internal/modelfiles`) is in `TestInstalledAndLayout`. New: `TestStopEndsTheTreeThatStartStarted`, `TestStopRefusesAComfyUIThatHorusDidNotStart`, `TestStopWithAStaleState`, `TestOtherOSAnswersNotSupported`, `TestComfyUIOutputAndExitCodes`, `TestComfyUIFlagsPickThePlace`. | Go test functions 185 -> 189. | Agent (T7) |
| 2026-11-11 | `tools/goal_check.py`: with `--real`, the row `backend` also needs `started_backend: true` in the result, so an image from a ComfyUI that already answered does not pass. A new flag `--only <row...>` checks only the rows named. The Phase 1 row `setup-commands` finds `installcomfyui` and `startcomfyui` in `internal/backend`. | The row proves "after a start through the backend". The real run of this PC runs only the row `backend` (`--only backend`): the other rows run `go test ./...` in the environment of the real run (`HORUS_ROOT` of the main checkout), and that is not needed to prove this row. | Agent (T7) |
| 2026-11-11 | The real run of the row `backend` uses a second ComfyUI on port 8189, from a second install folder that reads the owner's install through junctions (`python_embeded` and each folder of `ComfyUI`, except its own `user`, `input`, `output` and `temp`; the 36 small files of the `ComfyUI` folder are copies). The owner's ComfyUI on port 8188 keeps running. | The owner's ComfyUI is the owner's app: horus must not stop it (standing rule: `Backend.Stop` ends only a ComfyUI that horus started). A second ComfyUI 0.44 from the same folder does not start: "Another ComfyUI is already using this database: '<ComfyUI dir>\ComfyUI\user\comfyui.db'" (first run below). The second folder gives the second ComfyUI its own database. The backend runs the same command line in it, through the same code; only `HORUS_COMFYUI_DIR` and `HORUS_COMFYUI_URL` differ. | Agent (T7) |
| 2026-11-11 | The Green of T8 is read as: `python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin> mcp` prints PASS for `layout`, `same-behaviour`, `os-boundary`, `direction`, `process-tree` and `token` (6 of 8), and `python tools/goal_check.py --phase 2 --no-ci --real --only commands --server-cmd <bin> mcp` prints `PASS  commands` on this PC. The row `backend` with `--real` is for T10. | The plan says `goal: 8 of 8 rows passed` without `--real`, but the decision of T0 makes `backend` and `commands` pass only with `--real`. The two commands prove the same rows. Standing rule: a plan command that does not fit the tool gets the equivalent command. | Agent (T8) |
| 2026-11-11 | `internal/doctor` is a `Service` with 4 interfaces that it declares: `Server` (`Version`, the answer of `/system_stats`; `*comfy.Client` has the new method `Version`), `Install` (`Installed`, the backend), `Programs` (`LookPath`, `platform.Programs`) and `Models` (`Check` of one workflow). `Check` returns one typed `Result` for each check, and `internal/cli/doctor.go` writes the lines. The check "ComfyUI folder" asks the backend, the one place that knows the layout (T7 decision). The OS facts (`goos`, the venv folder, `.exe`) come from `platform.VenvBin` and `platform.ExeName`. The service still reads `.mcp.json` and the venv files itself. | Design rules 2 and 4. The spec bans `net/http` and `os/exec` in a service, not file reads; `devtools` also reads and writes files. The last entry of `allow_os-boundary.txt` (`checksetup.go`, `runtime.GOOS`) and the 4 entries of `allow_direction.txt` are gone. | Agent (T8) |
| 2026-11-11 | The model check of `horus doctor -workflow` runs `horus workflow check` in the same process: the glue `workflowCheck` in `cmd/horus/wiring.go` calls `cli.Workflow` with `check <name> [-quick]` and gives its text lines. Before, `checksetup` started the binary again as a child process with `-json` and the folders in environment variables. | A service cannot import a handler or `os/exec`, and the spec decision of this phase is "no child process for the setup service". The lines are the same: `check-setup -workflow it-illustration -quick` of the old and the new binary on this PC give equal output (`diff` prints nothing). The case "the child does not start" no longer exists. | Agent (T8) |
| 2026-11-11 | `internal/platform` gets `Programs` (`LookPath`, and `Run(ctx, Command)` that returns the exit code) and the type `Command`. `provision` declares `Programs` with both methods, `doctor` declares only `LookPath`. No new adapter package. | One "run a program" for the three users (doctor, comfy-mcp, register). The spec puts processes in `internal/platform`. A package that only wraps `os/exec` hides nothing more (design rule 1). `Command` is in the base package, so a service can name it in its interface. | Agent (T8) |
| 2026-11-11 | `provision.InstallComfyMCP` and `provision.Register` are functions that take a `Programs`, not methods of `provision.Service`. | The `Service` holds the model files, the downloader, ComfyUI and the backend; these two steps use none of them. A field that only two methods use, and a longer `New`, add nothing. | Agent (T8) |
| 2026-11-11 | `InstallComfyMCP` writes its step lines ("Creating the venv ...", "SKIP: ...", "OK: ...") to its writer itself. | Its lines come between the lines of `python -m venv` and `pip`, which go to the same writer while they run, as the lines of `Backend.Install`. A typed result cannot keep that order. Every other new service returns typed results. | Agent (T8) |
| 2026-11-11 | `horus setup`: `provision.Plan` reads the facts (the backend `Installed`, and for each file of models.json the path and a size check, no SHA-256) and a pure function `steps` makes the steps: 1 backend step when ComfyUI is missing, then 1 file step for each file that is not there with its size. `provision.RunPlan` (only with `-yes`) calls `Backend.Install`, then `Install` with `Only` set to the file names of the plan. A plan with only a backend step downloads nothing. `provision.Backend` gets `Install` (T7 decision). The exit code is 0 when the plan is printed (with or without steps) or the run is done, 1 for an error, 2 for a flag or an argument. | The plan must be quick and read-only, so it checks the sizes as `horus workflow list` does. The run does what the plan says and no more. The SHA-256 of the present files is the job of `horus workflow check` and `horus doctor -workflow`. | Agent (T8) |
| 2026-11-11 | The text of the plan: `PRESENT ` or `MISSING ` and `ComfyUI in <dir>`, one line for each file (`<role> <file> (<GB> GB) <path>`), then `Plan: 0 steps. Nothing is missing.` or `Plan: <n> step(s):` with numbered steps and `Nothing changed. Run horus setup <workflow> -yes to run the plan.` | The goal tool reads the number of steps, the words "nothing is missing", and the paths. | Agent (T8) |
| 2026-11-11 | `devtools.Sync` returns typed copies (`Copied`, `Same`, `Replaced`, `Conflict`), and `internal/cli/devtools.go` writes the lines. Its check for ComfyUI asks the backend (`Install.Installed`) instead of its own `main.py` test. `devtools.NewWorkflow` is the old `Create`. | Design rules: a service knows no output text; one place knows the layout. The lines of `sync-workflows` are the same (`TestSyncWorkflowsLinesAndExitCodes` compares them exactly). | Agent (T8) |
| 2026-11-11 | The handlers of `horus errors` and `horus mcp` moved to `internal/cli` (`cli.Errors`, `cli.MCP` with a `serve` function from the wiring). `cmd/horus` is now `main.go` (the command table) and `wiring.go`. `download-model` keeps `download.Main` (T6 decision). `check-setup` and `doctor` are one handler, `cli.RunDoctor`, with two names; both show the same summary in `horus -h`. | Plan T8 step 4. `cli.MCP` cannot import `internal/mcpserver` (a handler imports no handler), so the wiring gives the server as a function. | Agent (T8) |
| 2026-11-11 | archtest has no allow list: `Compare` and `ParseAllow` became `Problems` (every break fails `TestRules`), and `TestCompare` became `TestProblems`, which also fails when a file `allow_*.txt` exists in `internal/archtest`. The 5 Phase 1 lines of `layers.go` are gone. The goal tool rows `os-boundary`, `direction` and `token` pass when archtest passes and no allow list file exists. | Plan T8 step 5: a new entry must not be possible. The planted-file test (`TestPlanted`) and the rules did not change. | Agent (T8) |
| 2026-11-11 | The goal tool row `commands` with `--real` runs `horus setup it-illustration`, reads the paths of its plan (each model file, and `ComfyUI/main.py` and `ComfyUI/extra_model_paths.yaml`), keeps their size and time, runs `horus doctor`, runs `horus setup it-illustration` again, and compares. It fails on a FAIL line or a non-zero exit of doctor, on a plan with a step, on a changed file, or when the two plans differ. The Go tests of `./internal/provision` also run in the row. The Phase 1 row `setup-commands` finds the moved commands in `internal/doctor`, `internal/provision` and `internal/devtools`. | T0 left the row without the file check. Limit: the first run of `setup` is not inside the comparison; `TestSetupWithoutYesChangesNoFile` (cmd/horus) and `TestPlanOfAnEmptyMachine` (provision) prove that a plan without `-yes` writes nothing. | Agent (T8) |
| 2026-11-11 | `tools/phase2_packages.txt` and the layer table of the spec do not change: `go list ./...` gives the 18 listed packages. | Measured: `PASS  layout - 0 packages off target, 0 listed packages missing`. | Agent (T8) |
| 2026-11-11 | The tests moved. `checksetup`: 7 tests went to `internal/doctor` on fakes (`TestModelsLinesAreIndentedAndFailureCounts` became `TestModelsLinesAndFailureCount`; the indent is in `TestDoctorLinesAndExitCode` of `internal/cli`), `TestRunRejectsAnUnknownFlag` and `TestRunModelCheckIsFullByDefault` went to `internal/cli` (`TestDoctorRejectsAnUnknownFlag`, `TestDoctorModelCheckIsFullByDefault`), and the 2 `TestExecModels...` tests became `TestDoctorThroughTheWiring` and `TestWorkflowCheckGivesTheLinesAndTheExitCode` (`cmd/horus`, on the sandbox of `workflow_test.go`). `installcomfymcp`: 5 tests to `internal/provision`, `TestBadFlagExits2` to `internal/cli` (`TestInstallComfyMCPBadFlagExits2`). `register`: 4 tests to `internal/provision`, the 4 `TestRun...` tests to `internal/cli` (`TestRegisterFindsClaudeAndPassesTheFlags`, `TestRegisterWithoutClaude`, `TestRegisterWithoutARepoFails`, `TestRegisterTakesRootFromTheSettings`) on a fake `Programs`, and `TestRegisterFindsClaudeOnThePath` (provision) runs the real PATH. `newworkflow` and `syncworkflows`: 7 tests to `internal/devtools`, `TestRun` and `TestRunExitCodes` to `internal/cli`. New: `TestCheckChangesNoFile`, `TestVersion`, `TestRunGivesTheExitCodeAndTheOutput`, 6 plan tests of `internal/provision`, 5 tests of `horus setup` in `internal/cli`, `TestSetupWithoutYesChangesNoFile`, `TestInstallComfyMCPReportsAFailure`. | A test follows the rules of its package: the tests of `doctor` use fakes instead of `httptest`, and the tests of `provision` and `devtools` use the base package `platform` or fakes. Go test functions 189 -> 207. | Agent (T8) |
| 2026-11-11 | T9: the Red of the plan (`git grep -n -E "internal/(scripts|server|models)" -- '*.md' ':!docs/plans' ':!docs/specs'`) already printed nothing on `origin/main`. The Red of T9 is the same search with the stale texts that the docs still held: `horus workflow \(child\)`, `scripts/<command>`, `horus start-comfyui" does` and `start "horus workflow`. It printed 7 lines on `origin/main`. The Green runs both commands and both print nothing. | The earlier tasks had already removed the old package paths from the text, but the layout list of `AGENTS.md` and the three diagrams of `docs/mcp.md` still described the Phase 1 design. A Red that prints nothing proves nothing (the empty-implementation test). Standing rule: use the equivalent command that proves the same goal. | Agent (T9) |
| 2026-11-11 | T9: `docs/mcp.md` says that the token is in the memory of `horus mcp` while an install runs, and that only `internal/download` reads it (environment first, then the `.env` file in the repo root). Before, it said that the server never reads `HF_TOKEN`. | After the decision on the process boundary, the download runs in the server process. The old sentence was false. D7 of #4 (only the download code reads the token) stays true. | Agent (T9) |
| 2026-11-11 | T9: `docs/setup.md` tells that `horus setup <workflow>` stops at the next step on Ctrl+C and that an install ends with `FAIL: context canceled`, that ComfyUI 0.44 locks its database so one install folder cannot run two ComfyUI, and that a PC can keep the model files in a folder that `extra_model_paths.yaml` maps. It tells a reader to use `horus doctor` and `horus setup`, and says that the old names still work. Every `horus check-setup` in the docs outside the plans and specs became `horus doctor`, except the sentences that name the old name. | Plan T9 step 3, and the decisions of T6, T7 and T8 that a reader sees. | Agent (T9) |
| 2026-11-11 | The rows `os-boundary`, `direction` and `token` need `--- PASS` for `TestRules`, `TestPlanted` and `TestProblems` in `go test -v -run` of `internal/archtest`. The rows `backend` and `commands` need that each of their packages has a test file and a passing test, before they ask for `--real`. | A package with no test file exits 0, so the exit code alone passes an empty implementation (issue #80). The three names are the tests that enforce the rules and prove them with planted bad files. | Agent (#80) |
| 2026-11-11 | The row `same-behaviour` needs these jobs of the newest `ci.yml` run on HEAD to be `success`: `contract` and `workflow-parity` on `windows-latest` and `macos-latest`, `go windows/amd64` and `go darwin/arm64`. The row `process-tree` needs the two `go` jobs. A job that is missing, running, cancelled or failed fails the row and the detail names it. `go linux/amd64` is not required, because no oracle names it. `--no-ci` skips all of this. The row `process-tree` also needs `TestTreeOutlivesItsStarterAndEndsFromAnotherProcess` (check 1 and check 2) to pass, not any test with `Tree` in its name. | The oracle of each row names these jobs (issue #81). A stub test `TestTree` that does nothing passed the old row. The pinned name is the one test that holds the two checks of the spec. | Agent (#81) |
| 2026-11-11 | T10: the plan Green (one command `python tools/goal_check.py --phase 2 --real`) is replaced by three runs on the same commit `9699bb8` with the same binary: all rows with the CI part (6 PASS), `--real --only commands` with the settings of the owner (PASS), and `--real --only backend` with the settings of the port 8189 method (PASS). | The owner's ComfyUI ran on port 8188, so the row `backend` needs `HORUS_COMFYUI_DIR` = a folder that links to the owner's install and `HORUS_COMFYUI_URL` on port 8189 (T7 decision). One command gives the same environment to every row, so `go test ./...`, the contract test and the parity test of `same-behaviour` also run with these settings and with `HORUS_ROOT` = the main checkout. The standing rule "never touch the model files of the owner's PC" weighs more than one command line. The three runs prove the same rows (standing rule: the equivalent command). | Agent (T10) |
| 2026-11-11 | T10: the checks of `os-boundary`, `direction` and `token` pass on an empty `internal/archtest` (measured: `goal: 3 of 3 rows passed` with one file `package archtest`). Filed as #80, which blocked T10. PR #83 fixed it; on `9699bb8` the same stub gives `goal: 0 of 3 rows passed` (`## T10`). | The spec oracle is "with planted bad files". The tool read only the exit code of `go test`, and a package with no test exits 0. | Agent (T10) |
| 2026-11-11 | T10: the CI part of the goal tool reads only the jobs `contract <os>`. It does not read `workflow-parity <os>` or `go darwin/arm64`, so a red parity or tree test on macOS still gives PASS for `same-behaviour` and `process-tree`. Filed as #81, which blocked T10. PR #84 fixed it; on `9699bb8` a red parity job and a missing `go darwin/arm64` job fail the rows (`## T10`). | The spec oracle of both rows names CI on `windows-latest` and `macos-latest`. | Agent (T10) |
| 2026-11-11 | T10: three limits of the checks are accepted, not filed. `layout` passes on an empty package with the right name: the ratified oracle is `go list` against the list, and the rows `direction` and `same-behaviour` prove where the code is. `backend` does not judge the content of the PNG: the agent opens the PNG and judges it, as in T7. `commands` runs the Go tests of `internal/cli`, `internal/doctor` and `internal/provision`, not of `cmd/horus` (`TestSetupWithoutYesChangesNoFile`, the old command names): `go test ./...` of `same-behaviour` runs them, and `8 of 8` needs both rows. | Each limit is covered by another part of the same tool or by the visual check of the ratified oracle. | Agent (T10) |
| 2026-11-11 | T10: `horus doctor` checks `http://127.0.0.1:8188` also when `HORUS_COMFYUI_URL` names another port. The old `check-setup` did the same. Filed as #82, a follow-up outside the epic. | It threatens no goal row: the behaviour did not change in Phase 2. | Agent (T10) |
| 2026-11-11 | T10: the real run of the row `backend` on `9699bb8` started after 6 GPU readings (18:37 to 19:28) that were not quiet: 472 to 910 MiB, 37 to 57 %. At 19:28 the game process had ended (the GPU apps list had only its client window), and the reading was 483 MiB, 28 %. The run passed at once, so there is no second run to record. | Standing rule: wait for a free GPU, at most 6 readings of 10 minutes (task brief). The memory was below 1.5 GB in every reading. The process that used the GPU had ended. A failed run gets one more run, and none was needed. | Agent (T10) |
| 2026-11-11 | Issue #90: `QueueState` returns `graph.Queue{Running bool; Position int}` with the methods `Pending()` and `Absent()`, declared in the base package `internal/graph` next to `graph.Job`. The `ComfyUI` interface of `internal/generate` keeps its 9 methods. | The type is a pure value that both sides may import (the import rules allow `graph` for both). No wiring change is needed in `cmd/horus`. All 9 methods are still called by the service, so none is removed. The progress texts are built from the same words, so the output is the same. | Agent (#90) |
| 2026-11-11 | #86: the handler `cli.RunDoctor` runs the model check. It calls the doctor service for the checks 1 to 7, then (with `-workflow`) `Check` of the setup service through `cli.Setup`, and writes the lines with `printer.check` of `horus workflow` and an indent of 6 spaces. `internal/doctor` lost `Models`, `Options.Workflow`, `Options.Quick` and `Result.Lines`. `doctorAt` gives the two services, and the glue `workflowCheck` is gone. This replaces the T8 decision on the glue. | The fix of the issue, and its Verify block names a test of `RunDoctor` on a fake `Setup`. The handler sees both services, so no service gets text and the wiring holds no logic (design rule 5). A doctor interface for a typed model check would only move the same composition into a service and hide nothing (design rule 1). One printer writes the check lines of both commands, so the lines stay equal. | Agent (#86) |
| 2026-11-11 | #86: the detail `cannot run the model check: <error>` is gone, with its case in the test. A failed `Check` of the setup service gives `FAIL: <error>` (indented) and `FAIL  models for '<name>' - see lines above. Fix: horus workflow install <name>`, as before. | The old glue always returned a nil error, so the program never printed that detail. The output of the binary does not change (`diff` of 6 cases prints nothing, `## Issue 86`). | Agent (#86) |
| 2026-11-11 | #86: the tests moved. `TestModelsLinesAndFailureCount` and `TestNoWorkflowFlagMeansNoModelCheck` (`internal/doctor`) became `TestDoctorModelCheckLinesAndExitCode` and `TestDoctorWithoutWorkflowRunsNoModelCheck` (`internal/cli`, on a fake `Setup`). `TestWorkflowCheckGivesTheLinesAndTheExitCode` (`cmd/horus`, the glue) became `TestDoctorModelCheckOfAnUnknownWorkflow`. `TestDoctorThroughTheWiring` runs `RunDoctor` on `doctorAt`. | A test follows its code. Go test functions 207 -> 207. | Agent (#86) |
| 2026-11-11 | #88: the pure function `provision.InstallState(present, files int) string` owns the rule of `install_state`. `Row.State()` (for `list_workflows`) and the handler of `describe_workflow` both call it. The handler still counts the files with the state `present` from the rows of its check. | The fix of the issue: one rule has one place (design rule 1), and the service owns its decisions. The name of the issue fits, and a function with two integers is the smallest interface for both callers. Counting the rows that the handler shows is mapping of the typed result, not a rule. | Agent (#88) |
| 2026-11-11 | Issue #87: `download.Error` gets `Reasons`, the lines that `Download` writes for a failed attempt (`Attempt <n> of <m> did not finish ...`) and the hint line after an HTTP error, in order. The glue `downloader` in `cmd/horus/wiring.go` copies them to `provision.DownloadError.Reasons`, as it copies `Kind`. `provision.Install` builds the detail with the pure function `failDetail` (the reasons, then `FAIL: <error>`, joined by ` \| `). `whyLine` and `lineTee` are gone, and the service gives `Watch.Out` to the downloader as it is. | The service must not parse the text of an adapter when a typed value can cross the interface (design rules 2 and 4). The reasons are exactly the lines that `whyLine` picked: no other line of `Download` starts with `FAIL:`, `Attempt ` or `The server answered with an error`, and the `FAIL:` part comes from the error, as before. The lines on `Watch.Out` do not change: `Download` writes each line with one newline, and `lineTee` wrote the same lines again. | Agent (#87) |
| 2026-11-11 | Issue #87: `platform.PythonLauncher` became the pure function `platform.PythonCommand(version) (cmd []string, notFound string)`. It gives `py.exe -3.12` on Windows and `python3.12` elsewhere, and the text for a program that is not on the PATH. It calls no `exec.LookPath`. `InstallComfyMCP` finds `cmd[0]` with `Programs.LookPath`. The option `Launcher` stays: when it is set, the service uses it as before, without a look-up. | A base package must not run `os/exec` for a service outside an interface that the service declares. The new name makes each old caller a compile error, because the contract changed (no look-up, no error). `Launcher` is the seam of the 5 older tests of `comfymcp_test.go`, which run a fake python. | Agent (#87) |
| 2026-11-11 | Issue #87: the error of a missing launcher keeps the text of today (`The Python Launcher (py.exe) was not found. Install Python 3.12 from python.org.` on Windows, `python3.12 was not found. Install Python 3.12.` elsewhere). The Green of the issue says "Python 3.12 was not found", and the fix part of the issue says that the service "keeps its error text". The test `TestDefaultLauncherNotOnThePathFails` checks the exact text of `PythonCommand`. | The task brief: the output of `horus install-comfy-mcp` must not change. The issue is not consistent with itself, so the rule "no change that a user can see" decides. | Agent (#87) |
| 2026-11-11 | Issue #89, what counts as an export that "no other package uses": a name that no other package references, test files included. A function, method, constant or variable of this kind is unexported. A type, an interface or a sentinel error stays exported when it appears in the signature or a field of an exported name that another package uses (`backend.Layout`, `doctor.Server`, `generate.ComfyUI`, `catalog.Summary`, `herr.Spec`, `platform.ErrBadTree` and the like). The 10 result types of `internal/mcpserver` (`DescribeResult`, `Envelope` and so on) stay exported too: an unexported name would collide with the helper of the same name (`envelope`, `reporter`), and they describe the wire shape of the tools. | An unexported type in an exported signature is a worse interface than an exported one. The rule is the one of the issue (design rule 2), applied where it makes the interface smaller. | Agent (#89) |
| 2026-11-11 | Issue #89: the members of a group of values stay together. `devtools.Conflict` stays with `Copied`, `Same` and `Replaced`. `provision.StatusDownloaded`, `StatusPresent` and `StatusDeleted` stay with `StatusOK` and the other `Status` values, which other packages use. A name that is also used by a test of another package (`cli.Mark`, `generate.ReportInterval`, `generate.AbsentLimit`, `platform.BlockDelete`, `platform.ErrCannotBlock`, `comfytest.*`) stays. | A group is one vocabulary. Cutting it in two hides which values the field can hold. A use by a test of another package is a use. | Agent (#89) |
| 2026-11-11 | Issue #89: `internal/archtest` keeps its exported names (`Scan`, `Violation`, `Problems`, `Rules`, `File`, `RuleOS`, `RuleDirection`, `RuleToken`). Its only user is its own test. | The package is the tool of the CI gate and no other package imports it. Its test drives it only through these names, and a rename of 8 names would only touch the test. This is the one place where the rule "a use only by the package's own tests: unexport" is not applied. | Agent (#89) |
| 2026-11-11 | Issue #89: `comfy.Round1` and `generate.round1` became one function, `graph.Round1`. The package `graph` is a base package that both `comfy` (adapter) and `generate` (service) may import, and `generate` must not import `comfy`. `graph` has a new test `TestRound1RoundsTheExactBinaryValueHalfToEven`, because the function had no test of its own. | One copy, in the place that the import rules allow (design rule 1). | Agent (#89) |
| 2026-11-11 | Issue #89, two more duplicates found by the sweep: `isFile` (3 copies in `backend`, `doctor`, `provision`) became `platform.IsFile`, and `splitLines` with `bom` (2 copies in `catalog` and `modelfiles`) became `catalog.SplitLines` and `catalog.TrimBOM`. `leaf` (2 copies) and `seconds` (2 copies) differ in what they do, so they stay. | Same rule as for `Round1`: one copy in a base package. `leaf` of `catalog` cuts at `/` and `\`, `leaf` of `provision` cuts at `/` only. The two `seconds` return a string and a number. | Agent (#89) |
| 2026-11-11 | Issue #89: a comment that said "ports horus/x.py" or "as workflow.ps1 does" now says what the code does. A comment that names Python or PowerShell only to explain a kept format stays (`pyRepr`, `PyDumps`, `psString`, the casts of PowerShell in `catalog/workflowdir.go`), because those rules still shape the output and the golden files. The comment of a golden file says that the first version of horus wrote it. `internal/herr/herr_test.go` keeps `.ps1` in its code, because the test checks that no `next_step` names a script. | The comment must be true. A true comment about a kept format explains why. | Agent (#89) |
| 2026-11-11 | Issue #89: `Entry` and `Status` of `internal/comfy` became `historyEntry` and `jobStatus`, and `HTTPError` became `httpError`. Their only users are in the package. | The issue lists them as exports that no other package uses. | Agent (#89) |
| 2026-11-11 | T11: plan step 2 names "the user-scope `horus` MCP server". The run used `claude -p --mcp-config <file> --strict-mcp-config` with a config of its own that starts the fresh binary of `d9827a2` with `mcp`, and the settings of the port 8189 method. Only the tools `list_workflows`, `describe_workflow` and `generate_image` were allowed, and no built-in tool. | The user-scope entry runs `~\go\bin\horus.exe`, which a task must not replace (no `go install`), and it reaches the owner's ComfyUI on port 8188, which must get no job (T7 decision). The same server code runs through the same harness. | Agent (T11) |
| 2026-11-11 | T11: three designs that the earlier tasks recorded are accepted, not filed: the handlers of `install-comfyui`, `start-comfyui` and `stop-comfyui` call the adapter `backend` through `cli.Backend` (T7); `download-model` keeps its handler in `internal/download` (T6); the service `generate` has the pass-through methods `Running` and `FreeMemory` (T5). | Each one has a recorded reason inside the design rules: a service method that only passes the call on hides nothing (design rule 1), and the token rule keeps the flag `-token-env-var` in `internal/download`. The calls still go one way. | Agent (T11) |
| 2026-11-11 | T11: each design defect of part C is an issue that blocks T11 (#86 to #90). The docs defect of part A threatens no goal row, so it is a follow-up outside the epic (#91). | Task brief: a defect against a goal row or a design rule blocks the task. "I would have done it differently" is not a defect. | Agent (T11) |

## Baseline

Measured at `3e27da4` (main after PR #54), before any move task.

- Packages off target: 12 (`internal/models`, `internal/server` and the 10 packages of `internal/scripts/`). Listed packages that do not exist yet: 11.
- Go test functions (`go test -list . ./...`, lines that start with `Test`): 157.
- `taskkill` references in Go files: 8 (in 6 lines of `internal/models/kill_unix.go` and `internal/models/kill_windows.go`).
- Files that name `HF_TOKEN` (10, test files included): `internal/herr/herr.go`, `internal/models/models.go`, `internal/models/models_test.go`, `internal/scripts/downloadmodel/command.go`, `internal/scripts/downloadmodel/downloadmodel.go`, `internal/scripts/downloadmodel/downloadmodel_test.go`, `internal/scripts/workflow/install.go`, `internal/scripts/workflow/workflow_test.go`, `internal/settings/settings.go`, `internal/settings/settings_test.go`.
- Files that break the rule `os-boundary` (19): 9 files with an OS name (`start_*`, `kill_*`, `freespace_*`, `diskfree_windows`, `detach_*`) and 10 more that use `runtime.GOOS`, `syscall` or a `//go:build` line (`checksetup.go`, `freespace_other.go`, `installcomfymcp.go`, `installcomfymcp_test.go`, `diskfree_other.go`, `installcomfyui.go`, `register_test.go`, `startcomfyui.go`, `files.go`, `workflow_test.go`).

## T0

Red (the tool before the change):

    python old_goal_check.py --phase 2 --no-ci
    old_goal_check.py: error: unrecognized arguments: --phase 2      (exit 2)

Green: `python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>/horus.exe mcp` (exit 1):

    FAIL  layout - 12 packages off target, 11 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 157 Go test functions (baseline 157); 0 changed contract files
    FAIL  os-boundary - missing: internal/archtest (task T1); allow list length unknown
    FAIL  direction - missing: internal/archtest (task T1); allow list length unknown
    FAIL  process-tree - missing: internal/platform (task T2); 8 taskkill references
    FAIL  backend - missing: internal/backend (task T7)
    FAIL  token - missing: internal/archtest (task T1); allow list length unknown
    FAIL  commands - missing: internal/cli, internal/doctor (tasks T8 and T9)
    goal: 1 of 8 rows passed

Ratchet: rows with a measuring command 0 to 8.

Phase 1 (python tools/goal_check.py --no-ci, unchanged behaviour): 7 of 8 rows passed. Only real-image fails (needs --real on the GPU machine).

## T1

Red, before the package existed: `go test ./internal/archtest` (exit 1):

    stat ...\internal\archtest: directory not found
    FAIL	./internal/archtest [setup failed]

Green: `go test -v ./internal/archtest`:

    --- PASS: TestRules (0.03s)
    --- PASS: TestPlanted (0.01s)
    --- PASS: TestCompare (0.00s)
    ok  	github.com/hieplam/horus/internal/archtest	0.792s

Green: `python tools/goal_check.py --phase 2 --no-ci --server-cmd <worktree>\bin\horus.exe mcp` (exit 1):

    FAIL  layout - 12 packages off target, 10 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 160 Go test functions (baseline 157); 0 changed contract files
    FAIL  os-boundary - 19 entries on the allow list; archtest: ok
    FAIL  direction - 10 entries on the allow list; archtest: ok
    FAIL  process-tree - missing: internal/platform (task T2); 8 taskkill references
    FAIL  backend - missing: internal/backend (task T7)
    FAIL  token - 4 entries on the allow list; archtest: ok
    FAIL  commands - missing: internal/cli, internal/doctor (tasks T8 and T9)
    goal: 1 of 8 rows passed

Green: a new file `internal/catalog/x_windows.go` (`package catalog`), not committed:

    --- FAIL: TestRules (0.03s)
        archtest_test.go:74: os-boundary: internal/catalog/x_windows.go: the file name has the OS name windows (internal/catalog/x_windows.go). Fix the code: the allow list only becomes shorter
    FAIL	github.com/hieplam/horus/internal/archtest	0.662s

Stub check 1: `Scan` changed to `return nil, nil`, not committed. `go test -run TestPlanted ./internal/archtest` fails with 23 lines `the rules flag [], want [...]`, one for each planted bad file:

    --- FAIL: TestPlanted (0.00s)
        archtest_test.go:151: internal/catalog/name_windows.go: the rules flag [], want [os-boundary]
        archtest_test.go:151: internal/catalog/goos.go: the rules flag [], want [os-boundary]
        ...

Stub check 2: the extra line `internal/catalog/catalog.go` in `allow_token.txt`, not committed:

    --- FAIL: TestRules (0.03s)
        archtest_test.go:74: token: "internal/catalog/catalog.go" no longer breaks the rule: remove this entry from allow_token.txt

Planted files: 34 under `internal/archtest/testdata/planted`. 23 are bad (os-boundary 8, direction 12, token 3). 11 must not be flagged: the two exempt packages, a build tag that is not an OS, a handler that imports a service, a test file that imports `comfytest`, the wiring, a package that imports itself, the download package and the refusal with `HF_TOKEN`, `HF_TOKEN_INVALID`, and a comment.

Ratchet: rules that a test enforces 0 -> 3. Baseline of the next tasks: `os-boundary` 19 files, `direction` 10 pairs, `token` 4 files.

Side measure: `layout` now reports 10 listed packages missing (11 at T0), because `internal/archtest` exists.

## T2

Red (before the change, at `5582e6a`):

    go test ./internal/platform
    stat ...\internal\platform: directory not found
    FAIL	./internal/platform [setup failed]

    python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>/horus.exe mcp
    FAIL  os-boundary - 19 entries on the allow list; archtest: ok
    FAIL  process-tree - missing: internal/platform (task T2); 8 taskkill references

Green on this PC (Windows 11):

    go test -count=1 -v ./internal/platform
    --- PASS: TestVenvPathsFollowTheOS (0.00s)
    --- PASS: TestIsRootedIsIsPathRootedOfThisOS (0.00s)
    --- PASS: TestTreeOutlivesItsStarterAndEndsFromAnotherProcess (0.26s)
    --- PASS: TestTreeEndsWithItsContext (0.08s)
    --- PASS: TestEndTreeRefusesAnIDThatIsNotATree (0.00s)
    --- PASS: TestFreeSpaceOfThisSystem (0.00s)
    ok  	github.com/hieplam/horus/internal/platform	0.836s

    python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>/horus.exe mcp   (rebased on T3, 7f52f09)
    FAIL  layout - 11 packages off target, 8 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 164 Go test functions (baseline 157); 0 changed contract files
    FAIL  os-boundary - 3 entries on the allow list; archtest: ok
    FAIL  direction - 10 entries on the allow list; archtest: ok
    PASS  process-tree - tree test: ok; 0 taskkill references
    FAIL  backend - missing: internal/backend (task T7)
    FAIL  token - 4 entries on the allow list; archtest: ok
    FAIL  commands - missing: internal/cli, internal/doctor (tasks T8 and T9)
    goal: 2 of 8 rows passed

    git grep -n taskkill -- '*.go'
    (no output, exit 1)

Stub checks, run on this PC:

- `EndTree` that returns nil: `check 2: 2 of 2 processes alive 5 s after EndTree`.
- The job handle copied into the starter, not into the root: `check 2: 2 of 2 processes alive 5 s after EndTree`. The name of the job ends with the handle of the starter.

The 3 files that stay on the allow list `os-boundary` are the `goos` variables of `checksetup.go`, `installcomfyui.go` and `startcomfyui.go` (tasks T7 and T8). The three test files that used `runtime.GOOS` now call `platform.ExeName` and `platform.BlockDelete`, and the venv test moved to `internal/platform`.

Ratchet: `os-boundary` allow list 19 -> 3. `taskkill` references 8 -> 0. Packages off target 11 -> 11 (T2 adds no package outside the list), listed packages missing 9 -> 8 (`internal/platform` exists). Go test functions 160 -> 164. Contract `30 of 30 passed`, parity `17 of 17 cases equal`, `gofmt -l .` prints nothing, `lint-public: 0 finding(s)`.

## T3

Red, before the move: `go list ./internal/mcpserver` (exit 1):

    stat ...\internal\mcpserver: directory not found

Green: `go list ./internal/mcpserver` prints `github.com/hieplam/horus/internal/mcpserver`. `go list ./internal/server` prints `stat ...\internal\server: directory not found`. `go test ./...` passes. `python tests/mcp_contract.py --server-cmd <bin>/horus.exe mcp` prints `contract: 30 of 30 passed`. The golden file shows as a pure rename: `internal/{server => mcpserver}/testdata/tools.golden.json | 0`.

Green: `python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>/horus.exe mcp`:

    FAIL  layout - 11 packages off target, 9 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 160 Go test functions (baseline 157); 0 changed contract files

Ratchet: packages off target 12 -> 11.

Decision: `internal/archtest/layers.go` already lists `internal/mcpserver` as a handler in the target table. So the line `internal/server` was deleted, not renamed. The entry of `allow_direction.txt` was renamed to `internal/mcpserver -> internal/comfy`.

## T4

Red, before the move: `go list ./internal/download` (exit 1):

    stat ...\internal\download: directory not found

Before: `go test -list .` of the old package printed 13 tests. After: `go test -list . ./internal/download` prints the same 13 names (checked with a diff).

Green: `go test ./...` passes. `python tools/goal_check.py --phase 2` prints:

    FAIL  layout - 10 packages off target, 7 listed packages missing
    PASS  same-behaviour - ... parity: 17 of 17; go test ./...: ok; 164 Go test functions (baseline 157); 0 changed contract files
    PASS  token - 0 entries on the allow list; archtest: ok

`python tests/mcp_contract.py --server-cmd <bin>/horus.exe mcp` prints `contract: 30 of 30 passed`. `python tests/workflow_parity.py --horus bin/horus.exe` prints `parity: 17 of 17 cases equal`. `gofmt -l .` prints nothing. `lint-public: 0 finding(s)`. `go vet ./...` is clean for windows, darwin/arm64 and linux/amd64.

Ratchet: `token` allow list 4 -> 0. Packages off target 11 -> 10. Go test functions 164 -> 164. The `direction` list keeps 10 entries (one renamed).

## T5

Red, before the change (at `09fec43`, and the same on `50545a1`):

    go list ./internal/graph
    stat ...\internal\graph: directory not found      (exit 1)

    internal\archtest\allow_direction.txt:4:internal/generate -> internal/comfy
    internal\archtest\allow_direction.txt:10:internal/scripts/runillustration -> internal/comfy
    internal\archtest\allow_direction.txt:13:internal/mcpserver -> internal/comfy

    go list -f '{{join .Imports "\n"}}' ./internal/generate   (one line of 17)
    github.com/hieplam/horus/internal/comfy

Baseline of the goal tool before the change: `direction - 10 entries on the allow list`, 164 Go test functions.

Green: `go list -f '{{join .Imports "\n"}}' ./internal/generate | grep -E "internal/comfy|net/http|os/exec"` prints nothing (grep exit 1). `go test ./...` passes. `gofmt -l .` prints nothing. `go vet ./...` is clean for windows, darwin/arm64 and linux/amd64. `lint-public: 0 finding(s)`.

    python tests/mcp_contract.py --server-cmd <bin>\horus.exe mcp
    contract: 30 of 30 passed

    python tests/workflow_parity.py
    parity: 17 of 17 cases equal

    go test -count=1 -v -run TestUIToAPIGolden ./internal/graph
    --- PASS: TestUIToAPIGolden (0.00s)

    git diff origin/main --stat -- tests/ '*golden*'
    (no output)
    git hash-object tests/fixtures/ui_to_api.golden.json
    536efff1e26c28834cef99295023b2d44c75f043   (the same before and after)

    python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>\horus.exe mcp
    FAIL  layout - 9 packages off target, 5 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 174 Go test functions (baseline 157); 0 changed contract files
    FAIL  os-boundary - 3 entries on the allow list; archtest: ok
    FAIL  direction - 7 entries on the allow list; archtest: ok
    PASS  process-tree - tree test: ok; 0 taskkill references
    FAIL  backend - missing: internal/backend (task T7)
    PASS  token - 0 entries on the allow list; archtest: ok
    FAIL  commands - missing: internal/doctor (tasks T8 and T9)
    goal: 3 of 8 rows passed

    python tools/goal_check.py --no-ci   (Phase 1)
    goal: 7 of 8 rows passed   (only real-image fails: it needs --real on the GPU machine)

Stub check: the old import back in a file that is not committed (`internal/generate/stub_check.go` with `import _ "github.com/hieplam/horus/internal/comfy"`):

    --- FAIL: TestRules (0.03s)
        archtest_test.go:74: direction: internal/generate -> internal/comfy: the service layer must not import the adapter layer (internal/generate/stub_check.go). Fix the code: the allow list only becomes shorter
    FAIL	github.com/hieplam/horus/internal/archtest

Before the allow list was shortened, the new code made archtest fail the other way: `"internal/generate -> internal/comfy" no longer breaks the rule: remove this entry from allow_direction.txt` (and the same for `runillustration` and `mcpserver`).

Ratchet: `direction` allow list 10 -> 7 (the entries of `generate`, `runillustration` and `mcpserver` are gone; no entry of `mcpserver` stays for T6). Packages off target 10 -> 9 (`internal/scripts/runillustration` is gone). Listed packages missing 7 -> 5 (`internal/graph`, `internal/cli`). Go test functions 164 -> 174.

## Issue 75

Red, before the change (at `7ad4f65`). On this PC (Windows) the test passes with the default `GOMAXPROCS`
(`go test -count=300 -run TestGenerateImageToolCallsTheService ./internal/mcpserver` prints `ok`). With one
processor it hangs, like the CI job `go linux/amd64` of PR #74:

    GOMAXPROCS=1 go test -count=300 -timeout 40s -run TestGenerateImageToolCallsTheService ./internal/mcpserver
    panic: test timed out after 40s
        running tests:
            TestGenerateImageToolCallsTheService (40s)
    ... server_test.go:56                      (t.Cleanup closes the client session)
    ... tool_generate_image_test.go:119        (t.Fatalf "progress with a token", with mu locked)
    ... tool_generate_image_test.go:88         (the notification handler waits for mu)

The same two defects are in `TestSDKProgressNotifications` (a read after a 10 ms poll and a sleep of 50 ms) and in
`TestGenerateImageTool` of `cmd/horus` (reads `got` right after `CallTool`).

Green (after the change):

    GOMAXPROCS=1 go test -count=300 -timeout 60s -run TestGenerateImageToolCallsTheService ./internal/mcpserver
    ok  	github.com/hieplam/horus/internal/mcpserver	1.773s
    GOMAXPROCS=1 go test -count=100 -timeout 120s -run "TestGenerateImageTool|TestSDKProgress" ./cmd/horus ./internal/mcpserver
    ok  	github.com/hieplam/horus/cmd/horus	4.743s
    ok  	github.com/hieplam/horus/internal/mcpserver	1.676s

Stub check: in `progress.go`, the line `if token == nil || session == nil {` became `if true || token == nil || session == nil {`
(a server that sends no notification). Not committed.

    --- FAIL: TestSDKProgressNotifications (10.00s)
        server_test.go:253: progress messages [] after 10 seconds, want 4
    --- FAIL: TestGenerateImageToolCallsTheService (10.00s)
        tool_generate_image_test.go:117: no progress notification in 10 seconds
    FAIL	github.com/hieplam/horus/internal/mcpserver	21.198s
    --- FAIL: TestGenerateImageTool (10.02s)
        image_test.go:197: no progress notification in 10 seconds
    FAIL	github.com/hieplam/horus/cmd/horus	10.166s

The tests fail in their time limit (10 s each). They do not hang and they do not pass.

## T6

Red, before the change (at `7ad4f65`):

    go list ./internal/provision ./internal/modelfiles
    stat ...\internal\provision: directory not found
    stat ...\internal\modelfiles: directory not found      (exit 1)

    git grep -n "HORUS_JSON" -- internal/mcpserver internal/provision internal/models
    internal/models/models.go:36:const Mark = "HORUS_JSON "     (and 6 more lines of internal/models)

    git grep -n "os/exec" -- internal/mcpserver internal/provision
    internal/mcpserver/tool_models_test.go:16:	"os/exec"

    python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>\horus.exe mcp
    FAIL  layout - 9 packages off target, 5 listed packages missing
    PASS  same-behaviour - ... 174 Go test functions (baseline 157); 0 changed contract files
    FAIL  direction - 7 entries on the allow list; archtest: ok

Green:

    python tests/workflow_parity.py --horus bin/horus.exe
    EQUAL     list
    ... (17 lines EQUAL)
    parity: 17 of 17 cases equal

    python tests/mcp_contract.py --server-cmd <bin>\horus.exe mcp
    PASS  install of an installed style -> ok, already_installed
    PASS  install without accept_license -> LICENSE_CONFIRMATION_REQUIRED, action_by=human, with size
    PASS  uninstall without confirm -> CONFIRMATION_REQUIRED, plan: 1 to delete, 2 shared kept; nothing deleted
    PASS  uninstall with confirm deletes the 1 unshared file and keeps the 2 shared files
    PASS  uninstall with remove_shared deletes the 2 shared files
    PASS  after uninstall, list_workflows shows both styles not_installed
    contract: 30 of 30 passed

    go list ./internal/models ./internal/scripts/workflow
    stat ...\internal\models: directory not found
    stat ...\internal\scripts\workflow: directory not found      (exit 1)

    git grep -n "os/exec" -- internal/mcpserver internal/provision
    (no output, exit 1)

    go test -count=1 -run TestInstallReportsProgressAndACancelStopsTheDownload -v ./internal/mcpserver
    --- PASS: TestInstallReportsProgressAndACancelStopsTheDownload

    python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>\horus.exe mcp
    FAIL  layout - 7 packages off target, 3 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 185 Go test functions (baseline 157); 0 changed contract files
    FAIL  os-boundary - 3 entries on the allow list; archtest: ok
    FAIL  direction - 4 entries on the allow list; archtest: ok
    PASS  process-tree - tree test: ok; 0 taskkill references
    FAIL  backend - missing: internal/backend (task T7)
    PASS  token - 0 entries on the allow list; archtest: ok
    FAIL  commands - missing: internal/doctor (tasks T8 and T9)
    goal: 3 of 8 rows passed

    python tools/goal_check.py --no-ci   (Phase 1)
    goal: 7 of 8 rows passed   (only real-image fails: it needs --real on the GPU machine)

`gofmt -l .` prints nothing. `go vet ./...` is clean for windows, darwin/arm64 and linux/amd64. `lint-public: 0 finding(s)`. `go test ./...` passes.

Stub checks, run on this PC with code that is not committed:

- The stop after a cancel removed from `Install`: `TestInstallReportsProgressAndACancelStopsTheDownload` fails with `the install started the next download after the cancel: ...\text_encoders\qwen3vl_4b_int8_convrot.safetensors`, and `TestInstallStopsWhenTheContextEnds` fails with `err 2 of 2 file(s) are not ready., 2 downloads, mapped []`.
- The new part of the rule `token` disabled: `TestPlanted` fails with `cmd/horus/token_name.go: the rules flag [], want [token]` and the same for `cmd/horus/token_alias.go`.
- A `provision` that does nothing: the 17 parity cases and the install and uninstall checks of the contract test read its output, so they fail.

Ratchet: packages off target 9 -> 7 (`internal/models` and `internal/scripts/workflow` are gone). Listed packages missing 5 -> 3 (`internal/modelfiles`, `internal/provision`). `direction` allow list 7 -> 4 (the entries `internal/models -> os/exec`, `internal/scripts/workflow -> internal/download` and `internal/scripts/workflow -> net/http` are gone). Go test functions 174 -> 185. `token` allow list 0 -> 0, with 2 more planted files.

After the rebase on `9c5a6a5` (the fix of issue #75): `GOMAXPROCS=1 go test -count=50 ./internal/mcpserver ./cmd/horus` prints `ok` for both packages. `go test ./...` passes, `contract: 30 of 30 passed`, `parity: 17 of 17 cases equal`, `lint-public: 0 finding(s)`, and the goal tool gives the same 8 lines as above. The test of T6 that reads progress (`TestInstallReportsProgressAndACancelStopsTheDownload`) already waits on a channel with a time limit and holds no lock across a failure.

## T7

Step 1: issue #53 has a result (PASS, 2026-11-11). Its layout, start command and stop command are in the first T7 row of the Decisions table.

Red, before the change (at `d527cc8`):

    go list ./internal/backend
    stat ...\internal\backend: directory not found      (exit 1)

    python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>\horus.exe mcp
    FAIL  layout - 7 packages off target, 3 listed packages missing
    FAIL  os-boundary - 3 entries on the allow list; archtest: ok
    FAIL  backend - missing: internal/backend (task T7)

Green, Go tests on this PC (Windows 11):

    go test -count=1 -v -run "Stop|NotSupported|Installed|Port" ./internal/backend
    --- PASS: TestOtherOSAnswersNotSupported (0.00s)
    --- PASS: TestInstalledAndLayout (0.00s)
    --- PASS: TestPortFollowsTheSetting (1.31s)
    --- PASS: TestStopEndsTheTreeThatStartStarted (0.93s)
    --- PASS: TestStopRefusesAComfyUIThatHorusDidNotStart (1.47s)
    --- PASS: TestStopWithAStaleState (0.40s)
    ok  	github.com/hieplam/horus/internal/backend

    go test ./internal/backend ./internal/generate
    ok  	github.com/hieplam/horus/internal/backend
    ok  	github.com/hieplam/horus/internal/generate

    GOMAXPROCS=1 go test -count=20 ./internal/backend ./internal/generate ./internal/mcpserver ./cmd/horus
    ok  	github.com/hieplam/horus/internal/backend	199.149s
    ok  	github.com/hieplam/horus/internal/generate	20.304s
    ok  	github.com/hieplam/horus/internal/mcpserver	3.005s
    ok  	github.com/hieplam/horus/cmd/horus	18.214s

`TestDownBackendStartsOneTime` (fake backend): a generate call on a ComfyUI that is down calls `Start` 1 time, and the next call 0 times. `TestStopEndsTheTreeThatStartStarted`: a starter process calls `Start` and ends. The stand-in ComfyUI (the test binary, which answers on its port) and its grandchild live. `Stop` ends both (0 of 2 alive within 5 s), and the port closes. `TestStopRefusesAComfyUIThatHorusDidNotStart`: with no state file, or with a state file of another address, `Stop` refuses, and 2 of 2 processes and the tree of the other address live. The tests use free ports from `127.0.0.1:0`, never 8188 or 8189, and end every process that they start.

    python tests/mcp_contract.py --server-cmd <bin>\horus.exe mcp
    contract: 30 of 30 passed
    python tests/workflow_parity.py --horus <bin>\horus.exe
    parity: 17 of 17 cases equal

    python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>\horus.exe mcp
    FAIL  layout - 5 packages off target, 2 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 189 Go test functions (baseline 157); 0 changed contract files
    FAIL  os-boundary - 1 entries on the allow list; archtest: ok
    FAIL  direction - 4 entries on the allow list; archtest: ok
    PASS  process-tree - tree test: ok; 0 taskkill references
    FAIL  backend - Go tests ok; needs --real on the owner's PC for the image through the backend
    PASS  token - 0 entries on the allow list; archtest: ok
    FAIL  commands - missing: internal/doctor (tasks T8 and T9)
    goal: 3 of 8 rows passed

    python tools/goal_check.py --no-ci   (Phase 1)
    goal: 7 of 8 rows passed   (only real-image fails: it needs --real on the GPU machine)

`gofmt -l .` prints nothing. `go vet ./...` is clean for windows, darwin/arm64 and linux/amd64. `lint-public: 0 finding(s)`. `git grep -n "os/exec\|platform" -- internal/comfy` prints nothing.

### The real run on this PC

The owner's ComfyUI ran on port 8188 the whole time (pid 14432 before and after each step). The run used port 8189 (`HORUS_COMFYUI_URL=http://127.0.0.1:8189`, `HORUS_AUTOSTART=1`, `HORUS_ROOT` = the main checkout).

GPU before the runs: `329 MiB, 0 %` (16:22 and 16:23). After the runs: `193 MiB, 0 %`.

The refusal, against the owner's ComfyUI. There is no state file, and this path calls no `EndTree` (proven by `TestStopRefusesAComfyUIThatHorusDidNotStart`):

    horus stop-comfyui
    horus stop-comfyui: horus did not start the ComfyUI that answers at http://127.0.0.1:8188 (<ComfyUI dir>\horus-comfyui.state.json holds no ComfyUI that horus started), so it does not stop it. Stop it where it was started.
    exit 1
    port 8188 listening, pid 14432

Run 1 (16:23), from the same install folder: the backend started ComfyUI, and ComfyUI ended at once. The tool printed `FAIL  backend - generate_image failed: BACKEND_UNAVAILABLE: horus could not start ComfyUI at http://127.0.0.1:8189`. The log `horus-comfyui.err.log` of that start:

    [ERROR] ASSETS_STARTUP_FAILED: in_use
    Another ComfyUI is already using this database: '<ComfyUI dir>\ComfyUI\user\comfyui.db'.

Run 2 (16:26), from a second install folder that reads the owner's install through junctions (see Decisions), with its own `ComfyUI\user`:

    port 8189 closed
    python tools/goal_check.py --phase 2 --real --only backend --real-out <scratch> --server-cmd <bin>\horus.exe mcp
    PASS  backend - 1536x1024 <scratch>\real-image-seed7-20261111-162629.png (seed 7, 237 s); started through the backend
    goal: 1 of 1 rows passed

    horus stop-comfyui      (HORUS_COMFYUI_URL=http://127.0.0.1:8189)
    The ComfyUI that horus started at http://127.0.0.1:8189 (process 20312) was not running any more.
    port 8188 listening, pid 14432
    port 8189 closed

The image shows a mint-green server box on the left and a sky-blue server box on the right, with sun-yellow envelopes on a lilac conveyor belt between them: it shows the subject (`REAL_SUBJECT` of `tools/goal_check.py`). The ComfyUI of run 2 had already ended when the tool ended. The Python MCP client puts the server in a Job Object that ends with the client (decision G9 of `2026-11-10-go-port-evidence.md`). So a start and a stop of a live ComfyUI ran next, with the same folder and port:

    horus start-comfyui     (HORUS_COMFYUI_URL=http://127.0.0.1:8189)
    Started ComfyUI (process 18112). Waiting for http://127.0.0.1:8189 ...
    OK: ComfyUI answers at http://127.0.0.1:8189
    exit 0  (the starter has ended)
    port 8189 listening, pid 18112
    processes of the tree: 18112 python.exe, 12584 conhost.exe
    horus stop-comfyui      (HORUS_COMFYUI_URL=http://127.0.0.1:8189)
    OK: stopped the ComfyUI that horus started at http://127.0.0.1:8189 (process 18112).
    exit 0
    port 8188 listening, pid 14432
    port 8189 closed
    processes of that tree alive 5 s after the stop: 0 of 2

The second install folder was removed after the run (each junction with a non-recursive delete). The owner's folder still has its `main.py`, its `python.exe` and 26 entries in `models`.

Stub checks, run on this PC with code that is not committed:

- `Stop` that returns nil at once: `TestStopEndsTheTreeThatStartStarted` fails with `2 of 2 processes alive 5 s after Stop` and `the server still answers after Stop`.
- `Stop` that does not compare the address of the state file: `TestStopRefusesAComfyUIThatHorusDidNotStart` fails with `Stop ended the tree of another address`.
- `Start` that returns nil at once: `TestStartsThePythonOfThePortableBuildAndWaitsForTheServer` fails (`fake python args = ""`), and `TestStopEndsTheTreeThatStartStarted` fails with `timeout: the record of the fake ComfyUI`.
- A service that does not call `Start`: `TestDownBackendStartsOneTime` fails with `BACKEND_UNAVAILABLE: horus could not start ComfyUI at http://fake`. In the real run, a start that leaves ComfyUI down gives the same code (run 1 above).

Ratchet: packages off target 7 -> 5 (`internal/scripts/installcomfyui` and `internal/scripts/startcomfyui` are gone). Listed packages missing 3 -> 2 (`internal/backend` exists). `os-boundary` allow list 3 -> 1. Images after a start through the backend 0 -> 1. Go test functions 185 -> 189. Contract `30 of 30 passed`, parity `17 of 17 cases equal`.

## T8

Red, before the change (at `9b48d87`):

    horus doctor
    horus: unknown command "doctor"      (exit 2)

    python tools/goal_check.py --phase 2 --no-ci --only layout os-boundary direction token commands
    FAIL  layout - 5 packages off target, 2 listed packages missing
    FAIL  os-boundary - 1 entries on the allow list; archtest: ok
    FAIL  direction - 4 entries on the allow list; archtest: ok
    PASS  token - 0 entries on the allow list; archtest: ok
    FAIL  commands - missing: internal/doctor (tasks T8 and T9)
    goal: 1 of 5 rows passed

    go test -list . ./... | grep -c '^Test'
    189

Green, Go tests on this PC (Windows 11):

    go test ./...
    ok  	github.com/hieplam/horus/cmd/horus
    ok  	github.com/hieplam/horus/internal/archtest
    ok  	github.com/hieplam/horus/internal/backend
    ok  	github.com/hieplam/horus/internal/catalog
    ok  	github.com/hieplam/horus/internal/cli
    ok  	github.com/hieplam/horus/internal/comfy
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

    GOMAXPROCS=1 go test -count=10 ./cmd/horus ./internal/cli ./internal/doctor ./internal/devtools ./internal/provision ./internal/platform ./internal/comfy ./internal/archtest ./internal/mcpserver
    ok  	github.com/hieplam/horus/cmd/horus	10.923s
    ok  	github.com/hieplam/horus/internal/cli	0.997s
    ok  	github.com/hieplam/horus/internal/doctor	2.711s
    ok  	github.com/hieplam/horus/internal/devtools	1.726s
    ok  	github.com/hieplam/horus/internal/provision	36.949s
    ok  	github.com/hieplam/horus/internal/platform	2.942s
    ok  	github.com/hieplam/horus/internal/comfy	1.585s
    ok  	github.com/hieplam/horus/internal/archtest	0.923s
    ok  	github.com/hieplam/horus/internal/mcpserver	2.351s

The plan of `horus setup` (`internal/provision/setup_test.go`): `TestPlanOfAnEmptyMachine` (no ComfyUI and no file give the steps `backend, file diffusion, file text_encoder, file vae`, and no call that changes a file), `TestPlanOfAReadyMachine` (0 steps, and `RunPlan` of that plan changes nothing), `TestRunPlanInstallsTheBackendThenTheFiles` (the order `install|download flux3-image-klein-int8-convrot.safetensors|download qwen3vl_4b_int8_convrot.safetensors|download flux2-vae.safetensors`, then 0 steps). The workflow is the `models.json` of `workflows/it-illustration`, read only. The fake model files have their folders in a temporary folder. `TestSetupWithoutYesChangesNoFile` (`cmd/horus`): with the real model files adapter and the real downloader on a sandbox, `horus setup demo` without `-yes` makes 0 requests and changes no file, also when ComfyUI is missing. With `-yes` it downloads both files.

    python tests/mcp_contract.py --server-cmd <bin>\horus.exe mcp
    contract: 30 of 30 passed
    python tests/workflow_parity.py --horus <bin>\horus.exe
    parity: 17 of 17 cases equal

    python tools/goal_check.py --phase 2 --no-ci --server-cmd <bin>\horus.exe mcp
    PASS  layout - 0 packages off target, 0 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 207 Go test functions (baseline 157); 0 changed contract files
    PASS  os-boundary - 0 allow list files; archtest: ok
    PASS  direction - 0 allow list files; archtest: ok
    PASS  process-tree - tree test: ok; 0 taskkill references
    FAIL  backend - Go tests ok; needs --real on the owner's PC for the image through the backend
    PASS  token - 0 allow list files; archtest: ok
    FAIL  commands - Go tests ok; needs --real on the owner's PC for horus doctor and horus setup
    goal: 6 of 8 rows passed

    python tools/goal_check.py --no-ci --only setup-commands   (Phase 1)
    PASS  setup-commands - 8 of 8 commands have a passing test

`gofmt -l .` prints nothing. `go vet ./...` is clean for windows, darwin/arm64 and linux/amd64. `lint-public: 0 finding(s)`.

### The real commands on this PC

Read-only commands only, with `HORUS_ROOT` = the main checkout. The owner's ComfyUI on port 8188 got only GET requests (`/system_stats`, `/object_info`), no job. `horus setup` ran without `-yes` only, after the Go tests above.

    horus doctor                      (exit 0)
    PASS  ComfyUI folder - <ComfyUI dir>
    PASS  ComfyUI server - answers at http://127.0.0.1:8188 (ComfyUI 0.44.0)
    PASS  comfy-mcp environment - ~\.venvs\comfy-mcp\Scripts\comfy-mcp.exe
    PASS  comfy-cli - ~\.venvs\comfy-mcp\Scripts\comfy.exe
    PASS  .mcp.json - command points at ~\.venvs\comfy-mcp\Scripts\comfy-mcp.exe, COMFY_BIN at ~\.venvs\comfy-mcp\Scripts\comfy.exe
    PASS  horus on PATH - ~\go\bin\horus.exe
    PASS  .mcp.json horus - command points at ~\go\bin\horus.exe
    All checks passed.

`horus check-setup` of the old binary (at `9b48d87`) and `horus doctor` and `horus check-setup` of the new one print the same 8 lines (`diff` prints nothing). `check-setup -workflow it-illustration -quick` of the old binary and `doctor -workflow it-illustration -quick` of the new one print the same lines, and both exit 0.

    horus setup it-illustration       (exit 0)
    Setup of workflow 'it-illustration':
    PRESENT   ComfyUI in <ComfyUI dir>
    PRESENT   diffusion flux3-image-klein-int8-convrot.safetensors (5.83 GB) ~\ComfyUI-models\diffusion_models\flux3-image-klein-int8-convrot.safetensors
    PRESENT   text_encoder qwen3vl_4b_int8_convrot.safetensors (4.86 GB) ~\ComfyUI-models\text_encoders\qwen3vl_4b_int8_convrot.safetensors
    PRESENT   vae flux2-vae.safetensors (0.34 GB) ~\ComfyUI-models\vae\flux2-vae.safetensors
    Plan: 0 steps. Nothing is missing.

    python tools/goal_check.py --phase 2 --no-ci --real --only commands --server-cmd <bin>\horus.exe mcp
    PASS  commands - horus doctor: 0 FAIL lines; horus setup it-illustration: 0 steps, nothing is missing; 5 of 5 named files present and unchanged
    goal: 1 of 1 rows passed

Stub checks:

    the row commands with the old binary (no doctor, no setup)
    FAIL  commands - horus setup it-illustration: exit 2, 0 paths in the plan: Run horus <command> -h for the flags of a command.
    an empty file internal/archtest/allow_token.txt
    FAIL  token - 1 allow list files (allow_token.txt); archtest: FAIL
    a package internal/scripts/checksetup with one file
    FAIL  layout - 1 packages off target, 0 listed packages missing

- A `setup` that prints nothing fails `TestSetupPrintsThePlanAndRunsNothingWithoutYes` and `TestSetupOfAReadyMachine` (they compare the whole text), and `TestSetupWithoutYesChangesNoFile`.
- A `Plan` that returns no step fails `TestPlanOfAnEmptyMachine`. A `RunPlan` that installs nothing fails `TestRunPlanInstallsTheBackendThenTheFiles`.
- A doctor that prints nothing fails `TestDoctorLinesAndExitCode`.

Ratchet: packages off target 5 -> 0, listed packages missing 2 -> 0. Allow list entries: os-boundary 1 -> 0, direction 4 -> 0, token 0 -> 0. Allow list files 3 -> 0. Commands 0 -> 2 (`doctor`, `setup`). Go test functions 189 -> 207. Contract `30 of 30 passed`, parity `17 of 17 cases equal`. Rows passed without `--real`: 3 -> 6 of 8.

## T9

Red, before the change (`origin/main`, `999a4c3`):

    git grep -n -E "internal/(scripts|server|models)" -- '*.md' ':!docs/plans' ':!docs/specs'
    (prints nothing: the plan Red is empty, see the Decisions table)

    git grep -n -E 'horus workflow \(child\)|scripts/<command>|horus start-comfyui" does|start "horus workflow' origin/main -- '*.md' ':!docs/plans' ':!docs/specs'
    origin/main:AGENTS.md:76:  ... (server, herr, catalog, comfy, generate, models, settings, scripts/<command>)
    origin/main:docs/mcp.md:28:  participant W as horus workflow (child)
    origin/main:docs/mcp.md:31:  S->>W: start "horus workflow list -json"
    origin/main:docs/mcp.md:35:  S->>W: start "horus workflow check -quick -json"
    origin/main:docs/mcp.md:46:  participant W as horus workflow (child)
    origin/main:docs/mcp.md:50:  S->>W: start "horus workflow install -json"
    origin/main:docs/mcp.md:73:  S->>C: start it through the backend, as "horus start-comfyui" does (Windows only)
    (7 lines)

    git grep -n "horus doctor" -- docs/setup.md README.md AGENTS.md | wc -l
    0 lines before (the command did not exist in the docs)

Green, after the change:

    git grep -n -E "internal/(scripts|server|models)" -- '*.md' ':!docs/plans' ':!docs/specs'
    (prints nothing)

    git grep -n -E 'horus workflow \(child\)|scripts/<command>|horus start-comfyui" does|start "horus workflow' -- '*.md' ':!docs/plans' ':!docs/specs'
    (prints nothing)

    git grep -n "horus doctor" -- docs/setup.md README.md AGENTS.md | wc -l
    13

    python scripts/lint-public.py
    lint-public: 0 finding(s)

    tests/mcp_contract.py --server-cmd bin/horus.exe mcp
    PASS  docs/mcp.md lists every error code
    contract: 30 of 30 passed

    python tools/goal_check.py --phase 2 --no-ci --server-cmd bin/horus.exe mcp
    goal: 6 of 8 rows passed   (backend and commands need --real, which T9 does not run)

The command names, flags and package paths in the docs were copied from `horus -h`, `horus <command> -h` and `ls internal` of the built binary of this commit.

Stub check: without the edits, the second Red search still prints 7 lines, and `horus doctor` has 0 lines in the three files. An edit that only removed the old words and kept the old diagrams would leave the diagrams without the `provision service` and `generate service` participants. The diagram labels equal the names in the text.

Ratchet: stale package and process names in the docs 7 -> 0. Lines with `horus doctor` in the three files 0 -> 13.

## Issue 80

Goal: the rows `os-boundary`, `direction` and `token` fail on a stub `internal/archtest`.

Red (before the change; `internal/archtest` replaced by one file `package archtest`, not committed):

    python tools/goal_check.py --phase 2 --only os-boundary direction token
    PASS  os-boundary - 0 allow list files; archtest: ok
    PASS  direction - 0 allow list files; archtest: ok
    PASS  token - 0 allow list files; archtest: ok
    goal: 3 of 3 rows passed

Green (after the change, same stub):

    FAIL  os-boundary - 0 allow list files; archtest: no passing test: TestRules, TestPlanted, TestProblems
    FAIL  direction - 0 allow list files; archtest: no passing test: TestRules, TestPlanted, TestProblems
    FAIL  token - 0 allow list files; archtest: no passing test: TestRules, TestPlanted, TestProblems
    goal: 0 of 3 rows passed

Green (real tree):

    PASS  os-boundary - 0 allow list files; archtest: ok
    PASS  direction - 0 allow list files; archtest: ok
    PASS  token - 0 allow list files; archtest: ok
    goal: 3 of 3 rows passed

Stub check: the stub is the Red. A check that reads only the exit code passes it.

Same question for the other rows. `backend` and `commands` took exit 0 of `go test`, so an empty package passed that step. They now use `go_test_ran`, which fails on `[no test files]` and on a run with no passing test. `layout`, `same-behaviour` and `process-tree` were judged. `same-behaviour` fails on a stub because the contract tests run on the binary. `process-tree` has a weak spot that issue #81 fixes: any test with `Tree` in its name passes.

Ratchet: checks of the three rows that pass on an empty archtest: 3 -> 0.

## Issue 81

Goal: the rows `same-behaviour` and `process-tree` read every CI job that their oracle names.

Red (`ci_detail` of `origin/main`, jobs of `ci.yml` as hand-made lists, no network):

    origin/main ci_detail, workflow-parity macos-latest failure -> (True, 'CI contract job green on windows-latest and macos-latest')
    origin/main ci_detail, go darwin/arm64 missing -> (True, 'CI contract job green on windows-latest and macos-latest')

Green (`python -m unittest discover -s tools -v`, the same lists through `judge_jobs`):

    test_all_green_passes_same_behaviour_and_process_tree ... ok
    test_empty_job_list_fails ... ok
    test_job_names_match_the_workflow_file ... ok
    test_missing_darwin_job_fails_both_rows_and_names_it ... ok
    test_process_tree_ignores_a_red_parity_job ... ok
    test_red_parity_job_fails_same_behaviour_and_names_it ... ok
    test_running_and_cancelled_jobs_fail ... ok
    Ran 7 tests in 0.001s
    OK

The failing tests assert the names, for example `go darwin/arm64 (missing)` and `workflow-parity macos-latest`.

Green (real tree, HEAD `6789c49`, newest CI run `completed`, `success`):

    PASS  same-behaviour - contract: local 30 of 30; CI: 6 jobs green (contract windows-latest, contract macos-latest, workflow-parity windows-latest, workflow-parity macos-latest, go windows/amd64, go darwin/arm64); parity: 17 of 17; go test ./...: ok; 207 Go test functions (baseline 157); 0 changed contract files
    PASS  process-tree - tree test: ok; 0 taskkill references; CI: 2 jobs green (go windows/amd64, go darwin/arm64)
    (layout, os-boundary, direction, token: PASS; backend, commands: FAIL, they need --real)
    goal: 6 of 8 rows passed

A first run on the same commit while its CI run was still going gave, as designed:

    FAIL  same-behaviour - ...; CI: not green: contract macos-latest (in_progress), workflow-parity macos-latest (in_progress); ...
    PASS  process-tree - tree test: ok; 0 taskkill references; CI: 2 jobs green (go windows/amd64, go darwin/arm64)

Stub check: a `ci_detail` that ignores the job names, or reads only `contract`, returns `True` on the Red lists (the Red output). The unit tests fail it.

Same question for the other rows. `process-tree` passed on any test with `Tree` in its name, so a stub `func TestTree(t *testing.T) {}` passed with 0 `taskkill`. It now needs the named test with check 1 and check 2. `layout` compares `go list` with a committed list, so a stub package still counts as present (that is its goal: where each concern lives, and `same-behaviour` and the archtest rows prove behaviour). `backend` and `commands` were fixed in #80. The rest of `same-behaviour` (contract 30 of 30, parity 17 of 17, `go test ./...`, test count, golden files) fails on a stub, because the contract and parity tests run the built binary.

Ratchet: CI jobs that the oracle names and the tool does not read: 3 -> 0 (`workflow-parity windows-latest`, `workflow-parity macos-latest`, `go darwin/arm64`). Rows with a CI part: 1 -> 2.

Stub check of the pinned tree test: with `TestTreeOutlivesItsStarterAndEndsFromAnotherProcess` renamed to `TestTreeStub` (not committed):

    FAIL  process-tree - TestTreeOutlivesItsStarterAndEndsFromAnotherProcess did not pass in internal/platform; 0 taskkill references

## T10

Result: 8 of 8 rows pass on `main` = `9699bb8`, in three runs (see the T10 decisions). The stub probes of the fixed tool all fail as they must.

The first run, on `3bad82f`, also gave 8 of 8, but it found that two checks of the tool were too weak to prove their rows alone. These became #80 (the archtest rows passed on an empty `internal/archtest`) and #81 (the CI part read only the `contract` jobs). Both are fixed (PR #83, PR #84), and every output below is from `9699bb8`. Follow-up outside the epic: #82.

### Build and install

A clean worktree, detached at `origin/main`:

    git rev-parse HEAD
    9699bb8d4a8fe4fb4df21e8b9d8f7fe0355a03f8
    git status --porcelain
    (prints nothing)

    go build -o bin/ ./cmd/horus
    go install ./cmd/horus
    go version -m ~\go\bin\horus.exe
        build	vcs=git
        build	vcs.revision=9699bb8d4a8fe4fb4df21e8b9d8f7fe0355a03f8
        build	vcs.time=2026-11-11T11:33:20Z
        build	vcs.modified=false

So the user-scope MCP entry `horus` (`~\go\bin\horus.exe`) runs the code of `9699bb8`.

CI of the commit (`gh run view 43012909485 --json headSha,status,conclusion,jobs`):

    9699bb8d4a8fe4fb4df21e8b9d8f7fe0355a03f8
    completed
    success
    workflow-parity macos-latest	success
    go darwin/arm64	success
    go windows/amd64	success
    contract macos-latest	success
    go linux/amd64	success
    workflow-parity windows-latest	success
    contract windows-latest	success

### Run 1: every row, with the CI part, no GPU

    python tools/goal_check.py --phase 2 --server-cmd <worktree>/bin/horus.exe mcp      (exit 1)
    PASS  layout - 0 packages off target, 0 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI: 6 jobs green (contract windows-latest, contract macos-latest, workflow-parity windows-latest, workflow-parity macos-latest, go windows/amd64, go darwin/arm64); parity: 17 of 17; go test ./...: ok; 207 Go test functions (baseline 157); 0 changed contract files
    PASS  os-boundary - 0 allow list files; archtest: ok
    PASS  direction - 0 allow list files; archtest: ok
    PASS  process-tree - tree test: ok; 0 taskkill references; CI: 2 jobs green (go windows/amd64, go darwin/arm64)
    FAIL  backend - Go tests ok; needs --real on the owner's PC for the image through the backend
    PASS  token - 0 allow list files; archtest: ok
    FAIL  commands - Go tests ok; needs --real on the owner's PC for horus doctor and horus setup
    goal: 6 of 8 rows passed

### Run 2: the row commands, for real

`HORUS_ROOT` = the main checkout (read only). The owner's ComfyUI on port 8188 (pid 14432) got only GET requests.

    python tools/goal_check.py --phase 2 --real --only commands --server-cmd <worktree>/bin/horus.exe mcp      (exit 0)
    PASS  commands - horus doctor: 0 FAIL lines; horus setup it-illustration: 0 steps, nothing is missing; 5 of 5 named files present and unchanged
    goal: 1 of 1 rows passed

### Run 3: the row backend, for real

The owner's ComfyUI answered on port 8188 (ComfyUI 0.44.0, pid 14432). It was not stopped and got no job. The run used the method of T7: a second install folder `<scratch>\comfy8189` with a junction `python_embeded` to the owner's install, a folder `ComfyUI` with 22 junctions (each folder of the owner's `ComfyUI` except `user`, `input`, `output`, `temp`), 4 empty folders of its own and 36 copied small files. Settings: `HORUS_COMFYUI_DIR=<scratch>\comfy8189`, `HORUS_COMFYUI_URL=http://127.0.0.1:8189`, `HORUS_AUTOSTART=1`, `HORUS_ROOT` = the main checkout.

GPU before the run (`nvidia-smi --query-gpu=memory.used,utilization.gpu --format=csv,noheader`). A game ran on the PC (see the T10 decision):

    18:37  746 MiB, 42 %
    18:48  884 MiB, 57 %
    18:58  896 MiB, 46 %
    19:08  898 MiB, 52 %
    19:18  910 MiB, 47 %
    19:28  472 MiB, 37 %   (the game process had ended)
    19:28  483 MiB, 28 %   port 8188 pid 14432, port 8189 closed

The run (19:28:47 to 19:33:02):

    python tools/goal_check.py --phase 2 --real --only backend --real-out <scratch>\real --server-cmd <worktree>/bin/horus.exe mcp      (exit 0)
    PASS  backend - 1536x1024 <scratch>\real\real-image-seed7-20261111-192903.png (seed 7, 239 s); started through the backend
    goal: 1 of 1 rows passed

The image: on the left a mint-green server box, on the right a sky-blue server box, and between them a purple conveyor belt with five yellow-orange envelopes. It shows the subject (`REAL_SUBJECT` of `tools/goal_check.py`). The envelopes are more amber than sun-yellow, and the belt is a deeper purple than lilac. It is the same picture as the run on `3bad82f` (same seed).

After the run (19:33):

    horus stop-comfyui      (same settings)
    The ComfyUI that horus started at http://127.0.0.1:8189 (process 6524) was not running any more.
    exit 0
    8189 listeners: 0
    processes with comfy8189 in the command line: 0
    port 8188 pid: 14432
    nvidia-smi: 7 MiB, 0 %

The ComfyUI of the run had already ended when the tool ended: the Python MCP client ends its Job Object (decision G9 of `2026-11-10-go-port-evidence.md`, and the T7 run). The start and the stop of a live ComfyUI through the backend are proven in `## T7`.

Cleanup: the 23 junctions (1 + 22) were deleted one at a time with `(Get-Item <junction>).Delete()`, which removes only the link. Then a recursive listing found 0 links in the folder, and the 43 files and the real folders that this task made were deleted one by one (each folder after it was empty). No recursive delete ran. After the cleanup:

    Test-Path <ComfyUI dir>\ComfyUI\main.py              True
    Test-Path <ComfyUI dir>\python_embeded\python.exe    True
    entries in <ComfyUI dir>\ComfyUI\models              26

    horus workflow check it-illustration -quick      (HORUS_ROOT = the main checkout, exit 0)
    Checking workflow 'it-illustration': 3 model file(s), size only.
    OK        diffusion ~\ComfyUI-models\diffusion_models\flux3-image-klein-int8-convrot.safetensors (5827874779 bytes)
    OK        text_encoder ~\ComfyUI-models\text_encoders\qwen3vl_4b_int8_convrot.safetensors (4858559240 bytes)
    OK        vae ~\ComfyUI-models\vae\flux2-vae.safetensors (336211292 bytes)
    OK        models.json lists all 3 model file(s) that workflow.json loads
    OK        the server has all 10 node classes of workflow.json
    OK        the server lists flux3-image-klein-int8-convrot.safetensors
    OK        the server lists qwen3vl_4b_int8_convrot.safetensors
    OK        the server lists flux2-vae.safetensors
    OK: workflow 'it-illustration' is ready.

The byte sizes are equal to `workflows/it-illustration/models.json`.

### Stub probes of the fixed tool

Each probe was not committed and was restored with `git checkout`. The tree was clean after each one (`git status --porcelain` prints nothing).

Probe 1: `internal/archtest` with one file `package archtest` (no rules, no tests):

    python tools/goal_check.py --phase 2 --only os-boundary direction token      (exit 1)
    FAIL  os-boundary - 0 allow list files; archtest: no passing test: TestRules, TestPlanted, TestProblems
    FAIL  direction - 0 allow list files; archtest: no passing test: TestRules, TestPlanted, TestProblems
    FAIL  token - 0 allow list files; archtest: no passing test: TestRules, TestPlanted, TestProblems
    goal: 0 of 3 rows passed

Probe 2: `archtest.Scan` returns `nil, nil` at once (the real tests stay):

    python tools/goal_check.py --phase 2 --only os-boundary direction token      (exit 1)
    FAIL  os-boundary - 0 allow list files; archtest: FAIL
    FAIL  direction - 0 allow list files; archtest: FAIL
    FAIL  token - 0 allow list files; archtest: FAIL
    goal: 0 of 3 rows passed

Probe 3: the CI judgement of the tool (`judge_jobs` with `SAME_BEHAVIOUR_JOBS` and `PROCESS_TREE_JOBS`) on job lists made by hand, no network:

    red parity macOS
      same-behaviour: (False, 'CI: not green: workflow-parity macos-latest (failure)')
      process-tree:   (True, 'CI: 2 jobs green (go windows/amd64, go darwin/arm64)')
    no go darwin/arm64
      same-behaviour: (False, 'CI: not green: go darwin/arm64 (missing)')
      process-tree:   (False, 'CI: not green: go darwin/arm64 (missing)')
    contract jobs only
      same-behaviour: (False, 'CI: not green: workflow-parity windows-latest (missing), workflow-parity macos-latest (missing), go windows/amd64 (missing), go darwin/arm64 (missing)')
      process-tree:   (False, 'CI: not green: go windows/amd64 (missing), go darwin/arm64 (missing)')

`process-tree` passes on a red parity job, and that is correct: its oracle names only the tree test, which the `go` jobs run.

Probe 4: `TestTreeOutlivesItsStarterAndEndsFromAnotherProcess` renamed to `TestTreeStub`:

    python tools/goal_check.py --phase 2 --no-ci --only process-tree      (exit 1)
    FAIL  process-tree - TestTreeOutlivesItsStarterAndEndsFromAnotherProcess did not pass in internal/platform; 0 taskkill references
    goal: 0 of 1 rows passed

No check passes on a stub of the implementation, so no new issue was filed. A test with the pinned name and an empty body still passes, but that is a weakened test, which a standing rule forbids. The limits that were accepted are in the T10 decisions.

### The check of the epic tool on every closed task

    node epic.mjs -R hieplam/horus check <n> --dry-run
    PASS #55 via PR #68 (https://github.com/hieplam/horus/pull/68)
    PASS #56 via PR #69 (https://github.com/hieplam/horus/pull/69)
    PASS #57 via PR #71 (https://github.com/hieplam/horus/pull/71)
    PASS #58 via PR #70 (https://github.com/hieplam/horus/pull/70)
    PASS #59 via PR #72 (https://github.com/hieplam/horus/pull/72)
    PASS #60 via PR #73 (https://github.com/hieplam/horus/pull/73)
    PASS #61 via PR #74 (https://github.com/hieplam/horus/pull/74)
    PASS #62 via PR #77 (https://github.com/hieplam/horus/pull/77)
    PASS #63 via PR #78 (https://github.com/hieplam/horus/pull/78)
    PASS #64 via PR #79 (https://github.com/hieplam/horus/pull/79)
    PASS #75 via PR #76 (https://github.com/hieplam/horus/pull/76)
    PASS #80 via PR #83 (https://github.com/hieplam/horus/pull/83)
    PASS #81 via PR #84 (https://github.com/hieplam/horus/pull/84)

All 13 issues are closed.

### The contract files did not change

    git diff --stat 3e27da4 -- tests/mcp_contract.py tests/workflow_parity.py tests/fixtures '*golden*'
     internal/{server => mcpserver}/testdata/tools.golden.json | 0
     1 file changed, 0 insertions(+), 0 deletions(-)

    git diff --name-status -M 3e27da4 -- tests/mcp_contract.py tests/workflow_parity.py tests/fixtures '*golden*'
    R100	internal/server/testdata/tools.golden.json	internal/mcpserver/testdata/tools.golden.json

The only entry is the golden file that moved with its package, with 100 percent equal content (decision of T0). No content changed.

### Ratchets

| Ratchet | Baseline | Today (`9699bb8`) | Source today |
|---|---|---|---|
| Packages off target | 12 | 0 | Run 1, `layout` |
| Files that break `os-boundary` | 19 | 0 | Run 1, `os-boundary` (no allow list file, the 3 archtest tests pass) |
| Imports that break `direction` | 10 | 0 | Run 1, `direction` |
| Files that break `token` | 4 | 0 | Run 1, `token` |
| `taskkill` references | 8 | 0 | Run 1, `process-tree` |
| Go test functions | 157 | 207 | Run 1, `same-behaviour` |
| Contract | 30 of 30 | 30 of 30 | Run 1, and CI `contract` on both systems |
| Parity | 17 of 17 | 17 of 17 | Run 1, and CI `workflow-parity` on both systems |
| Images after a start through the backend | 0 | 1 | Run 3 |
| Commands (`doctor`, `setup`) | 0 | 2 | Run 2 |
| Rows passed | 1 of 8 (T0) | 8 of 8 (runs 1 to 3) | |
| Checks that pass on a stub (found by T10) | 2 issues (#80, #81) | 0 | Stub probes 1 to 4 |

Stub check of the whole result: the tool gave `goal: 1 of 8 rows passed` at T0 on the tree before the work, so the 8 passes come from the work.

## Issue 90

Goal: no text code passes between `generate` and `comfy` (design rule 2). The progress texts of `generate_image` do not change.

Red (`git grep -n 'pending:\|HasPrefix(state, "pending")\|SplitN(state' -- internal ':!*_test.go'` on `d9827a2`):

    internal/comfy/comfy.go:316:// QueueState is "running", "pending:<position>" or "absent".
    internal/comfy/comfy.go:334:			return fmt.Sprintf("pending:%d", i+1), nil
    internal/comfy/comfy.go:356:	} else if strings.HasPrefix(state, "pending") {
    internal/generate/generate.go:63:	// QueueState is "running", "pending:<position>" or "absent".
    internal/generate/generate.go:596:				} else if strings.HasPrefix(state, "pending") {
    internal/generate/generate.go:597:					text = "waiting in the backend queue, position " + strings.SplitN(state, ":", 2)[1]

Green (the same command prints nothing; `go test ./...` ok for every package; `GOMAXPROCS=1 go test -count=10 ./internal/generate ./internal/comfy ./cmd/horus` ok):

    contract: 30 of 30 passed
    goal_check --phase 2 --no-ci: 6 PASS rows
    Go test functions: 210 -> 211 (new: TestQueueStates in internal/graph)

`TestTimeoutCancelsTheJob` in `internal/generate` still checks `rendering, ` and `waiting in the backend queue, position 2, `. The test of `internal/comfy` on `comfytest` checks the typed state for a done (absent), a running and a pending job (`graph.Queue{Position: 2}`).

Stub check: a `QueueState` of the fake that returns `graph.Queue{}` for a running job:

    --- FAIL: TestTimeoutCancelsTheJob (0.13s)
        generate_test.go:329: running: {Code:GENERATION_FAILED Message:the job left the backend queue without a result ...

Interface question: the service calls all 9 methods of `ComfyUI`, so no method is removed.

Ratchet: text codes that a service parses from an adapter: 1 -> 0.

## Issue 86

Goal: no service gets the text of a handler, and the wiring holds no logic (row `direction`, design rule 5). The output of `horus doctor -workflow it-illustration -quick` does not change.

Red (before the change, on `d9827a2`):

    git grep -n "cli.Workflow(\|lines \[\]string, ok bool" -- cmd/horus/wiring.go internal/doctor/doctor.go
    cmd/horus/wiring.go:120:	code := cli.Workflow(ctx, args, &buf, &buf, func(cli.SetupPlace) cli.Setup { return w.setupAt(w.place) })
    internal/doctor/doctor.go:61:	Check(ctx context.Context, workflow string, quick bool) (lines []string, ok bool, err error)

Green (after the change):

    git grep -n "cli.Workflow(\|lines \[\]string, ok bool" -- cmd/horus/wiring.go internal/doctor/doctor.go
    (no output, exit 1)

The output of the binary of `d9827a2` and of the new binary, with `HORUS_ROOT` = the main checkout, for 6 read-only cases: `doctor`, `check-setup`, `doctor -workflow it-illustration -quick`, `check-setup -workflow it-illustration -quick`, `doctor -workflow nosuch -quick` (exit 1 in both) and `check-setup -workflow it-illustration -quick -models-dir <empty folder>`:

    diff -r before after
    (no output, exit 0)

The other checks:

    go test ./...                                   ok for every package
    go test -count=1 ./internal/archtest            ok
    tests/mcp_contract.py                           contract: 30 of 30 passed
    tests/workflow_parity.py                        parity: 17 of 17 cases equal
    tools/goal_check.py --phase 2 --no-ci           goal: 6 of 8 rows passed (backend and commands need --real)
    scripts/lint-public.py                          lint-public: 0 finding(s)
    gofmt -l .                                      (no output)
    GOOS=darwin GOARCH=arm64 go vet ./...           ok
    GOOS=linux GOARCH=amd64 go vet ./...            ok

The new test `TestDoctorModelCheckLinesAndExitCode` (`internal/cli`, on a fake `Setup`) compares the full output: the model lines, their indent of 6 spaces, the PASS or FAIL line, and exit code 1 for a failed check.

Stub check: `RunDoctor` with the model check switched off (`if false && workflow != ""`, not committed):

    go test ./internal/cli/ ./cmd/horus/
    --- FAIL: TestDoctorModelCheckLinesAndExitCode (0.00s)
    --- FAIL: TestDoctorModelCheckIsFullByDefault (0.00s)
    FAIL	github.com/hieplam/horus/internal/cli	0.732s
    --- FAIL: TestDoctorThroughTheWiring (0.05s)
    --- FAIL: TestDoctorModelCheckOfAnUnknownWorkflow (0.03s)
    FAIL	github.com/hieplam/horus/cmd/horus	1.183s

    diff before/doctor_-workflow_it-illustration_-quick.txt stub/out.txt
    8,18d7
    <       Checking workflow 'it-illustration': 3 model file(s), size only.
    <       OK        diffusion ~/ComfyUI-models/diffusion_models/flux3-image-klein-int8-convrot.safetensors (5827874779 bytes)
    (8 more model lines)
    <       OK: workflow 'it-illustration' is ready.
    < PASS  models for 'it-illustration' - all files present, size checked (the SHA-256 is not checked; drop -quick)

Ratchet: calls from the wiring into a handler to feed a service: 1 -> 0. Go test functions: 207 -> 207.

## Issue 88

Goal: one place decides `install_state`, and both tools give the same value for the same disk.

Red (before the change, on `64e93a6`):

    git grep -n 'state = "partially_installed"\|return "partially_installed"' -- internal ':!*_test.go'
    internal/mcpserver/tool_describe_workflow.go:94:				state = "partially_installed"
    internal/provision/result.go:116:		return "partially_installed"

    git grep -n "partially_installed" -- internal/mcpserver ':!*_test.go' ':!*testdata*'
    internal/mcpserver/schema.go:103:var tInstallState = tEnum("installed", "partially_installed", "not_installed")
    internal/mcpserver/tool_describe_workflow.go:94:				state = "partially_installed"

Green (after the change):

    git grep -n 'state = "partially_installed"\|return "partially_installed"' -- internal ':!*_test.go'
    internal/provision/result.go:126:		return "partially_installed"

    git grep -n "partially_installed" -- internal/mcpserver ':!*_test.go' ':!*testdata*'
    internal/mcpserver/schema.go:103:var tInstallState = tEnum("installed", "partially_installed", "not_installed")

    go test -count=1 -run "TestBothToolsGiveTheSameInstallState|TestRowState" -v ./internal/mcpserver/ ./internal/provision/
    --- PASS: TestBothToolsGiveTheSameInstallState (0.02s)
    ok  	github.com/hieplam/horus/internal/mcpserver	1.791s
    --- PASS: TestRowState (0.00s)
    ok  	github.com/hieplam/horus/internal/provision	0.801s

`TestBothToolsGiveTheSameInstallState` (`internal/mcpserver`, on a fake `Setup`) calls `list_workflows` and `describe_workflow` for 0, 1 and 3 of the 3 files of `it-illustration` on disk. Both tools give `not_installed`, `partially_installed` and `installed`. `TestRowState` also checks `InstallState` for each row.

The other checks:

    go test -count=1 ./...                          ok for every package
    tests/mcp_contract.py                           contract: 30 of 30 passed
    tests/workflow_parity.py                        parity: 17 of 17 cases equal
    tools/goal_check.py --phase 2 --no-ci           goal: 6 of 8 rows passed (backend and commands need --real)
    scripts/lint-public.py                          lint-public: 0 finding(s)
    gofmt -l .                                      (no output)
    GOOS=darwin GOARCH=arm64 go vet ./...           ok
    GOOS=linux GOARCH=amd64 go vet ./...            ok
    git status -- internal/mcpserver/testdata       (no change to tools.golden.json)

Stub check: `describe_workflow` returns a fixed `installed` (not committed):

    go test -count=1 -run TestBothToolsGiveTheSameInstallState ./internal/mcpserver/
    --- FAIL: TestBothToolsGiveTheSameInstallState (0.03s)
        tool_setup_test.go:404: 0 of 3 files: list_workflows "not_installed", describe_workflow "installed", want "not_installed"
        tool_setup_test.go:404: 1 of 3 files: list_workflows "partially_installed", describe_workflow "installed", want "partially_installed"
    FAIL	github.com/hieplam/horus/internal/mcpserver	1.874s

Ratchet: copies of the `install_state` rule: 2 -> 1. Go test functions: 208 -> 209.

## Issue 87

The service `internal/provision` read the text lines of the downloader to build the detail of a failed download. It also found Python through `platform.PythonLauncher`, which ran `exec.LookPath`. The decisions are the three `#87` rows of the Decisions table.

Red, before the change (at `d9827a2`):

    $ git grep -n "whyLine\|PythonLauncher(" -- internal/provision ':!*_test.go'
    internal/provision/actions.go:302:// whyLine picks the download lines that go into the detail of a FAILED item:
    internal/provision/actions.go:305:var whyLine = regexp.MustCompile(`^(FAIL:|The server answered with an error|Attempt )`)
    internal/provision/actions.go:356:				if whyLine.MatchString(l) {
    internal/provision/comfymcp.go:82:			if launcher, err = platform.PythonLauncher(o.PythonVersion); err != nil {
    $ git grep -n "exec.LookPath" -- internal/platform/platform.go
    internal/platform/platform.go:110:		p, err := exec.LookPath("py.exe")
    internal/platform/platform.go:116:	p, err := exec.LookPath("python" + version)

Green, after the change:

    $ git grep -n "whyLine\|PythonLauncher(" -- internal/provision ':!*_test.go'; echo "exit=$?"
    exit=1
    $ git grep -n "exec.LookPath" -- internal/platform/platform.go; echo "exit=$?"
    exit=1
    $ go test ./...
    ok  	github.com/hieplam/horus/cmd/horus
    ok  	github.com/hieplam/horus/internal/archtest
    ok  	github.com/hieplam/horus/internal/backend
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
    $ GOMAXPROCS=1 go test -count=10 ./internal/provision ./internal/mcpserver ./cmd/horus
    ok  	github.com/hieplam/horus/internal/provision	40.011s
    ok  	github.com/hieplam/horus/internal/mcpserver	2.014s
    ok  	github.com/hieplam/horus/cmd/horus	10.248s
    $ python tests/mcp_contract.py --server-cmd <repo>\bin\horus.exe mcp
    contract: 30 of 30 passed
    $ python tests/workflow_parity.py --horus bin/horus.exe
    parity: 17 of 17 cases equal
    $ python tools/goal_check.py --phase 2 --no-ci --server-cmd <repo>\bin\horus.exe mcp
    PASS  layout - 0 packages off target, 0 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 212 Go test functions (baseline 157); 0 changed contract files
    PASS  os-boundary - 0 allow list files; archtest: ok
    PASS  direction - 0 allow list files; archtest: ok
    PASS  process-tree - tree test: ok; 0 taskkill references; CI part skipped (--no-ci)
    FAIL  backend - Go tests ok; needs --real on the owner's PC for the image through the backend
    PASS  token - 0 allow list files; archtest: ok
    FAIL  commands - Go tests ok; needs --real on the owner's PC for horus doctor and horus setup
    goal: 6 of 8 rows passed

The rows `backend` and `commands` need `--real` (decision of T0). The new tests:

- `TestFailedDetailComesFromTheReasonsNotFromTheText` (`internal/provision`): a fake downloader that writes no text and returns a `*DownloadError` with 2 reason lines gives the detail of today.
- `TestDefaultLauncherNotOnThePathFails` and `TestDefaultLauncherIsTheProgramThatLookPathFinds` (`internal/provision`): a fake `Programs`.
- `TestPythonCommandFollowsTheOS` (`internal/platform`).
- `TestFailedAttemptsGiveTheirLinesAsReasons` (`internal/download`).

`TestInstallDownloadsEachFileAndKeepsTheKindOfAFailure` keeps its assertions. Its fake downloader now also returns the reason line that it writes, as the real downloader does.

Stub check 1: `failDetail` ignores `Reasons`. This is the result of a service that still filters the text, because the fake writes no text:

    --- FAIL: TestInstallDownloadsEachFileAndKeepsTheKindOfAFailure (0.01s)
        provision_test.go:209: detail "FAIL: Download did not finish after 4 attempts."
    --- FAIL: TestFailedDetailComesFromTheReasonsNotFromTheText (0.00s)
        provision_test.go:244: detail
            "FAIL: Download did not finish after 4 attempts."
            want
            "Attempt 1 of 4 did not finish (the server returned error: 403, 0 of 21 bytes). | The server answered with an error. HTTP 401 = token missing or wrong. HTTP 403 = license not accepted for this account. | FAIL: Download did not finish after 4 attempts."
    FAIL	github.com/hieplam/horus/internal/provision	0.720s

Stub check 2: `InstallComfyMCP` looks on the real PATH (`platform.Programs{}.LookPath` in place of `programs.LookPath`). The run used a PATH without `py.exe`, so that no real Python or pip could start:

    --- FAIL: TestDefaultLauncherNotOnThePathFails (0.00s)
        comfymcp_test.go:185: looked up [], want [py.exe]
    --- FAIL: TestDefaultLauncherIsTheProgramThatLookPathFinds (0.00s)
        comfymcp_test.go:202: The Python Launcher (py.exe) was not found. Install Python 3.12 from python.org.
    FAIL	github.com/hieplam/horus/internal/provision	0.580s

Ratchet: places where a service parses the text of an adapter or reaches `os/exec` through a base package: 2 -> 0. Go test functions (`git grep -h "^func Test" -- '*.go' | wc -l`): 212 -> 217 after the rebase on `e8cfd6b` (210 -> 215 on `d9827a2`).

## Issue 89

The port left names that no other package uses, three copies of small helpers, and comments that name files that do not exist any more. The decisions are the `#89` rows of the Decisions table. The issue was written on `d9827a2`. Four fixes merged after that (#90, #86, #88, #87). Every name of the issue was still there on `72a47c5`, so all of them were fixed.

Red, before the change (on `72a47c5`):

    $ git grep -n "^func NoConsole\|^func isFile\|^func Validate\|^func Round1\|horus/models.py reads" -- internal/platform/platform.go internal/modelfiles internal/generate/generate.go internal/comfy internal/download
    internal/comfy/jobs.go:131:func Round1(x float64) float64 {
    internal/download/download.go:49:// Kind is the outcome of a failed download. horus/models.py reads four of them
    internal/download/download.go:305:// it holds "error: <code>", the text that horus/models.py reads.
    internal/generate/generate.go:190:func Validate(w *catalog.Workflow, r Request) (*checked, error) {
    internal/modelfiles/modelfiles.go:268:func isFile(p string) bool {
    internal/platform/platform.go:60:func NoConsole(cmd *exec.Cmd) { noConsole(cmd) }
    $ go doc -short ./internal/comfy
    func ExecutionSeconds(e *Entry) *float64
    func JobError(promptID string, status Status) *herr.Error
    func RejectedGraphError(reply *catalog.Object) *herr.Error
    func Round1(x float64) float64
    type Client struct{ ... }
        func New(u string) *Client
    type Entry struct{ ... }
    type HTTPError struct{ ... }
    type Status struct{ ... }

Green, after the change:

    $ git grep -n "^func NoConsole\|^func isFile\|^func Validate\|^func Round1\|horus/models.py reads" -- internal/platform/platform.go internal/modelfiles internal/generate/generate.go internal/comfy internal/download; echo "lines=$(git grep -n '^func NoConsole\|^func isFile\|^func Validate\|^func Round1\|horus/models.py reads' -- internal/platform/platform.go internal/modelfiles internal/generate/generate.go internal/comfy internal/download | wc -l)"
    lines=0
    $ go doc -short ./internal/comfy
    type Client struct{ ... }
        func New(u string) *Client
    $ go doc ./internal/comfy Client | grep -c "GetJSON\|PostJSON"
    0
    $ gofmt -l .
    $ go vet ./...
    $ GOOS=linux GOARCH=amd64 go vet ./... && GOOS=darwin GOARCH=arm64 go vet ./... && echo vet-ok
    vet-ok
    $ go test ./...
    ok  	github.com/hieplam/horus/cmd/horus
    ok  	github.com/hieplam/horus/internal/archtest
    ok  	github.com/hieplam/horus/internal/backend
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
    $ python tests/mcp_contract.py --server-cmd <repo>\bin\horus.exe mcp
    contract: 30 of 30 passed
    $ python tests/workflow_parity.py --horus bin/horus.exe
    parity: 17 of 17 cases equal
    $ python scripts/lint-public.py
    lint-public: 0 finding(s)
    $ python tools/goal_check.py --phase 2 --no-ci --server-cmd <repo>\bin\horus.exe mcp
    PASS  layout - 0 packages off target, 0 listed packages missing
    PASS  same-behaviour - contract: local 30 of 30; CI part skipped (--no-ci); parity: 17 of 17; go test ./...: ok; 215 Go test functions (baseline 157); 0 changed contract files
    PASS  os-boundary - 0 allow list files; archtest: ok
    PASS  direction - 0 allow list files; archtest: ok
    PASS  process-tree - tree test: ok; 0 taskkill references; CI part skipped (--no-ci)
    FAIL  backend - Go tests ok; needs --real on the owner's PC for the image through the backend
    PASS  token - 0 allow list files; archtest: ok
    FAIL  commands - Go tests ok; needs --real on the owner's PC for horus doctor and horus setup
    goal: 6 of 8 rows passed

The rows `backend` and `commands` need `--real` (decision of T0).

Stub check: doing nothing leaves the 6 lines of the first `git grep` and the 9 lines of `go doc`, so Green fails. A stub also fails: an empty `graph.Round1` fails `TestRound1RoundsTheExactBinaryValueHalfToEven`, and a `comfy.getJSON` that does nothing fails the tests of `comfy` and the contract test.

Go test functions: 217 -> 218 (`git grep -h "^func Test" -- '*_test.go' | wc -l`). No test was deleted. The new one is `TestRound1RoundsTheExactBinaryValueHalfToEven` in `internal/graph`. No test file lost an assertion: the changes to test files are renames of unexported names and comments.

Ratchet (numbers from commands, on `72a47c5` and after the change):

| Measure | Command | Before | After |
|---|---|---|---|
| Names that the issue lists | the Red `git grep` and `go doc` above | 6 lines and 9 lines | 0 lines and 2 lines |
| Stale comments: `.ps1`, `horus/<x>.py`, `scripts/<x>.py`, `tools/make_<x>.py`, `<x>.py` of the old server, `asyncio`, `pydantic` | `git grep -nE '\.ps1\|horus/[a-z_]+\.py\|scripts/[a-z-]+\.py\|tools/make_[a-z_]+\.py\|\b(server\|comfy\|catalog\|models\|settings\|errors\|generate)\.py\b\|asyncio\|pydantic' -- '*.go' \| wc -l` | 125 | 4 |
| Exported names that no other package references (a Go AST walk over `internal/` and `cmd/horus`, qualified use `pkg.Name` for names, and the method name for methods; the script is not in the repo) | scratch tool | 114 | 75 |
| Definitions of the helpers `isFile`, `splitLines`, `round1` and `bom` | `git grep -nE '^func (isFile\|IsFile\|splitLines\|SplitLines\|[rR]ound1)\b\|^var bom' -- '*.go' \| wc -l` | 10 | 4 |

The 4 comments that remain are in `internal/herr/herr_test.go`: the test checks that no `next_step` names a `.ps1` script. The 75 names that remain are the types, interfaces and sentinel errors in signatures, the groups of values, and the names that a test of another package uses (see the Decisions rows). The walk matches a name by `pkg.Name`, so a type that other packages use only through an interface is counted. That is why 75 is not 0.

## T11

Result: the commands, the docs and the MCP tools do what the owner ratified. A real agent harness listed the workflows, read the rules, got an error that it could act on, and made one image that shows the subject. The design review found 5 defects against the design rules (#86 to #90). They block T11. One docs defect is a follow-up outside the epic (#91).

Fresh build of `origin/main` = `d9827a2`, in a worktree:

    go build -o bin/ ./cmd/horus
    go version -m bin/horus.exe
        build	vcs.revision=d9827a23f20ded61ff324c60195b24bc2d8da93c
        build	vcs.modified=false

Every command below ran with `HORUS_ROOT` = the main checkout (it holds `.env`; nothing there changed). The owner's ComfyUI answered on port 8188 the whole time (ComfyUI 0.44.0, pid 14432 before and after each step).

### Part A: a new command-line user

`horus -h` (exit 0):

    Usage: horus <command> [flags]

    Commands:
      check-setup         one PASS or FAIL line for each check of the setup
      doctor              one PASS or FAIL line for each check of the setup
      download-model      download one model file with size and SHA-256 checks
      errors              print the table of error codes (-json for JSON)
      install-comfy-mcp   install comfy-mcp and comfy-cli in their own venv
      install-comfyui     install the ComfyUI portable build, Windows only
      mcp                 serve the 5 horus tools over stdio (MCP)
      new-workflow        make a new workflow folder from the template
      register            register horus mcp with Claude Code
      run-illustration    make one image without MCP
      setup               print what this machine needs for a workflow; -yes installs it
      start-comfyui       start ComfyUI and wait for its server, Windows only
      stop-comfyui        stop the ComfyUI that horus started, Windows only
      sync-workflows      copy the workflows into the ComfyUI page
      workflow            list, check, install or remove the model files of a workflow

    Run horus <command> -h for the flags of a command.

`horus <command> -h` for each of the 15 commands: each exits 0, and each text names the flags that the command reads. The first lines (trimmed; the full texts list every flag):

    check-setup -h        Usage of horus check-setup:  -comfyui-dir, -models-dir, -quick, -server-url (default "http://127.0.0.1:8188"), -text-encoder-dir, -venv-path, -workflow      exit 0
    doctor -h             Usage of horus doctor: (the same 7 flags)                                                                                                                 exit 0
    download-model -h     Usage: horus download-model -url <https address> -destination <file> -expected-bytes <n> -sha256 <hex>                                                    exit 0
    errors -h             Usage of horus errors:  -json  print the table as one JSON object: code -> retryable, action_by, next_step                                               exit 0
    install-comfy-mcp -h  Usage of horus install-comfy-mcp:  -python-version (default "3.12"), -venv-path                                                                            exit 0
    install-comfyui -h    Usage of horus install-comfyui:  -archive-path, -install-dir, -keep-archive, -seven-zip                                                                  exit 0
    mcp -h                Usage: horus mcp / Serves list_workflows, describe_workflow, install_workflow, uninstall_workflow and generate_image over stdio.                         exit 0
    new-workflow -h       Usage of horus new-workflow:  -description, -model-family (default "TBD"), -name, -workflows-dir                                                        exit 0
    register -h           Usage of horus register:  -horus-path, -root                                                                                                            exit 0
    run-illustration -h   Usage of horus run-illustration:  -height, -out, -seed, -steps, -style-file, -subject, -subject-file, -timeout, -url, -width, -workflow                 exit 0
    setup -h              Usage: horus setup <workflow> [-yes] [flags] / Print what this machine needs for the workflow. With -yes, install it: ComfyUI first, then the model files.  exit 0
    start-comfyui -h      Usage of horus start-comfyui:  -comfyui-dir, -server-url, -timeout-seconds (default 300)                                                                 exit 0
    stop-comfyui -h       Usage of horus stop-comfyui:  -comfyui-dir, -server-url                                                                                                  exit 0
    sync-workflows -h     Usage of horus sync-workflows:  -comfyui-dir, -force, -workflows-dir                                                                                    exit 0
    workflow -h           Usage: horus workflow list|check|install|remove [<workflow>] [flags]                                                                                     exit 0

`horus doctor` (exit 0). `horus check-setup` printed the same lines (`diff` printed nothing, exit 0):

    PASS  ComfyUI folder - <ComfyUI dir>
    PASS  ComfyUI server - answers at http://127.0.0.1:8188 (ComfyUI 0.44.0)
    PASS  comfy-mcp environment - ~\.venvs\comfy-mcp\Scripts\comfy-mcp.exe
    PASS  comfy-cli - ~\.venvs\comfy-mcp\Scripts\comfy.exe
    PASS  .mcp.json - command points at ~\.venvs\comfy-mcp\Scripts\comfy-mcp.exe, COMFY_BIN at ~\.venvs\comfy-mcp\Scripts\comfy.exe
    PASS  horus on PATH - ~\go\bin\horus.exe
    PASS  .mcp.json horus - command points at ~\go\bin\horus.exe
    All checks passed.

`horus doctor -workflow it-illustration -quick` (exit 0): the same 7 lines, then the 9 lines of the workflow check below with an indent of 6 spaces, then:

    PASS  models for 'it-illustration' - all files present, size checked (the SHA-256 is not checked; drop -quick)
    All checks passed.

`horus workflow list` (exit 0):

    it-illustration
      Text in, image out. The picture is 1536x1024. The model is FLUX 3 Image Klein (int8 build) running in ComfyUI. This is the first horus example.
      Models: 3 file(s), 11.02 GB (diffusion, text_encoder, vae). Installed: yes (size check).

    Install: horus workflow install <name>    Check: horus workflow check <name>

`horus workflow check it-illustration -quick` (exit 0):

    Checking workflow 'it-illustration': 3 model file(s), size only.
    OK        diffusion ~\ComfyUI-models\diffusion_models\flux3-image-klein-int8-convrot.safetensors (5827874779 bytes)
    OK        text_encoder ~\ComfyUI-models\text_encoders\qwen3vl_4b_int8_convrot.safetensors (4858559240 bytes)
    OK        vae ~\ComfyUI-models\vae\flux2-vae.safetensors (336211292 bytes)
    OK        models.json lists all 3 model file(s) that workflow.json loads
    OK        the server has all 10 node classes of workflow.json
    OK        the server lists flux3-image-klein-int8-convrot.safetensors
    OK        the server lists qwen3vl_4b_int8_convrot.safetensors
    OK        the server lists flux2-vae.safetensors
    OK: workflow 'it-illustration' is ready.

`horus setup it-illustration`, without `-yes` (exit 0):

    Setup of workflow 'it-illustration':
    PRESENT   ComfyUI in <ComfyUI dir>
    PRESENT   diffusion flux3-image-klein-int8-convrot.safetensors (5.83 GB) ~\ComfyUI-models\diffusion_models\flux3-image-klein-int8-convrot.safetensors
    PRESENT   text_encoder qwen3vl_4b_int8_convrot.safetensors (4.86 GB) ~\ComfyUI-models\text_encoders\qwen3vl_4b_int8_convrot.safetensors
    PRESENT   vae flux2-vae.safetensors (0.34 GB) ~\ComfyUI-models\vae\flux2-vae.safetensors
    Plan: 0 steps. Nothing is missing.

`horus errors -json` (exit 0): one JSON object with 21 codes: `BACKEND_NOT_INSTALLED`, `BACKEND_RESTART_REQUIRED`, `BACKEND_UNAVAILABLE`, `CANCELLED`, `CONFIRMATION_REQUIRED`, `DISK_FULL`, `DOWNLOAD_FAILED`, `FILE_CONFLICT`, `FILE_IN_USE`, `GENERATION_FAILED`, `GENERATION_TIMEOUT`, `HF_TOKEN_INVALID`, `INTERNAL`, `INVALID_ARGUMENT`, `LICENSE_CONFIRMATION_REQUIRED`, `LICENSE_NOT_ACCEPTED`, `NODE_PACK_MISSING`, `OUTPUT_EXISTS`, `OUT_OF_MEMORY`, `WORKFLOW_NOT_FOUND`, `WORKFLOW_NOT_INSTALLED`.

Wrong input: `horus nosuch` prints `horus: unknown command "nosuch"` and the usage (exit 2). `horus setup` without a workflow prints its usage (exit 2). `horus workflow check nosuch -quick` prints `FAIL: No workflow 'nosuch'. Known: it-illustration` (exit 1).

`horus stop-comfyui` against the owner's ComfyUI. The proof before the run: `Stop` (`internal/backend/portable.go:476-492`) reads `horus-comfyui.state.json` in the install folder. When no state file names the same address, it returns the refusal before it calls `platform.EndTree`. The owner's install folder had no state file (only the two log files of the T7 run). `TestStopRefusesAComfyUIThatHorusDidNotStart` passed on this commit (`--- PASS ... (1.91s)`). The run:

    pid on port 8188 before: 14432
    horus stop-comfyui
    horus stop-comfyui: horus did not start the ComfyUI that answers at http://127.0.0.1:8188 (<ComfyUI dir>\horus-comfyui.state.json holds no ComfyUI that horus started), so it does not stop it. Stop it where it was started.
    exit 1
    pid on port 8188 after: 14432

**`README.md` and `docs/setup.md` as a reader.** Every relative link of `README.md`, `docs/setup.md` and `docs/mcp.md` points at a file that exists (17 links), and the 3 anchors that they name exist (`#step-0-get-the-horus-program`, `#manage-workflows-list-install-check-remove`, `#optional-the-image-tool-for-agents-in-other-folders`). Every path and package that the text names exists: `.env.example`, `.mcp.json`, `workflows/it-illustration/{models.json,manifest.json,README.md}`, `workflows/_template/README.md`, `tests/{mcp_contract.py,workflow_parity.py,requirements.txt}`, `scripts/lint-public.py`, `tools/goal_check.py`, and the packages `internal/{mcpserver,provision,generate,modelfiles,download,comfy,backend,herr,archtest}`. Every command that they name exists in `horus -h`, and the read-only ones behave as the text says (above). Commands that change files were not run on this PC (`install-comfyui`, `workflow install`, `workflow remove`, `register`, `install-comfy-mcp`, `sync-workflows`, `setup -yes`); their behaviour is proven by the Go tests and the parity test. The mismatches (#91, a follow-up):

- `docs/setup.md:18`: "Two commands are Windows-only in this phase: `horus install-comfyui`, `horus start-comfyui` and `horus stop-comfyui`." It names three.
- `docs/setup.md:112`: "`horus setup it-illustration` does steps 1 and 4 in one command." It does not run `start-comfyui` (step 1), `sync-workflows` or `doctor -workflow` (step 4). The same paragraph says only that it does not start ComfyUI.
- The help texts write the home folder as `<home>/ComfyUI_windows_portable` (6 commands) and as `~/ComfyUI_windows_portable` (`setup`, `workflow`).
- Known before (#82): `horus doctor -server-url` defaults to `http://127.0.0.1:8188`, not to `HORUS_COMFYUI_URL`.

### Part B: a new agent through MCP

Harness: Claude Code 2.1.322, `claude -p` in an empty folder of the scratch area, with `--mcp-config <scratch>\mcp-horus.json --strict-mcp-config --tools "" --allowedTools mcp__horus__list_workflows,mcp__horus__describe_workflow,mcp__horus__generate_image --disallowedTools mcp__horus__install_workflow,mcp__horus__uninstall_workflow --output-format stream-json --verbose`. The config starts `<worktree>\bin\horus.exe mcp` with `HORUS_ROOT` = the main checkout, `HORUS_COMFYUI_DIR=<scratch>\comfy8189`, `HORUS_COMFYUI_URL=http://127.0.0.1:8189`, `HORUS_OUTPUT_DIR=<scratch>\out` and `HORUS_LOG_FILE`. The session saw 3 tools and the server `horus` as `connected`.

`<scratch>\comfy8189` is the second install folder of the T7 method: a junction `python_embeded` to the owner's install, a folder `ComfyUI` with 22 junctions (each folder of the owner's `ComfyUI` except `user`, `input`, `output`, `temp`), 4 empty folders of its own and 36 copied small files.

A first try from PowerShell 5.1 got "Claude requested permissions to use mcp__horus__list_workflows, but you haven't granted it yet" for each call: PowerShell 5.1 drops the empty argument of `--tools ""`, so the flags after it were read wrong. The runs below used `sh`. This is a fault of the command line, not of horus.

**Session 1 (19:45, 16 s): list, describe, one error on purpose.** The prompt, in plain words: list the workflows, describe it-illustration and say what sizes it allows and how to write a subject, then call `generate_image` with width 4096 and height 1024 and show the error fields. The calls and the results (trimmed):

    CALL list_workflows {}
    RESULT {"ok":true,"result":{"workflows":[{"name":"it-illustration","title":"FLUX 3 Image Klein, text to image", ... "install_state":"installed","download_bytes":11022645311}],"backend_running":false},"error":null}
    CALL describe_workflow {"workflow": "it-illustration"}
    RESULT {"ok":true,"result":{"name":"it-illustration", ... "output":{"format":"png","default_width":1536,"default_height":1024,"min_side":512,"max_side":1536,"multiple_of":16, ...},"subject":{"min_chars":40,"max_chars":4000, ...
    CALL generate_image {"workflow": "it-illustration", "subject": "a small test picture of one red cube on a white table", "width": 4096, "height": 1024}
    RESULT {"ok": false, "result": null, "error": {"code": "INVALID_ARGUMENT", "message": "width 4096 is not allowed", "retryable": false, "action_by": "caller",
            "next_step": "Fix the argument named in details.field, then call again.", "details": {"field": "width", "got": 4096, "max": 1536, "min": 512, "multiple_of": 16}}}

The agent read it correctly without help: "`action_by: "caller"` means that I must change my arguments", and it proposed 1536 x 512 (the nearest allowed shape) or the tested 1536 x 1024. The error has a code, `action_by` and a `next_step` that a new agent can act on, and `details` holds the limits. The server log has `call generate_image failed: INVALID_ARGUMENT: width 4096 is not allowed`. The argument check runs before any start of ComfyUI, so this call used no GPU.

GPU before the image (`nvidia-smi --query-gpu=memory.used,utilization.gpu --format=csv,noheader`): `0 MiB, 0 %` at 19:43 and at 19:46. Ports at 19:46: 8188 pid 14432, 8189 closed.

**Session 2 (19:46:39 to 19:50:47): one image.** The prompt asked for one picture with it-illustration, the default size, seed 31, and this subject, which T11 wrote:

> A flat vector illustration with clean dark navy outlines and solid flat colours on a plain cream background. On the left side, a tall orange lighthouse with one wide white stripe stands on a small grey rock. On the right side, a small teal sailboat with one white triangular sail floats on calm light-blue water. In the top center, a round yellow sun shines. Between the lighthouse and the boat, three white seagulls fly in a row. The picture contains no text of any kind: no letters, no digits, no symbols and no logos.

    CALL describe_workflow {"workflow": "it-illustration"}
    CALL generate_image {"workflow": "it-illustration", "subject": "<the subject above, exactly>", "seed": 31}
    RESULT {"ok":true,"result":{"workflow":"it-illustration","path":"<scratch>\\out\\it-illustration\\20261111T124645Z-31.png","bytes":1289411,"width":1536,"height":1024,"seed":31,
            "seconds":236.9,"backend_seconds":213.5,"started_backend":true,"job_id":"2b693af1-c509-4f3d-a199-feccff4e06c5"},"error":null}
    END success, 3 turns, 246693 ms

The call waited for the PNG and returned its path in the same session (no move to a background task). The server log has 29 progress lines. The backend started ComfyUI on port 8189 (`started_backend: true`).

The image (`<scratch>\out\it-illustration\20261111T124645Z-31.png`, 1536 x 1024), opened and judged: on the left, a tall orange lighthouse with one wide white stripe stands on grey rocks at the water line. On the right, a teal boat with one white triangular sail floats on light-blue water, and a round yellow sun is at the top center. Three white seagulls fly in a row between them, on a cream background with dark navy outlines and no text. It shows every object, colour and position of the subject. The boat is medium-sized rather than small.

After the run (19:51), with the same settings:

    horus stop-comfyui
    OK: stopped the ComfyUI that horus started at http://127.0.0.1:8189 (process 13376).
    exit 0
    8189 listeners: 0
    processes with comfy8189 in the command line: 0
    port 8188 pid: 14432
    nvidia-smi: 0 MiB, 0 %

So the ComfyUI that the server started kept running after `claude -p` and `horus mcp` ended, and `Backend.Stop` ended it from another process. T7 and T10 could not show this through MCP, because the Python MCP client ends its Job Object.

Cleanup: the 23 junctions (22 + 1) were deleted one at a time with `(Get-Item <junction>).Delete()`, which removes only the link. A recursive listing then found 0 links. The 43 files were deleted one by one, then the 8 empty folders, each with a non-recursive delete. After the cleanup:

    Test-Path <ComfyUI dir>\ComfyUI\main.py              True
    Test-Path <ComfyUI dir>\python_embeded\python.exe    True
    entries in <ComfyUI dir>\ComfyUI\models              26
    horus workflow check it-illustration -quick          exit 0, the same 9 lines as in part A:
      5827874779, 4858559240 and 336211292 bytes, equal to workflows/it-illustration/models.json
    port 8188 pid: 14432

### Part C: the design, package by package

Exported surface from `go doc -short ./internal/<pkg>` (functions, types, and the methods of each interface). "Used" means that the service calls the method, and "fake" names the test double.

| Package | Layer | What it hides | Exported surface | Verdict |
|---|---|---|---|---|
| `cmd/horus` | wiring | The creation of each adapter for the flags of a command. | none (`main.go`: 15 commands; `wiring.go`: 128 lines) | The glue `downloader` (a copy of the error kind) is fine. The glue `workflowCheck` builds a command line, calls the handler `cli.Workflow` and splits its text into lines for the doctor service: logic in the wiring, and a call from a service back into a handler (#86). |
| `internal/cli` | handler | Flags, text lines, exit codes. | 13 handler functions, `Illustrate`, 5 interfaces (`Backend` 3, `Doctor` 1, `Images` 1, `Setup` 4, `SetupSteps` 2), 6 structs, `Mark` | Flags in, text out. Each interface has a fake in its tests. The calls to the adapter `backend` through `cli.Backend` are a recorded T7 design (see Decisions). |
| `internal/mcpserver` | handler | The MCP protocol: schemas, envelope, progress, the mapping of typed results to the 21 codes. | `New`, `Run`, 2 interfaces (`Images` 3, `Setup` 5), 8 result types, 3 values | The error mapping is protocol work and fits here. `describe_workflow` has its own copy of the `install_state` rule of `provision.Row.State` (#88). |
| `internal/generate` | service | The steps of one image: checks, graph, wait loop, time limits, PNG file. | `New`, `Service` (`Generate`, `Running`, `FreeMemory`), `ComfyUI` 9, `Backend` 1, `Request`, `Result`, `Options`, `Report`, `Validate`, 3 vars | `ComfyUI` has 9 methods and the service uses all 9 (`IsUp`, `Address`, `ObjectInfo`, `Submit`, `History`, `QueueState`, `Cancel`, `View`, `Free`); fake `fakeComfy` implements all 9. `Backend.Start` is used; fake `fakeBackend`. Defect: `QueueState` returns the text code `pending:<n>` that the service and the adapter parse (#90). `Validate` is exported but returns an unexported type (#89). |
| `internal/provision` | service | Which file to download, where, which folder to map, which file may be deleted, the plan of `horus setup`. | `Service` (`List`, `Check`, `Install`, `Remove`, `PartSize`, `Plan`, `RunPlan`), `InstallComfyMCP`, `Register`, `CheckRoot`, 5 interfaces, 14 types | `ModelFiles` 7 (all used; fakes `memFiles`, `sizedFiles`, `diskFiles`), `Downloader` 1 (`fakeDownloader`, `slowDownloader`), `ComfyUI` 1 (`fakeComfy`, `noAnswer`), `Backend` 2 (`recordBackend`), `Programs` 2 (cli `fakePrograms`). The decisions are pure functions (`removeDecision`, `fileCheck`, `mappings`, `steps`): good. Defects: the service picks lines of the downloader's text with a regular expression, and it finds Python through `platform.PythonLauncher`, which calls `exec.LookPath` (#87). |
| `internal/doctor` | service | The 8 checks of `horus doctor`. | `New`, `Service.Check`, `Failed`, 4 interfaces (`Server` 1, `Install` 1, `Programs` 1, `Models` 1), `Options`, `Result` | All 4 interfaces used and faked. Defect: `Models.Check` returns the text lines of another command (#86). |
| `internal/devtools` | service | The new-workflow template and the copy into the ComfyUI pane. | `NewWorkflow`, `Sync`, `Install` 1, `Copy`, 4 consts | Fine. File work only, one interface with a fake. |
| `internal/comfy` | adapter | The ComfyUI REST API, its JSON, the `client_id`, the mapping of a failed job to a horus error. | `Client` (12 methods), `New`, plus 7 names and 2 methods that no other package uses | Deep for its callers. 9 exports have no caller outside the package, and `Round1` is a second copy of `generate.round1` (#89). |
| `internal/download` | adapter | Resume, size and SHA-256 checks, the token and its hosts. | `Download`, `Main`, `TokenAllowed`, `Options`, `Result`, `Error`, `Kind`, `SpareBytes` | Fine; the only reader of `HF_TOKEN`. Two comments still say that `horus/models.py` reads its text (#89). |
| `internal/modelfiles` | adapter | The folder order, `extra_model_paths.yaml` with its backup, hashes, delete of a read-only file. | `Store` with 7 methods | Fine. One dead function, `isFile` (#89). |
| `internal/backend` | adapter | 7-Zip, the pinned archive, the start of the process tree, the state file, the refusal of `Stop`. | `Backend` 5, `New`, `Portable`, `Layout`, `NotInstalledError`, consts | Fine. `Stop` refused the owner's ComfyUI on this PC (part A) and stopped its own (part B). |
| `internal/platform` | base | Job Objects and process groups, free space, OS paths. | 9 functions, `Programs` (2 methods), `Command`, 4 error values, `EndWait` | One free-space function for its 2 callers. `NoConsole` has no caller (#89). `PythonLauncher` runs `exec.LookPath` for a service (#87). |
| `internal/catalog` | base | `manifest.json` and `models.json`, and the rules of the old script for each key. | `Workflow`, `All`, `Get`, `DetailOf`, `SummaryOf`, `Folders`, `FindModelList`, `ReadModelList`, `ReadGraphRefs`, `Object`, 10 types | Two readers of `models.json` with their own error texts. A T6 decision keeps both, because the golden files hold both texts. Fine. |
| `internal/graph` | base | The UI-to-API graph conversion. Pure. | `UIToAPI`, `MissingModelFiles`, `API`, `Node`, `ObjectInfo`, `Job`, `Image`, `PyDumps`, `PyStr`, `Truthy`, `IsModelFile`, 3 tables | Fine. `Job` and `Image` are plain types that the service and the adapter share (T5 decision). |
| `internal/herr` | base | The 21 codes and the envelope error. | `Error`, `New`, `Codes`, `Lookup`, `TableJSON`, `Spec`, `Info` | Fine. |
| `internal/settings` | base | Environment, `.env`, the repo root. | 10 functions | Fine. One root finder and one `.env` parser; `internal/download` reads the token with `ParseEnv`. |
| `internal/archtest` | test | The rules of the spec. | `Scan`, `Problems`, `Rules`, `File`, `Violation`, 4 consts | Fine. Two places hold the rules by the letter only: #86 (a service gets the text of a handler at run time) and #87 (a service reaches `os/exec` through a base package). |

Checked and not found: an interface with no test double (each of the 19 interfaces of the services and the handlers has a fake in its tests, except `mcpserver.Setup`, whose tests use the real `provision.Service` on fake adapters), a second free-space check (one implementation in `platform`), a second way to find the repo root (only `settings.Root`), a second `.env` parser. `go vet ./...` is clean.

### Findings

Blocks #66 (each is a sub-issue of #52 and blocks #66):

- #86 `doctor`: the model check runs the handler of `horus workflow` and gives its text lines to the doctor service. Design rule 5, row `direction`.
- #87 `provision`: the service reads the text lines of the downloader, and finds Python through `exec.LookPath` in `internal/platform`. Design rules 2 and 4, row `direction`.
- #88 `mcpserver`: `describe_workflow` has its own copy of the `install_state` rule. Design rules 1 and 5.
- #89 dead code and exports that no other package uses, left by the port (`comfy`, `generate`, `platform`, `modelfiles`, `download`). Design rules 1 and 2.
- #90 `generate` and `comfy`: `QueueState` is a text code that two packages parse. Design rule 2.

Follow-up outside the epic:

- #91 `docs/setup.md` and the help texts: three sentences that the commands contradict.
- #82 (filed by T10) `horus doctor -server-url` ignores `HORUS_COMFYUI_URL`.

Ratchet: open defects against the goal rows and the design rules: unknown before T11 -> 5 found and open (#86 to #90). T11 merges when they are 0.

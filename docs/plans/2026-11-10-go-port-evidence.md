# Evidence log: port horus to Go (Phase 1)

Plan: [`2026-11-10-go-port.md`](2026-11-10-go-port.md). One section for each
task, with the literal output of its Red and Green commands.

## Decisions

Decisions made under the standing rules of the spec, newest last.

| Date | Task | Decision | Why |
|---|---|---|---|
| 2026-11-10 | plan | The `real-image` row does not install the model files from nothing. | The owner tests the install on a fresh machine. |
| 2026-11-10 | plan | Phase 1 has its own epic issue. Issue #4 stays the page of the whole goal. | The epic closes when its tasks close, and Phases 2 to 4 have no tasks yet. |
| 2026-11-10 | G0 | The `contract` row reads CI jobs by name: a job that starts with `contract` and has the system name (`windows-latest`, `macos-latest`) in it. | The plan names no job names. Task G8 must name its jobs this way. |
| 2026-11-10 | G0 | The Red of G0 ran in the main checkout, where `tools/goal_check.py` does not exist. | The worktree already had the file when the check ran. |
| 2026-11-10 | G1 | The golden files of G1 come from the Python code through `tools/make_server_golden.py`: the tool list of the Python server, the table of `horus/errors.py`, and `catalog.summary` and `catalog.detail` of `it-illustration`. Task G10 deletes this tool together with the Python server. | The plan asks for golden files made from the Python server and names no tool. The Go tests keep working after G10, because they read the committed golden files and not `horus/*.py`. |
| 2026-11-10 | G1 | Allowed difference: the `next_step` of `BACKEND_UNAVAILABLE` names `horus start-comfyui` instead of `scripts/start-comfyui.ps1`. The test `TestCodesMatchPython` allows only this replacement. | Rule 3 of the spec. |
| 2026-11-10 | G1 | Allowed difference: arguments that fail the input schema get the same answer shape as in Python: `isError`, no `structuredContent`, and the text `Error executing tool <name>: N validation error(s) for <name>Arguments` with the field and a message. The message lines after the field are close to the pydantic text, not word for word. Unknown arguments are ignored, as in Python. | The text comes from pydantic, a Python library. The contract test does not read it. |
| 2026-11-10 | G1 | Allowed difference: the `INTERNAL` message of a Go error is the error text, without the Python exception class name (`ValueError: ...`). | Go errors have no class name. |
| 2026-11-10 | G1 | A tool part that is not ported yet returns the `INTERNAL` envelope after the catalog part. `describe_workflow`, `install_workflow` and `uninstall_workflow` first check the name (`INVALID_ARGUMENT`, `WORKFLOW_NOT_FOUND`). `list_workflows` first reads the catalog. `generate_image` is not ported at all (task G2). The details name the task. | Plan step 6. |
| 2026-11-10 | G1 | The layout for the parallel lanes: each command is a file `cmd/horus/cmd_<name>.go` that calls `register` in `init`. A command that is not ported yet is one line in `cmd/horus/cmd_not_ported.go`. Each tool is a file `internal/server/tool_<name>.go`. The seams for the next tasks: `fileStates` in `tool_describe_workflow.go` (G5), the `body` of each tool, and `Reporter` in `progress.go` for progress notifications (G2, G5). | The coordinator asked for a shape that keeps the lanes out of shared files. A task that ports a command deletes one line in `cmd_not_ported.go`. |
| 2026-11-10 | G1 | `.gitattributes` keeps `*.go`, `go.mod` and `go.sum` at LF. The plan does not list this file. | `gofmt -l` lists a CRLF file, and a Windows checkout with `core.autocrlf` writes CRLF. |
| 2026-11-10 | G1 | Known difference, not fixed: `catalog.All` sorts the workflow folders by byte order. Python sorts `WindowsPath` objects without case on Windows. The order is the same for names in lower case, and `new-workflow` allows only kebab-case names. | Not forced by Go, but no shipped or allowed name shows the difference. |
| 2026-11-10 | G1 | The CI job `contract` runs on `windows-latest` and `macos-latest` with the name `contract <os>`. It prints the count and does not fail (until G8). | Plan step 7 and the G0 decision about job names. |
| 2026-11-10 | G6 | `install-comfyui` downloads with Go `net/http` (resume with a `Range` header, 3 attempts, 5 s and 10 s between them), not with `curl.exe`. The check for `curl.exe` is dropped. The size, the SHA-256, the free-space message and the call of `7z.exe` stay. | The plan names the archive pin, the free-space check and 7-Zip, not curl. A Go download is testable with a local HTTP server and needs no second program. |
| 2026-11-10 | G6 | Flags are Go style with the same meaning: `-install-dir`, `-archive-path`, `-seven-zip`, `-keep-archive`; `-comfyui-dir`, `-server-url`, `-timeout-seconds`; `-venv-path`, `-python-version`. A failure prints one line `horus <command>: <message>` to stderr and exits 1. A flag error exits 2. | Issue #13 and issue #4: same flags in Go style. The text of each message is the text of the script. |
| 2026-11-10 | G6 | `start-comfyui` passes `--port <port of -server-url>` as the script does, after the three arguments of the plan. The first check of the server comes after one poll interval (2 s), as in the script. | The script does this. The plan lists only the 3 fixed arguments. |
| 2026-11-10 | G6 | `install-comfy-mcp` also runs on macOS and Linux. It uses `python<version>` as the launcher and `bin/` in the venv there, and `py.exe -<version>` and `Scripts\` on Windows. The plan makes only `install-comfyui` and `start-comfyui` Windows-only. | The plan, step 3. |
| 2026-11-10 | G6 | The tests use the test binary as the fake `7z`, the fake `python.exe` and the fake Python launcher (`TestMain` with an environment variable). They run on Windows, macOS and Linux. The fake `python.exe` of `start-comfyui` waits 60 s and the test kills it by the process id from its record. | The coordinator asked for fakes that work on all three CI systems. |
| 2026-11-10 | G3 | The flags of `horus download-model`: `-url`, `-destination`, `-expected-bytes`, `-sha256`, `-token-env-var` (default `HF_TOKEN`), `-max-attempts` (default 4). A flag that does not parse exits 2, as every horus command does. A missing or wrong value prints a `FAIL:` line and exits 1, as the script did. | Rule 3 of the spec (Go style flags). Exit 2 for a parse error is the convention of `cmd/horus`. |
| 2026-11-10 | G3 | Allowed difference: the curl text is gone. The line `Attempt N of M did not finish (curl exit 22, ...)` is now `Attempt N of M did not finish (the server returned error: 401, ...)`, so the text `error: 401` and `error: 403` that `_download_error` in `horus/models.py` reads stays in the item detail. The hint line `curl exit 22 means ...` is now `The server answered with an error. HTTP 401 = ... HTTP 403 = ...`; it still names both codes, so it is still no evidence of either. | Rule 3 of the spec: no `curl.exe`. Task G4 must keep `Attempt ` and `FAIL:` lines in the detail of a `FAILED` item. |
| 2026-11-10 | G3 | The Go API for task G4: `downloadmodel.Download(ctx, Options) (Result, error)`. The error is an `*Error` with a `Kind`: `KindUnauthorized` (401), `KindForbidden` (403), `KindDiskFull`, `KindConflict`, `KindBadHash`, `KindFailed`. `Result.Skipped` is true for the `SKIP` case. `downloadmodel.Main` is the command. | The task asks for a small API that G4 calls. The four outcomes of the spec are values, and the text output stays the same as well. |
| 2026-11-10 | G3 | The retry loop of the script is kept as it was: 4 attempts, a wait of 5 s times the attempt number, also after HTTP 401 and 403. | The old code is right. A retry after 401 does no harm. |
| 2026-11-10 | G3 | The free space comes from a second set of OS files: `freespace_windows.go` (`GetDiskFreeSpaceExW`), `freespace_unix.go` (`darwin \|\| linux`, `syscall.Statfs`) and `freespace_other.go` (no value, so a warning, as the script did for a network path). They use only the standard library, so `go.mod` does not change. The check reads the nearest folder that exists, not the drive root. | Go has no portable call for free space, so Go forces this OS code. The spec names only the process tree as OS code; this is one more place. |
| 2026-11-10 | G3 | Three checks that the script did not have: an address with a user part (`https://user@host/`) is refused, because `net/http` sends the user part as a `Basic` Authorization header; the downloader checks again that the destination is absent just before the rename, because `os.Rename` replaces a file; and it reads at most one byte more than the expected size from an answer. | Rule 6 of the spec (no credential to another host) and "never overwrite a file that it did not write". `Move-Item` refused an existing file; `os.Rename` does not. |
| 2026-11-10 | G3 | The stub check found that Go's `net/http` itself drops the Authorization header on a redirect to a host that is not the first host or a subdomain of it. So the redirect test also passes without the own `CheckRedirect` rule. The own rule stays, because it also drops the header on a redirect to plain `http` and to a host that is not `huggingface.co`. The stub "always send the token" fails `TestTokenGoesOnlyToHuggingFaceHosts`. | Evidence for the Stub check, recorded so that nobody reads the redirect test as proof of the own rule alone. |

## Baseline (2026-11-10, commit `c5379f9`)

- Contract test against the Python server on the owner's PC: `contract: 30 of 30 passed`.
- `horus/*.py` plus `scripts/*.ps1`: 2,827 lines.

## G0

Red (before building, in the main checkout):

```
$ python tools/goal_check.py --no-ci
python.exe: can't open file '...\horus	ools\goal_check.py': [Errno 2] No such file or directory
exit 2
```

Green:

```
$ python tools/goal_check.py --no-ci
FAIL  contract - server command not found: horus
FAIL  graph - missing: ./internal/comfy
FAIL  workflow-cli - missing: tests/workflow_parity.py
FAIL  downloader - missing: ./internal/scripts/downloadmodel
FAIL  setup-commands - missing: go.mod
FAIL  builds - missing: go.mod or cmd/horus
FAIL  real-image - needs --real on the GPU machine
FAIL  no-python - 2827 lines left; 18 files of Python and PowerShell remain; .mcp.json horus entry is not `horus mcp`
goal: 0 of 8 rows passed
exit 1
```

Stub check on the contract row: with the Python server as the server command,
the same tool prints `PASS  contract - local 30 of 30; CI part skipped (--no-ci)`.
So the row measures the contract test. It does not print a fixed line.

Ratchet: `goal`: 0 of 8 rows (opening value). Contract against the Go binary: not started.
The CI job `python-baseline` result is in the pull request.

CI run 1 failed with `contract: 0 of 30 passed`: `FileNotFoundError` for the server. The test looks for `horus-mcp.exe` next to `python.exe`. On the runner the scripts folder is `Scripts` below the folder of `python.exe`. Decision: the job passes `--server-cmd horus-mcp` (the program on the PATH after `pip install -e .`). The test is not changed.

## G1

MCP Go SDK: `github.com/modelcontextprotocol/go-sdk` v1.8.0 (the newest version on 2026-11-10), with Go 1.27.3.

SDK feature check. Each feature has a Go test that uses the SDK through a real client session (in-memory transport):

| Feature | SDK API | Test | Result |
|---|---|---|---|
| An output schema for each tool | `mcp.Tool.OutputSchema` (raw JSON) | `TestToolsMatchPython` | supported |
| `structuredContent` with `isError` | `mcp.CallToolResult.StructuredContent`, `IsError` | `TestErrorEnvelope` | supported |
| The annotations `readOnlyHint` and `destructiveHint` | `mcp.ToolAnnotations.ReadOnlyHint`, `DestructiveHint` | `TestToolsMatchPython` | supported |
| The server `instructions` | `mcp.ServerOptions.Instructions` | `TestToolsMatchPython` | supported |
| Progress notifications | `CallToolParams.GetProgressToken`, `ServerSession.NotifyProgress` | `TestSDKProgressNotifications` | supported |

No feature is missing, so no issue was filed.

Red (before building, in the worktree on `origin/main` at `d80128a`):

```
$ go build ./cmd/horus
go: go.mod file not found in current directory or any parent directory; see 'go help modules'
exit 1
$ python tools/goal_check.py --no-ci
FAIL  contract - server command not found: horus
FAIL  graph - missing: ./internal/comfy
FAIL  workflow-cli - missing: tests/workflow_parity.py
FAIL  downloader - missing: ./internal/scripts/downloadmodel
FAIL  setup-commands - missing: go.mod
FAIL  builds - missing: go.mod or cmd/horus
FAIL  real-image - needs --real on the GPU machine
FAIL  no-python - 2827 lines left; 18 files of Python and PowerShell remain; .mcp.json horus entry is not `horus mcp`
goal: 0 of 8 rows passed
exit 1
```

Green:

```
$ python tools/goal_check.py --no-ci
FAIL  contract - server command not found: horus
FAIL  graph - missing: ./internal/comfy
FAIL  workflow-cli - missing: tests/workflow_parity.py
FAIL  downloader - missing: ./internal/scripts/downloadmodel
FAIL  setup-commands - pattern ./internal/scripts/...: GetFileAttributesEx .\internal\scripts\: The system cannot find the file specified.
PASS  builds - 3 of 3 build
FAIL  real-image - needs --real on the GPU machine
FAIL  no-python - 2827 lines left; 18 files of Python and PowerShell remain; .mcp.json horus entry is not `horus mcp`
goal: 1 of 8 rows passed
exit 1
```

```
$ go build -o bin/horus.exe ./cmd/horus
$ python tests/mcp_contract.py --server-cmd <worktree>/bin/horus.exe mcp
PASS  server identifies as horus and has instructions
PASS  tools/list has exactly the 5 tools
PASS  every tool has an outputSchema with ok, result, error
PASS  uninstall_workflow is marked destructive, list_workflows read-only
FAIL  list_workflows shows installed and partially_installed styles  -- {"ok": false, "result": null, "error": {"code": "INTERNAL", "message": "list_workflows is not ported to Go yet: the install state of each workflow comes in task G5", ...}}
FAIL  describe_workflow gives subject rules, examples and 3 present files  -- {... "code": "INTERNAL", ...}
PASS  list_workflows and describe_workflow carry no theme or look
PASS  describe_workflow hides the ComfyUI graph (no node class names)
PASS  unknown workflow -> WORKFLOW_NOT_FOUND with the known names, isError
FAIL  (16 checks of generate_image, install_workflow and uninstall_workflow: INTERNAL, not ported yet)
PASS  errors are the same envelope in structuredContent and in text
FAIL  backend down and autostart off -> BACKEND_UNAVAILABLE, action_by=human
PASS  it-illustration: manifest has no theme and no caption style file
PASS  it-illustration: the graph holds no style words
PASS  docs/mcp.md lists every error code
contract: 11 of 30 passed
exit 1
```

The 6 checks that the plan names pass. Two more checks pass for a weak reason, so task G5 must prove them again:

- "carry no theme or look" passes because no tool returns a result yet.
- "hides the ComfyUI graph" passes because the answer is an `INTERNAL` envelope.

```
$ go test -count=1 -v ./...   (only the result lines)
--- PASS: TestEveryPortMapRowHasACommand
--- PASS: TestExitCodes
ok  	github.com/hieplam/horus/cmd/horus
--- PASS: TestShippedWorkflowMatchesPython
--- PASS: TestManifestChecks
--- PASS: TestFoldersThatAreNotOffered
--- PASS: TestGetErrors
--- PASS: TestLicenseWithoutModelsFile
--- PASS: TestPyRepr
ok  	github.com/hieplam/horus/internal/catalog
--- PASS: TestCodesMatchPython
--- PASS: TestInfoDefaultsAndOverrides
--- PASS: TestUnknownCodePanics
--- PASS: TestTableJSON
ok  	github.com/hieplam/horus/internal/herr
--- PASS: TestToolsMatchPython
--- PASS: TestErrorEnvelope
--- PASS: TestNotPortedToolsAnswerInternal
--- PASS: TestArgumentErrors
--- PASS: TestSDKProgressNotifications
--- PASS: TestTitle
ok  	github.com/hieplam/horus/internal/server
--- PASS: TestEnvironmentWinsOverDotEnvThenDefault
--- PASS: TestParseEnv
--- PASS: TestGetRefusesTheToken
--- PASS: TestDefaultsUseRepoRoot
--- PASS: TestRootFindsWorkflowsFolderAboveTheCurrentFolder
--- PASS: TestRootFallsBackToTheExecutableFolder
ok  	github.com/hieplam/horus/internal/settings
```

- `TestCodesMatchPython` compares the 21 codes, `retryable`, `action_by` and `next_step` with `horus/errors.py` (golden file).
- `TestToolsMatchPython` compares the input schema, the output schema, the annotations, the title and the description of each tool with the Python server (golden file). It also compares the server name, the version and the instructions.
- `TestShippedWorkflowMatchesPython` compares `catalog.SummaryOf` and `catalog.DetailOf` of `it-illustration` with `catalog.summary` and `catalog.detail` of Python, as text, so the key order is checked too.

Stub check:

- An empty `main` builds but answers no MCP call. Then the contract test prints `the server starts and answers over stdio` as FAIL, and the 6 named checks do not run.
- Mutation run: `destructiveHint` of `uninstall_workflow` set to false, and one argument description changed. `TestToolsMatchPython` then fails with `uninstall_workflow: inputSchema differs` and `uninstall_workflow: annotations differs`. After the revert it passes. A tool list without schemas fails the same way.

`python scripts/lint-public.py`: `lint-public: 0 finding(s)`. `gofmt -l .` prints nothing.

Ratchet: contract checks that pass against Go: 0 → 11 (the plan asks for at least 6). `goal`: 0 → 1 of 8 (`builds`).

## G6

Baseline before building: `python tools/goal_check.py --no-ci` on `origin/main` (`6d8553f`) has `FAIL  setup-commands` (no `internal/scripts`), `goal: 1 of 8`. Commands with a passing test: 0 of 8.

Red (before building, in the main checkout at `6d8553f`):

```
$ go test ./internal/scripts/installcomfyui ./internal/scripts/startcomfyui ./internal/scripts/installcomfymcp
stat <repo>\internal\scripts\installcomfyui: directory not found
FAIL	./internal/scripts/installcomfyui [setup failed]
stat <repo>\internal\scripts\startcomfyui: directory not found
FAIL	./internal/scripts/startcomfyui [setup failed]
stat <repo>\internal\scripts\installcomfymcp: directory not found
FAIL	./internal/scripts/installcomfymcp [setup failed]
FAIL
exit 1
```

Green:

```
$ go test -v ./internal/scripts/installcomfyui ./internal/scripts/startcomfyui ./internal/scripts/installcomfymcp
--- PASS: TestWrongSHA256StopsBeforeUnpacking (0.05s)
--- PASS: TestRightSHA256UnpacksWithTheArchivePath (0.53s)
--- PASS: TestKeepArchive (0.04s)
--- PASS: TestTooLittleFreeSpaceStopsWithTheMessageOfTheScript (0.00s)
--- PASS: TestAlreadyInstalledSkips (0.00s)
--- PASS: TestNonEmptyFolderWithoutComfyUIFails (0.00s)
--- PASS: TestSevenZipMissingFails (0.00s)
--- PASS: TestSevenZipFailureKeepsTheArchive (0.04s)
--- PASS: TestWrongLayoutFails (0.04s)
--- PASS: TestVerifiedArchiveIsReusedAndOtherFileIsRefused (0.04s)
--- PASS: TestDownloadResumesAPartFile (0.05s)
--- PASS: TestDownloadRetriesAfterAFailedAttempt (0.04s)
--- PASS: TestDownloadGivesUpAfterThreeAttempts (0.00s)
--- PASS: TestPinnedValues (0.00s)
--- PASS: TestNotSupportedOffWindows (0.00s)
--- PASS: TestBadFlagExits2 (0.00s)
ok  	github.com/hieplam/horus/internal/scripts/installcomfyui	2.627s
--- PASS: TestStartsThePythonOfThePortableBuildAndWaitsForTheServer (1.72s)
--- PASS: TestWritesTheTwoLogFiles (1.63s)
--- PASS: TestAlreadyAnsweringStartsNothing (0.02s)
--- PASS: TestFailsAfterTheTimeLimitWhenNothingAnswers (2.55s)
--- PASS: TestFailsWhenThePythonStops (2.10s)
--- PASS: TestFailsWhenComfyUIIsNotInstalled (0.01s)
--- PASS: TestNotSupportedOffWindows (0.00s)
--- PASS: TestBadFlagExits2 (0.00s)
ok  	github.com/hieplam/horus/internal/scripts/startcomfyui	9.783s
--- PASS: TestMakesTheVenvAndInstallsThePinnedVersions (0.85s)
--- PASS: TestRunAgainChangesNothing (0.70s)
--- PASS: TestPipFailureFails (0.65s)
--- PASS: TestFolderThatIsNoVenvFails (0.00s)
--- PASS: TestMissingLauncherFails (0.00s)
--- PASS: TestVenvPathsFollowTheOS (0.00s)
--- PASS: TestBadFlagExits2 (0.00s)
ok  	github.com/hieplam/horus/internal/scripts/installcomfymcp	2.711s
```

Each test reads a file on disk or the record of a fake program. The tests never start the real ComfyUI, never download the real archive and never run a real `pip install`.

Stub check (mutation runs, each reverted after the run):

- `install-comfyui` with the SHA-256 check switched off: `TestWrongSHA256StopsBeforeUnpacking` fails with `error = <nil>`.
- `install-comfy-mcp` with the `pip install` call replaced by `python -c pass`: `TestMakesTheVenvAndInstallsThePinnedVersions`, `TestRunAgainChangesNothing` and `TestPipFailureFails` fail with `Expected program is missing`.
- A command that does nothing leaves the record of the fake programs empty. The tests of the three commands each read that record or the files that the fake program makes.

`GOOS=linux` and `GOOS=darwin` `go vet ./internal/scripts/... ./cmd/...` pass, so the detach file pair builds on all three systems. `gofmt -l .` prints nothing. `python scripts/lint-public.py`: `lint-public: 0 finding(s)`.

Ratchet: `python tools/goal_check.py --no-ci` gives `FAIL  setup-commands - 3 of 8 commands have a test; none for checksetup, syncworkflows, newworkflow, register, runillustration`. Commands with a passing test: 0 → 3 of 8. `goal`: 1 of 8, unchanged (the row passes after task G7). The real run of the three commands on this PC is task G9.
## G3

Red (before building, in the worktree on `origin/main` at `6d8553f`):

```
$ go test ./internal/scripts/downloadmodel
# ./internal/scripts/downloadmodel
stat <worktree>\internal\scripts\downloadmodel: directory not found
FAIL	./internal/scripts/downloadmodel [setup failed]
FAIL
exit 1
$ python tools/goal_check.py --no-ci
FAIL  contract - server command not found: horus
FAIL  graph - missing: ./internal/comfy
FAIL  workflow-cli - missing: tests/workflow_parity.py
FAIL  downloader - missing: ./internal/scripts/downloadmodel
FAIL  setup-commands - pattern ./internal/scripts/...: GetFileAttributesEx .\internal\scripts\: The system cannot find the file specified.
PASS  builds - 3 of 3 build
FAIL  real-image - needs --real on the GPU machine
FAIL  no-python - 2827 lines left; 18 files of Python and PowerShell remain; .mcp.json horus entry is not `horus mcp`
goal: 1 of 8 rows passed
exit 1
```

Green. Every test runs against a local fake HTTPS server (`httptest.NewTLSServer`, port 0) in temporary folders. A test transport sends every host name to that server, so no test reaches the network. The token is a made-up value.

```
$ go test -count=1 -v ./internal/scripts/downloadmodel   (only the result lines)
--- PASS: TestFullDownloadWithRightSizeAndSHA256 (0.02s)
--- PASS: TestResumeFromPartFile (0.02s)
--- PASS: TestResumeWhenServerIgnoresRange (0.02s)
--- PASS: TestRetryAfterBrokenConnection (0.02s)
--- PASS: TestWrongSizeIsNotPutInPlace (0.02s)
--- PASS: TestWrongSHA256IsNotPutInPlace (0.02s)
--- PASS: TestDifferentFileWithSameNameIsNotOverwritten (0.01s)
--- PASS: TestFreeSpaceUnderSizePlusOneGiBIsRefused (0.01s)
--- PASS: TestFreeSpaceOfThisSystem (0.00s)
--- PASS: TestHTTP401And403GiveTwoOutcomes (0.02s)
--- PASS: TestTokenGoesOnlyToHuggingFaceHosts (0.12s)
--- PASS: TestTokenIsDroppedOnRedirectToAnotherHost (0.02s)
--- PASS: TestTokenIsInNoURLArgumentOrOutput (0.06s)
--- PASS: TestCommandFlagsAndExitCodes (0.00s)
ok  	github.com/hieplam/horus/internal/scripts/downloadmodel	2.137s
```

The cases of the plan and their tests:

| Case | Test | What the test reads |
|---|---|---|
| 1 full download | `TestFullDownloadWithRightSizeAndSHA256` | the SHA-256 of the file on disk, the absent `.part`, the exact output lines; a second run prints `SKIP` and makes no request |
| 2 resume | `TestResumeFromPartFile`, `TestResumeWhenServerIgnoresRange` | the fake server sees `Range: bytes=75000-`; a server that ignores `Range` gives a correct file |
| 3 retry | `TestRetryAfterBrokenConnection` | the server breaks the connection after 123,456 of 400,000 bytes; the second request has `Range: bytes=123456-`; one wait of 5 s |
| 4 wrong size | `TestWrongSizeIsNotPutInPlace` | a short and a long answer: 4 attempts, no destination file |
| 5 wrong SHA-256 | `TestWrongSHA256IsNotPutInPlace` | the message, no destination file, no `.part` file |
| 6 no overwrite | `TestDifferentFileWithSameNameIsNotOverwritten` | other size and same size with another hash: the user's file is unchanged, no request, the text gives `FILE_CONFLICT` in the check of `models.py` |
| 7 free space | `TestFreeSpaceUnderSizePlusOneGiBIsRefused`, `TestFreeSpaceOfThisSystem` | 1 byte under the size plus 1 GiB is refused (`DISK_FULL`), no request, no folder made; exactly the rest of a `.part` plus 1 GiB passes; the real free space of a temporary folder is read |
| 8 HTTP 401 and 403 | `TestHTTP401And403GiveTwoOutcomes` | `KindUnauthorized` and `KindForbidden`; the `Attempt` and `FAIL:` lines give `HF_TOKEN_INVALID` and `LICENSE_NOT_ACCEPTED` in the check of `models.py` |
| 9 token hosts | `TestTokenGoesOnlyToHuggingFaceHosts`, `TestTokenIsDroppedOnRedirectToAnotherHost` | the header that the fake server records: sent to `huggingface.co`, `cdn-lfs.huggingface.co`, `HuggingFace.co`, `huggingface.co:443`; not sent to `example.net`, `evil-huggingface.co`, `huggingface.co.evil.example`, a path or a query with the name; `http` and an address with a user part are refused; a redirect chain `huggingface.co` → `cdn-lfs.huggingface.co` → another host sends it twice, then not |
| 10 token hidden | `TestTokenIsInNoURLArgumentOrOutput` | through the command, with the token from the environment and from `.env`, for 200, 401 and 403: the token is in no request URL, no argument and no output line; `-h` prints no token |

```
$ python tools/goal_check.py --no-ci
FAIL  contract - server command not found: horus
FAIL  graph - missing: ./internal/comfy
FAIL  workflow-cli - missing: tests/workflow_parity.py
PASS  downloader - 14 tests passed
FAIL  setup-commands - 0 of 8 commands have a test; none for installcomfyui, startcomfyui, installcomfymcp, checksetup, syncworkflows, newworkflow, register, runillustration
PASS  builds - 3 of 3 build
FAIL  real-image - needs --real on the GPU machine
FAIL  no-python - 2827 lines left; 18 files of Python and PowerShell remain; .mcp.json horus entry is not `horus mcp`
goal: 2 of 8 rows passed
exit 1
```

Stub check (mutation runs, each reverted after the run):

- A `Download` that returns success and writes nothing: all 14 tests fail, the first is `--- FAIL: TestFullDownloadWithRightSizeAndSHA256`, because the test reads the file and its SHA-256.
- A downloader that always sends the token (no host check, no `CheckRedirect` rule): `--- FAIL: TestTokenGoesOnlyToHuggingFaceHosts`, because the fake server records the headers. The redirect test still passes, because `net/http` drops the header on a redirect to another domain by itself (see Decisions).

`python scripts/lint-public.py`: `lint-public: 0 finding(s)`. `gofmt -l .` prints nothing. `go test ./...` passes.

Ratchet: `goal`: 1 → 2 of 8 rows (`downloader`). Downloader cases that pass: 0 → 10 of 10.

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
| 2026-11-10 | G2 | The golden file of the graph comes from the Python `ui_to_api` through `tools/make_graph_golden.py`. `TestUIToAPIGolden` compares the JSON values, and also the JSON text, so the node order and the input order are checked too. Task G10 deletes the tool together with the Python package. | The plan names the tool and the file. The text compare is stronger than "the two JSON values are equal" and does not weaken the row. |
| 2026-11-10 | G2 | Autostart runs the same binary as `horus start-comfyui -server-url <url>`, with the time limit of `comfy.py` (420 s). The tail is the last 8 lines of stdout, then stderr. A line with "was not found" gives `BACKEND_NOT_INSTALLED`. | The flag is the Go-style name of `-ServerUrl` that task G6 uses. A run of the real binary with an empty ComfyUI folder gives `BACKEND_NOT_INSTALLED` (G2 section). |
| 2026-11-10 | G2 | `internal/comfy/start_windows.go` and `start_unix.go` (`//go:build darwin \|\| linux`) are a second OS file pair. On Windows the child gets `CREATE_NO_WINDOW`, as `creationflags` in `comfy.py`. `WaitDelay` of 10 s stops a child that keeps the pipes open from blocking the call. | The spec allows OS code for starting a process. The pair of task G5 is in `internal/models`, a package that `internal/comfy` does not import. |
| 2026-11-10 | G2 | `internal/server/args.go`: an integer argument outside the `int64` range is kept as a big integer. So a seed from 2^63 to 2^64-1 works, and a seed of 2^64 or more gets `INVALID_ARGUMENT` with `field=seed`, as in Python. `ImageResult.seed` is a `uint64`. | A Python `int` has no size limit. The G1 parser refused these values with a schema error. |
| 2026-11-10 | G2 | A cancelled call (an MCP cancel) returns the error of the context and removes the job from ComfyUI (`/interrupt` or a queue delete), as `CancelledError` does in Python. A cancel during `POST /prompt` leaves the job, as in Python. | Port of the `except asyncio.CancelledError` block. |
| 2026-11-10 | G2 | `TestNotPortedToolsAnswerInternal` no longer calls `generate_image`, because the tool is ported. `tool_generate_image_test.go` tests the tool through MCP, with and without a `progressToken`. | The test checks tools that are not ported. Its premise is false for `generate_image` now. |
| 2026-11-10 | G2 | The wait loop has package variables for the poll interval (2 s), the report interval (10 s) and the absent limit (30 s). The Go tests set them to milliseconds. The first poll reports at once, as in Python, where `last_report` starts at 0. | Tests with a fake ComfyUI must not wait 30 s. |
| 2026-11-10 | G2 | Known difference, not fixed: `out_path` goes through `filepath.Clean`, which also removes `..`. `pathlib.Path` keeps `..`. Both name the same file. | Not forced by Go, but the file is the same and the contract test compares `Path` values. |
| 2026-11-10 | G2 | Known difference, not fixed: the keys of `details` are in sorted order, because `details` is a Go map. Python keeps the insertion order. | Callers read `details` by key. G1 has the same rule for every error. |
| 2026-11-10 | G4 | `tests/workflow_parity.py` has 17 cases: the 10 cases of the plan and 7 more (`check-bad-size`, `check-bad-hash`, `remove-refused`, `check-unlisted-no-node`, `install-no-comfyui`, `remove-part-file`, `list-text`). So Green prints `parity: 17 of 17 cases equal`, not `10 of 10`. | The 10 cases reach only 9 of the 17 item statuses. The 7 more cases reach `BAD_SIZE`, `BAD_HASH`, `REFUSED`, `UNLISTED`, `NO_NODE` and the text output of `list`. More cases prove more of the same row. `DOWNLOADED`, `FAILED` and `DELETE_FAILED` need a download or a locked file; Go tests cover them (see `## G4`). |
| 2026-11-10 | G4 | A parity case compares the exit code, the `HORUS_JSON` result (for `list -json`: the JSON list), every other output line, and after the last step the files of the sandbox's ComfyUI folder (path, size, SHA-256) and the text of `extra_model_paths.yaml`. | The plan asks for the `HORUS_JSON` result. `horus/models.py` also reads the `-- <role> -> <path>` line, and a `remove` that prints the right result but deletes nothing is wrong. |
| 2026-11-10 | G4 | Normalisation of both sides: the sandbox folder becomes `<sandbox>`, every backslash becomes `/`, the port of the fake ComfyUI becomes `<port>`, and the time in a backup name becomes `<time>`. The golden files come from Windows. CI runs `--live` on `windows-latest`, and the default run (Go against the golden files) on `macos-latest`. | The golden files must hold no user path. With `/` for every separator, the Go output on macOS can be equal to the Windows golden files. |
| 2026-11-10 | G4 | Allowed difference (rule 3 of the spec): the table `REWRITE` of the parity tool changes the text of `workflow.ps1` before the comparison: `.\scripts\workflow.ps1` → `horus workflow`, `.\scripts\start-comfyui.ps1` → `horus start-comfyui`, `.\scripts\install-comfyui.ps1` → `horus install-comfyui`, `-ComfyUIDir` → `-comfyui-dir`, `-Force` → `-force`, `-Yes` → `-yes`, `-Only` → `-only`. Nothing else is changed. | A message that named a `.ps1` script names the `horus` subcommand, and flags take Go style. |
| 2026-11-10 | G4 | Safety of the parity tool: each case copies `workflow.ps1` and `download-model.ps1` into the sandbox (so the repo root of the script is the sandbox and it reads no `.env`), sets `HORUS_ROOT` to the sandbox for Go, removes `HF_TOKEN` and every other `HORUS_*` variable from the environment of the child, and sets the five `HORUS_*` folders and the URL as the contract test does. A fake ComfyUI answers `/object_info` with `tests/fixtures/object_info.json`. `install` runs only on a workflow whose files are present, so no case downloads. | The coordinator's safety rules for this PC: no real ComfyUI folder, no real model files, no download from `huggingface.co`, no token. |
| 2026-11-10 | G4 | The flags of `horus workflow`: `-only` (a comma list, or the flag again), `-yes`, `-force`, `-quick`, `-json`, `-workflows-dir`, `-comfyui-dir`, `-models-dir`, `-text-encoder-dir`, `-server-url`, `-models-json`. Flags may come before and after the action and the workflow name, because `horus/models.py` puts them after the name. The action ignores case, as `ValidateSet` did. An unknown action, a third word or a flag that does not parse exits 2 (the script exited 1 on a parameter error). | Rule 3 of the spec, and the exit code 2 for a usage error of every `horus` command (G3, G6). |
| 2026-11-10 | G4 | The default workflows folder is `<repo root>/workflows`. `horus workflow` does not read `HORUS_WORKFLOWS_DIR`, because `workflow.ps1` did not read it. The callers pass `-workflows-dir`, as `horus/models.py` passes `-WorkflowsDir`. | The old code is right. |
| 2026-11-10 | G4 | The detail of a `FAILED` item holds the downloader lines that start with `FAIL:`, `Attempt ` and `The server answered with an error`, joined with ` \| `. The script took `FAIL:`, `Attempt ` and `curl exit 22`; the Go downloader prints `The server answered with an error. HTTP 401 = ... HTTP 403 = ...` where `download-model.ps1` printed `curl exit 22 means ...` (G3). `horus workflow` writes the `FAIL: <message>` line of a failed download, as `download-model.ps1` did. | The G3 note: keep the `Attempt ` and `FAIL:` lines, so that task G5 finds `error: 401`, `error: 403`, `Not enough free space` and `Refusing to overwrite`. `TestDownloadFailuresKeepTheTextThatModelsPyReads` reads each of the four texts. |
| 2026-11-10 | G4 | The sizes in GB of `remove` (`{0:N2}`) follow .NET Framework in the `en-US` culture: the value with 15 significant digits, rounded half away from zero to 2 decimals, a comma between groups of 3 digits. Measured with `powershell.exe` on this PC: `999995000000` bytes gives `1,000.00`, `1005000000` gives `1.01`. `[math]::Round(x, 2)` of `gigabytes` is `x * 100`, rounded to even, `/ 100`. | The culture of this PC and of `windows-latest` is `en-US`. Go's own `%.2f` rounds the binary value and gives `1.00` for `1.005`. A script run in another culture prints other separators; Go always prints the `en-US` form. |
| 2026-11-10 | G4 | `remove -yes` also deletes a read-only file: Go removes the write protection and deletes again. | `Remove-Item -Force` deletes a read-only file; `os.Remove` on Windows does not. |
| 2026-11-10 | G4 | Known difference, not fixed: the text of an operating system error (the detail of `DELETE_FAILED`) and of a JSON error in `models.json` or `workflow.json` is the Go text, not the .NET text. The JSON escapes are also different (Go writes `<` for `<`, the script wrote `'` for `'`); the parsed values are equal. | Go forces its own error texts. No caller reads these texts; `horus/models.py` reads only the status. |
| 2026-11-10 | G4 | The Go comments name `workflow.ps1` or .NET, never the shell by name. | The `no-python` row of `tools/goal_check.py` fails for every Go file that contains the word `powershell`. |

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

## G2

Red (before building, in the worktree on `origin/main` at `6d8553f`):

```
$ go test ./internal/comfy -run TestUIToAPIGolden
# ./internal/comfy
stat <worktree>\internal\comfy: directory not found
FAIL	./internal/comfy [setup failed]
FAIL
exit 1
```

```
$ go build -o bin/horus.exe ./cmd/horus
$ python tests/mcp_contract.py --server-cmd <worktree>/bin/horus.exe mcp
FAIL  generate_image writes a PNG at out_path and returns its path and seed  -- {"ok": false, "result": null, "error": {"code": "INTERNAL", "message": "generate_image is not ported to Go yet: the imag...
FAIL  backend down and autostart off -> BACKEND_UNAVAILABLE, action_by=human  -- {"ok": false, "result": null, "error": {"code": "INTERNAL", "message": "generate_image is not ported to Go yet: the ima...
contract: 11 of 30 passed
exit 1
```

`python tools/goal_check.py --no-ci` showed `FAIL  graph - missing: ./internal/comfy` and `goal: 1 of 8 rows passed`.

Green (after the rebase on `origin/main` with G6 and G3):

```
$ python tools/make_graph_golden.py
wrote tests/fixtures/ui_to_api.golden.json: 10 nodes
```

```
$ python tools/goal_check.py --no-ci
FAIL  contract - server command not found: horus
PASS  graph - TestUIToAPIGolden passes
FAIL  workflow-cli - missing: tests/workflow_parity.py
PASS  downloader - 14 tests passed
FAIL  setup-commands - 3 of 8 commands have a test; none for checksetup, syncworkflows, newworkflow, register, runillustration
PASS  builds - 3 of 3 build
FAIL  real-image - needs --real on the GPU machine
FAIL  no-python - 2827 lines left; 18 files of Python and PowerShell remain; .mcp.json horus entry is not `horus mcp`
goal: 3 of 8 rows passed
exit 1
```

G2 adds the row `graph`. The row `downloader` comes from G3.

```
$ go build -o bin/horus.exe ./cmd/horus
$ python tests/mcp_contract.py --server-cmd <worktree>/bin/horus.exe mcp   (lines cut at 110 characters)
PASS  server identifies as horus and has instructions
PASS  tools/list has exactly the 5 tools
PASS  every tool has an outputSchema with ok, result, error
PASS  uninstall_workflow is marked destructive, list_workflows read-only
FAIL  list_workflows shows installed and partially_installed styles  -- {"ok": false, "result": null, "error": {"co
FAIL  describe_workflow gives subject rules, examples and 3 present files  -- {"ok": false, "result": null, "error
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
FAIL  install of an installed style -> ok, already_installed  -- {"ok": false, "result": null, "error": {"code": "
FAIL  install without accept_license -> LICENSE_CONFIRMATION_REQUIRED, action_by=human, with size  -- {"ok": false
FAIL  uninstall without confirm -> CONFIRMATION_REQUIRED, plan: 1 to delete, 2 shared kept; nothing deleted  -- {"
FAIL  uninstall with confirm deletes the 1 unshared file and keeps the 2 shared files  -- {"ok": false, "result": 
FAIL  uninstall with remove_shared deletes the 2 shared files  -- {"ok": false, "result": null, "error": {"code": 
FAIL  after uninstall, list_workflows shows both styles not_installed  -- {}
PASS  errors are the same envelope in structuredContent and in text
PASS  backend down and autostart off -> BACKEND_UNAVAILABLE, action_by=human
PASS  it-illustration: manifest has no theme and no caption style file
PASS  it-illustration: the graph holds no style words
PASS  docs/mcp.md lists every error code
contract: 22 of 30 passed
exit 1
```

- All 10 checks from "generate_image writes a PNG" to "a job over the time limit" pass.
- "style without model files -> WORKFLOW_NOT_INSTALLED" does not need G5. The missing-model check of `generate_image` finds the file.
- "backend down and autostart off" passes.
- The 8 checks that fail are the install, uninstall and install-state checks of task G5.

```
$ go test -count=1 -v ./internal/comfy ./internal/generate   (only the result lines)
--- PASS: TestJobErrors (0.00s)
--- PASS: TestRejectedGraphError (0.00s)
--- PASS: TestExecutionSeconds (0.00s)
--- PASS: TestClientJobCalls (0.01s)
--- PASS: TestSubmitHTTPErrorWithoutJSON (0.00s)
--- PASS: TestSubmitWithoutPromptID (0.00s)
--- PASS: TestBackendDown (0.00s)
--- PASS: TestEnsureUpAutostartOff (0.00s)
--- PASS: TestEnsureUpWhenUpStartsNothing (0.00s)
--- PASS: TestEnsureUpStartFails (0.52s)
--- PASS: TestEnsureUpStartsTheServer (0.02s)
--- PASS: TestStartCommandIsHorusStartComfyUI (0.00s)
--- PASS: TestUIToAPIGolden (0.00s)
--- PASS: TestUIToAPIRules (0.00s)
--- PASS: TestUIToAPIMissingNodePack (0.00s)
--- PASS: TestMissingModelFiles (0.00s)
--- PASS: TestComboOptions (0.00s)
--- PASS: TestPyDumps (0.00s)
ok  	github.com/hieplam/horus/internal/comfy
--- PASS: TestGenerateWritesThePNG (0.06s)
--- PASS: TestDefaultPathAndSeed (0.04s)
--- PASS: TestArgumentChecks (0.05s)
--- PASS: TestWorkflowNotInstalled (0.04s)
--- PASS: TestNodeCountMismatch (0.03s)
--- PASS: TestOutOfMemory (0.04s)
--- PASS: TestNoImage (0.04s)
--- PASS: TestTimeoutCancelsTheJob (0.35s)
--- PASS: TestJobLeftTheQueue (0.14s)
--- PASS: TestCancelledCallCancelsTheJob (0.03s)
--- PASS: TestBackendDownAutostartOff (0.03s)
--- PASS: TestPySuffixAndQuote (0.00s)
ok  	github.com/hieplam/horus/internal/generate
```

- `TestJobErrors` has one case for each mapped job error:
  - interrupted gives `CANCELLED`;
  - `torch.OutOfMemoryError`, "out of memory", "Out Of Memory" with spaces and "allocation on device" give `OUT_OF_MEMORY`;
  - "CUDA driver version is insufficient" gives `GENERATION_FAILED`, not retryable, with the driver `next_step`;
  - any other failure gives `GENERATION_FAILED`, retryable.
- `TestUIToAPIRules` compares the Go output with the output of the Python `ui_to_api` for the same small graph: a Note node, a seed with `randomize`, a linked widget input, a node without values.
- The tests use a fake ComfyUI on an ephemeral port (`internal/comfy/comfytest`) and temporary folders. They never start the real ComfyUI.
- `TestCancelledCallCancelsTheJob` failed at least 4 times in 30 runs before the fix. The test cancelled the call during `POST /prompt`, before the wait loop. The client also gave `BACKEND_UNAVAILABLE` instead of the context error. Root cause fix: the test cancels after the first poll of `/history`, and a cancelled call returns the context error. After the fix, 100 of 100 runs pass, and `go test -count=10 ./internal/...` passes.

Autostart through the real binary: `HORUS_AUTOSTART=1`, an empty ComfyUI folder and a free port. An MCP client calls `generate_image` of `bin/horus.exe mcp`:

```
{
 "code": "BACKEND_NOT_INSTALLED",
 "action_by": "human",
 "message": "horus could not start ComfyUI at http://127.0.0.1:<port>",
 "start_output_tail": [
  "horus start-comfyui: ComfyUI was not found in <empty dir>. Run horus install-comfyui first, or pass -comfyui-dir."
 ]
}
```

Stub check (mutation runs, each reverted after the run):

- `UIToAPI` that returns an empty graph: `TestUIToAPIGolden` fails with `ui_to_api differs from the golden file` and `got:  {}`.
- `Generate` that returns `ok` without writing the file: `TestGenerateWritesThePNG` fails with `no PNG at out_path: open ...\images\queue.png: The system cannot find the file specified.` `TestGenerateImageTool` (through MCP) fails with `no PNG`. Both tests read the first 8 bytes of the file.

`GOOS=linux` and `GOOS=darwin GOARCH=arm64` `go vet ./...` pass, so the start file pair builds on the three systems. `gofmt -l .` prints nothing. `python scripts/lint-public.py`: `lint-public: 0 finding(s)`.

Ratchet: `goal`: 1 → 2 of 8 on the base of G2 (`graph`). On `main` with G3 it is 3 of 8. Contract checks that pass against Go: 11 → 22. The plan asks for at least 11 + 8 = 19.

## G4

Baseline before building (`origin/main` at `d0f89ef`): `python tools/goal_check.py --no-ci` gives `FAIL  workflow-cli - missing: tests/workflow_parity.py` and `goal: 2 of 8 rows passed`. Parity: 0 cases equal.

The golden files: `python tests/workflow_parity.py --record` on this PC (Windows 11, Windows PowerShell 5.1, culture `en-US`) ran `workflow.ps1` in a sandbox for each case and wrote `tests/fixtures/workflow_parity/<case>.json` (17 files).

Red (the parity tool and the golden files committed, `horus workflow` still the line in `cmd_not_ported.go`):

```
$ horus workflow check it-illustration -json
horus workflow: not implemented yet
exit 2
$ python tests/workflow_parity.py   (the diff lines under each case left out)
DIFFERENT list
DIFFERENT check-quick
DIFFERENT check
DIFFERENT install-installed
DIFFERENT check-absent
DIFFERENT remove-plan
DIFFERENT remove-yes
DIFFERENT remove-yes-force
DIFFERENT list-after-remove
DIFFERENT check-only-vae
DIFFERENT check-bad-size
DIFFERENT check-bad-hash
DIFFERENT remove-refused
DIFFERENT check-unlisted-no-node
DIFFERENT install-no-comfyui
DIFFERENT remove-part-file
DIFFERENT list-text
parity: 0 of 17 cases equal
exit 1
$ python tools/goal_check.py --no-ci
FAIL  contract - server command not found: horus
FAIL  graph - missing: ./internal/comfy
FAIL  workflow-cli - parity: 0 of 17 cases equal
PASS  downloader - 14 tests passed
FAIL  setup-commands - 3 of 8 commands have a test; none for checksetup, syncworkflows, newworkflow, register, runillustration
PASS  builds - 3 of 3 build
FAIL  real-image - needs --real on the GPU machine
FAIL  no-python - 2827 lines left; 18 files of Python and PowerShell remain; .mcp.json horus entry is not `horus mcp`
goal: 2 of 8 rows passed
exit 1
```

Green:

```
$ python tests/workflow_parity.py --live
EQUAL     list
EQUAL     check-quick
EQUAL     check
EQUAL     install-installed
EQUAL     check-absent
EQUAL     remove-plan
EQUAL     remove-yes
EQUAL     remove-yes-force
EQUAL     list-after-remove
EQUAL     check-only-vae
EQUAL     check-bad-size
EQUAL     check-bad-hash
EQUAL     remove-refused
EQUAL     check-unlisted-no-node
EQUAL     install-no-comfyui
EQUAL     remove-part-file
EQUAL     list-text
parity: 17 of 17 cases equal
exit 0
$ python tools/goal_check.py --no-ci
FAIL  contract - server command not found: horus
FAIL  graph - missing: ./internal/comfy
PASS  workflow-cli - parity: 17 of 17 cases equal
PASS  downloader - 14 tests passed
FAIL  setup-commands - 3 of 8 commands have a test; none for checksetup, syncworkflows, newworkflow, register, runillustration
PASS  builds - 3 of 3 build
FAIL  real-image - needs --real on the GPU machine
FAIL  no-python - 2827 lines left; 18 files of Python and PowerShell remain; .mcp.json horus entry is not `horus mcp`
goal: 3 of 8 rows passed
exit 1
```

The cases and the item statuses that they reach:

| Case | Steps | Statuses |
|---|---|---|
| `list` | `list -json` | (the JSON list) |
| `check-quick` | `check it-illustration -quick`, no ComfyUI | `OK`, `SKIP` |
| `check` | `check it-illustration` | `OK` |
| `install-installed` | `install it-illustration` | `PRESENT`, `OK` |
| `check-absent` | `check absent-style` | `MISSING`, `NOT_SEEN` |
| `remove-plan` | `remove it-illustration` | `KEPT_SHARED`, `WOULD_DELETE` |
| `remove-yes` | `remove it-illustration -yes` | `KEPT_SHARED`, `DELETED` |
| `remove-yes-force` | `remove it-illustration -yes -force` | `DELETED` |
| `list-after-remove` | `remove ... -yes -force`, then `list -json` | `DELETED`, (the JSON list) |
| `check-only-vae` | `check it-illustration -only vae` | `OK` |
| `check-bad-size` | `check`, the VAE file has 1 byte more | `BAD_SIZE` |
| `check-bad-hash` | `check`, the text encoder has other bytes | `BAD_HASH` |
| `remove-refused` | `remove -yes -force`, the VAE file has 1 byte more | `REFUSED`, `DELETED` |
| `check-unlisted-no-node` | `check`, the graph loads one more file, the server has no `VAELoader` | `UNLISTED`, `NO_NODE`, `NOT_SEEN` |
| `install-no-comfyui` | `install` without `ComfyUI/main.py` | (the message that gives `BACKEND_NOT_INSTALLED`) |
| `remove-part-file` | `remove absent-style -yes`, with a `.part` file | `ABSENT`, `KEPT_SHARED`, `DELETED` |
| `list-text` | `list` without `-json` | (the text table) |

The three statuses that need a download or a locked file have Go tests against a local fake HTTPS server (no network):

```
$ go test -count=1 -v ./internal/scripts/workflow   (only the result lines)
--- PASS: TestInstallDownloadsMapsTheFolderAndChecks (0.05s)
--- PASS: TestInstallKeepsAFolderThatTheYAMLAlreadyMaps (0.03s)
--- PASS: TestDownloadFailuresKeepTheTextThatModelsPyReads (0.08s)
    --- PASS: TestDownloadFailuresKeepTheTextThatModelsPyReads/401 (0.02s)
    --- PASS: TestDownloadFailuresKeepTheTextThatModelsPyReads/403 (0.02s)
    --- PASS: TestDownloadFailuresKeepTheTextThatModelsPyReads/disk_full (0.02s)
    --- PASS: TestDownloadFailuresKeepTheTextThatModelsPyReads/conflict (0.01s)
--- PASS: TestRemoveThatCannotDeleteGivesDeleteFailed (0.01s)
--- PASS: TestRemoveDeletesAReadOnlyFile (0.01s)
--- PASS: TestFlagsAndExitCodes (0.01s)
--- PASS: TestYAMLFolders (0.01s)
--- PASS: TestNumbersAsTheScriptPrintsThem (0.00s)
ok  	github.com/hieplam/horus/internal/scripts/workflow	1.769s
```

- `TestInstallDownloadsMapsTheFolderAndChecks`: `DOWNLOADED` for both files, the bytes on disk, the `-- vae -> <path>` line, the two `horus_<folder>` blocks in `extra_model_paths.yaml`, the backup `extra_model_paths.yaml.bak-20261110-090807` with the old text, and a second install that gives `PRESENT` with no request and no change to the file.
- `TestDownloadFailuresKeepTheTextThatModelsPyReads`: HTTP 401, HTTP 403, too little free space and a different file on disk each give a `FAILED` item whose detail holds `error: 401`, `error: 403`, `Not enough free space` or `Refusing to overwrite`, with the `Attempt ` and `FAIL:` lines.
- `TestRemoveThatCannotDeleteGivesDeleteFailed`: an open file on Windows, a read-only folder on macOS and Linux.

Stub check (mutation runs, each reverted after the run):

- `writeResult` prints `HORUS_JSON {}`: `parity: 2 of 17 cases equal`, exit 1. The two equal cases are `list` and `list-text`, which have no `HORUS_JSON` line.
- `list` prints `[]`: `list`, `list-after-remove` and `list-text` are `DIFFERENT` (`parity: 0 of 3 cases equal`).
- The detail of a `FAILED` item left empty: the 4 subtests of `TestDownloadFailuresKeepTheTextThatModelsPyReads` fail.
- The tool counts only the cases that it compared, and exits 0 only when that count equals the number of cases. `--live` proves that the golden files still come from `workflow.ps1`.

`python scripts/lint-public.py`: `lint-public: 0 finding(s)`. `gofmt -l .` prints nothing. `go test ./...` passes. `GOOS=linux` and `GOOS=darwin` `go vet ./...` pass.

Ratchet: `goal`: 2 → 3 of 8 rows (`workflow-cli`). Parity: 0 → 17 of 17 cases equal (the plan asks for 10 of 10).

After the merge of `origin/main` with G2 (`7096ebd`), on this PC: `go test ./...` passes, `python tests/workflow_parity.py --live` prints `parity: 17 of 17 cases equal` (exit 0), and `python tools/goal_check.py --no-ci` prints `PASS  graph`, `PASS  workflow-cli`, `PASS  downloader`, `PASS  builds` and `goal: 4 of 8 rows passed`.

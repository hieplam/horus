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
| 2026-11-10 | G8 | The CI `contract` job is strict: it runs the contract test without `|| true` and ends with `grep -q "contract: 30 of 30 passed" contract.log`, the same rule as `python-baseline`. The grep has no `^` or `$` anchor. | The Windows runner can write CR before the line end. `python-baseline` already uses this form on the same system. |
| 2026-11-10 | G3 | Allowed difference: the curl text is gone. The line `Attempt N of M did not finish (curl exit 22, ...)` is now `Attempt N of M did not finish (the server returned error: 401, ...)`, so the text `error: 401` and `error: 403` that `_download_error` in `horus/models.py` reads stays in the item detail. The hint line `curl exit 22 means ...` is now `The server answered with an error. HTTP 401 = ... HTTP 403 = ...`; it still names both codes, so it is still no evidence of either. | Rule 3 of the spec: no `curl.exe`. Task G4 must keep `Attempt ` and `FAIL:` lines in the detail of a `FAILED` item. |
| 2026-11-10 | G3 | The Go API for task G4: `downloadmodel.Download(ctx, Options) (Result, error)`. The error is an `*Error` with a `Kind`: `KindUnauthorized` (401), `KindForbidden` (403), `KindDiskFull`, `KindConflict`, `KindBadHash`, `KindFailed`. `Result.Skipped` is true for the `SKIP` case. `downloadmodel.Main` is the command. | The task asks for a small API that G4 calls. The four outcomes of the spec are values, and the text output stays the same as well. |
| 2026-11-10 | G3 | The retry loop of the script is kept as it was: 4 attempts, a wait of 5 s times the attempt number, also after HTTP 401 and 403. | The old code is right. A retry after 401 does no harm. |
| 2026-11-10 | G3 | The free space comes from a second set of OS files: `freespace_windows.go` (`GetDiskFreeSpaceExW`), `freespace_unix.go` (`darwin \|\| linux`, `syscall.Statfs`) and `freespace_other.go` (no value, so a warning, as the script did for a network path). They use only the standard library, so `go.mod` does not change. The check reads the nearest folder that exists, not the drive root. | Go has no portable call for free space, so Go forces this OS code. The spec names only the process tree as OS code; this is one more place. |
| 2026-11-10 | G3 | Three checks that the script did not have: an address with a user part (`https://user@host/`) is refused, because `net/http` sends the user part as a `Basic` Authorization header; the downloader checks again that the destination is absent just before the rename, because `os.Rename` replaces a file; and it reads at most one byte more than the expected size from an answer. | Rule 6 of the spec (no credential to another host) and "never overwrite a file that it did not write". `Move-Item` refused an existing file; `os.Rename` does not. |
| 2026-11-10 | G3 | The stub check found that Go's `net/http` itself drops the Authorization header on a redirect to a host that is not the first host or a subdomain of it. So the redirect test also passes without the own `CheckRedirect` rule. The own rule stays, because it also drops the header on a redirect to plain `http` and to a host that is not `huggingface.co`. The stub "always send the token" fails `TestTokenGoesOnlyToHuggingFaceHosts`. | Evidence for the Stub check, recorded so that nobody reads the redirect test as proof of the own rule alone. |
| 2026-11-10 | G2 | The golden file of the graph comes from the Python `ui_to_api` through `tools/make_graph_golden.py`. `TestUIToAPIGolden` compares the JSON values, and also the JSON text, so the node order and the input order are checked too. Task G10 deletes the tool together with the Python package. | The plan names the tool and the file. The text compare is stronger than "the two JSON values are equal" and does not weaken the row. |
| 2026-11-10 | G2 | Autostart runs the same binary as `horus start-comfyui -server-url <url>`, with the time limit of `comfy.py` (420 s). The tail is the last 8 lines of stdout, then stderr. A line with "was not found" gives `BACKEND_NOT_INSTALLED`. | The flag is the Go-style name of `-ServerUrl` that task G6 uses. A run of the real binary with an empty ComfyUI folder gives `BACKEND_NOT_INSTALLED` (G2 section). |
| 2026-11-10 | G2 | `internal/comfy/start_windows.go` and `start_unix.go` (`//go:build darwin \|\| linux`) are a second OS file pair. On Windows the child gets `CREATE_NO_WINDOW`, as `creationflags` in `comfy.py`. `WaitDelay` of 10 s stops a child that keeps the pipes open from blocking the call. | The spec allows OS code for starting a process. The pair of task G5 is in `internal/models`, a package that `internal/comfy` does not import. |
| 2026-11-10 | #32 | `horus register` passes `HORUS_ROOT=<repo root>` with `-e`, after the name `horus` and before `--`. The root comes from `settings.Root()`, or from the new flag `-root`. It must be absolute and must have `workflows/`, or `register` fails and adds nothing. | `claude mcp add -e` takes many values, so it stands after the name, as the help example shows. A root without `workflows/` gives a broken entry. The Python server served the repo from every folder, so the old code is right. |
| 2026-11-10 | G2 | `internal/server/args.go`: an integer argument outside the `int64` range is kept as a big integer. So a seed from 2^63 to 2^64-1 works, and a seed of 2^64 or more gets `INVALID_ARGUMENT` with `field=seed`, as in Python. `ImageResult.seed` is a `uint64`. | A Python `int` has no size limit. The G1 parser refused these values with a schema error. |
| 2026-11-10 | G2 | A cancelled call (an MCP cancel) returns the error of the context and removes the job from ComfyUI (`/interrupt` or a queue delete), as `CancelledError` does in Python. A cancel during `POST /prompt` leaves the job, as in Python. | Port of the `except asyncio.CancelledError` block. |
| 2026-11-10 | G2 | `TestNotPortedToolsAnswerInternal` no longer calls `generate_image`, because the tool is ported. `tool_generate_image_test.go` tests the tool through MCP, with and without a `progressToken`. | The test checks tools that are not ported. Its premise is false for `generate_image` now. |
| 2026-11-10 | G2 | The wait loop has package variables for the poll interval (2 s), the report interval (10 s) and the absent limit (30 s). The Go tests set them to milliseconds. The first poll reports at once, as in Python, where `last_report` starts at 0. | Tests with a fake ComfyUI must not wait 30 s. |
| 2026-11-10 | G2 | Known difference, not fixed: `out_path` goes through `filepath.Clean`, which also removes `..`. `pathlib.Path` keeps `..`. Both name the same file. | Not forced by Go, but the file is the same and the contract test compares `Path` values. |
| 2026-11-10 | G2 | Known difference, not fixed: the keys of `details` are in sorted order, because `details` is a Go map. Python keeps the insertion order. | Callers read `details` by key. G1 has the same rule for every error. |
| 2026-11-10 | G7 | `check-setup` calls the model check through an interface `ModelChecker`. The real one runs `<this binary> workflow check <name> -quick -json` as a child process (the process boundary of the spec). Issue #11 (task G4) was not merged when G7 was built, so no model-file rule is copied. The folders go to the child in the variables `HORUS_COMFYUI_DIR`, `HORUS_MODELS_DIR` and `HORUS_TEXT_ENCODER_DIR`. The check drops the `HORUS_JSON` line and prints the other lines with 6 spaces in front, as the script did. The tests use a fake and the test binary as the child. | The coordinator named this design. Task G4 owns the model-file rules. |
| 2026-11-10 | G7 | `check-setup -workflow` checks the size only (`-quick`). The new flag `-full` removes `-quick`, so the SHA-256 is checked too. The script always hashed. | The coordinator named `-quick`. Hashing 11 GB takes minutes, so the fast check is the default and the old behaviour is one flag away. |
| 2026-11-10 | G7 | `check-setup` has 6 checks, and 1 more with `-workflow`: ComfyUI folder, ComfyUI server, comfy-mcp environment (found in the venv or on the PATH), `.mcp.json` comfy-mcp entry, `horus on PATH`, `.mcp.json horus` entry. An entry passes when its command is an existing file, or a name found on the PATH. The script also required that the command equals the venv path; Go does not, because the entry of the Go era is the bare name `horus`. The flag `-horus-venv-path` is gone. The flag `-models-json` is gone, because `horus workflow check` has no such flag. | Plan step 2 and issue #4 (port map). |
| 2026-11-10 | G7 | Flags in Go style: `-comfyui-dir`, `-server-url`, `-venv-path`, `-workflow`, `-full`, `-models-dir`, `-text-encoder-dir` (check-setup); `-comfyui-dir`, `-force`, `-workflows-dir` (sync-workflows); `-name`, `-description`, `-model-family`, `-workflows-dir` (new-workflow); `-horus-path` (register); `-subject`, `-subject-file`, `-out`, `-seed`, `-width`, `-height`, `-url`, `-workflow`, `-style-file`, `-timeout` (run-illustration). A flag error exits 2. A failure prints `horus <command>: <message>` to stderr and exits 1, except `run-illustration`, which prints `{"ok":false,"error":...}` to stderr as the script did. | Rule 3 of the spec and the decision of G6. |
| 2026-11-10 | G7 | `run-illustration` calls `generate.Generate`. `-workflow` takes a workflow name (default `it-illustration`) and, since issue #27, also the path of a `workflow.json`. `-steps` exists again (issue #27). The JSON line has `ok`, `prompt_id` (the job id), `out`, `seed`, `steps`, `width`, `height`, `execution_seconds` and `comfy_output` (issue #27 put `steps` and `comfy_output` back). The width and the height default to the manifest (1536 x 1024 for `it-illustration`), and the rules of the manifest apply. An existing `-out` file is replaced, as in the script. The call can start ComfyUI when it does not answer (the autostart of `internal/comfy`; `HORUS_AUTOSTART=0` turns it off). | The coordinator said that `run-illustration` must use `internal/generate` and not copy its logic. The plan asks for "the path and the seed". |
| 2026-11-10 | #27 | `run-illustration` has `-steps`, a `-workflow` path and the keys `steps` and `comfy_output` again. The hook in `internal/generate` is the optional field `Request.Steps` (a pointer; `generate_image` never sets it). `Result` has two fields with the json tag `-` (`Steps`, `ComfyOutput`), so the tool result does not change. The step count is read from the one `KSampler` node; a workflow without one reports 0, and `-steps` on it is an error. A `-workflow` value with a path separator or the suffix `.json` must end in `workflow.json`. Its folder name is the workflow name, and the parent folder becomes `HORUS_WORKFLOWS_DIR` for this process. | The old script is right (standing rule of the epic), and the spec does not allow the three differences. |
| 2026-11-10 | G7 | `register` runs `claude mcp get horus` in the temporary folder first, as the script did, and skips the add with `SKIP:` when Claude Code knows the server. Then it runs `claude mcp add --scope user horus -- <path of this binary> mcp`. The flag `-horus-path` overrides the path. In `cmd/horus` the package is imported as `registercmd`, because the function `register` of `main.go` has the same name. | Port of the `-Register` part, and the plan, step 3. |
| 2026-11-10 | G7 | `new-workflow` and `sync-workflows` read the workflows folder from `-workflows-dir`, then `HORUS_WORKFLOWS_DIR`, then `<repo root>/workflows`. A test writes only into a temporary copy. The kebab-case rule is the regular expression of the script. Known difference: Go `$` does not match before a trailing newline, so a name with a trailing newline is refused now; PowerShell accepted it. | The tests must not write into the real folder. |
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
| 2026-11-10 | G4 | The disk record of a parity case counts CRLF as LF before it takes the size and the SHA-256 of a file. | The first CI run on `macos-latest` gave `parity: 1 of 17 cases equal`. Every difference was one line, `comfy/ComfyUI/main.py 8 ddc00976caf4` (golden) against `comfy/ComfyUI/main.py 7 d13e881e48bd` (macOS): `make_sandbox` of the contract test writes `main.py` in text mode, so Windows writes CRLF. horus does not write that file. The model stand-ins and `extra_model_paths.yaml` have no CRLF. |
| 2026-11-10 | G5 | The ratchet of G5 is 22 → 30, so +8 and not +9. The baseline on `origin/main` (`3944492`) was 22, not 21: "style without model files -> WORKFLOW_NOT_INSTALLED naming the file" passes since G2, because `generate_image` finds the missing file in `/object_info`. 30 of 30 is the maximum. | The plan counted that check as a G5 check. The row `contract` asks for 30 of 30, and G5 reaches it. |
| 2026-11-10 | G5 | Allowed difference (rule 3 of the spec): the `INTERNAL` messages of `internal/models` name the subcommand: `horus workflow <action> printed no result` and `horus workflow list failed`, where `models.py` wrote `workflow.ps1 ...`. | A message that named a `.ps1` script names the `horus` subcommand. |
| 2026-11-10 | G5 | Only `install_workflow` stops on a cancel, as in `models.py`. The end of its context kills the process tree of the child. `ListStatus`, `Check`, `Remove` and the file states of `describe_workflow` run with `context.WithoutCancel`, so they run to the end, as the threads of `models.py` did. | The old code is right. A `remove -yes` that stops in the middle deletes only some files. |
| 2026-11-10 | G5 | The process tree: on Windows `taskkill /PID <pid> /T /F` (from `%SystemRoot%\System32`), then `Process.Kill`, as `Run.cancel` of `models.py` does; the child gets `CREATE_NO_WINDOW`. On macOS and Linux the child gets its own process group (`Setpgid`), and the cancel sends `SIGKILL` to the group. Not a Job Object. | The plan allows `taskkill /T`. It is the method of the old code, and it needs no Windows API calls. |
| 2026-11-10 | G5 | `TestNotPortedToolsAnswerInternal` is deleted, and the helper `notPorted` with it. No tool part is left that is not ported, so the premise of the test is false. `TestModelToolsOnASandbox` calls the 4 tools through MCP with a real child and checks that `describe_workflow` names no graph file and no node class. | The same reason as the G2 decision about this test. The test is not weakened to pass: its subject is gone. |
| 2026-11-10 | G5 | The Go tests of `internal/models` and `internal/server` use the test binary as the `horus workflow` child (`models.Exe` is the seam, `TestMain` the fake). In `internal/server` the fake runs the real `workflow.Main` in the child process, except in the `sleep` mode of the cancel test. | The process boundary of the spec: the server calls the workflow code only through a child process. |
| 2026-11-10 | G9 | `goal_check.py --real` imports the MCP client (`mcp==2.3.0`) only inside the `real-image` row. The other 7 rows stay standard library. The new flag `--real-out <folder>` names the folder of the PNG (default `.scratch/real-image`); the file name is `real-image-seed7-<time>.png`, so a second run never needs `overwrite`. The server gets the environment of the check, so it reads `HORUS_ROOT` and the `.env` of that root, and it may start ComfyUI. The client waits at most 3600 s for the call. | The plan names the subject, the seed and the size, not the folder or the time limit. The server's own limit is `HORUS_GENERATE_TIMEOUT` (1800 s) plus the start of ComfyUI (420 s). |
| 2026-11-10 | G9 | The checks ran the binary of the worktree (`bin/horus.exe`) with `HORUS_ROOT` set to the main checkout, and `go install ./cmd/horus` put a second copy in `%USERPROFILE%\go\bin`. That folder is not on the owner's PATH, so the first `horus check-setup` showed `FAIL  horus on PATH`. With the folder on the PATH of the shell, all 6 checks pass. The PATH of the owner was not changed. | The safety rules of this task: no change to the owner's configuration. Task G10 switches `.mcp.json` and decides how `horus` gets on the PATH. |
| 2026-11-10 | G9 | `check-setup` passed `.mcp.json horus`, because the entry still names the Python server and that file exists. The task brief expected a FAIL there; there is none, and `.mcp.json` is not changed in G9. | G7 decision: an entry passes when its command is an existing file. |
| 2026-11-10 | G9 | `horus sync-workflows` (no `-force`) reported `CONFLICT horus-it-illustration.json` and exit 1. The SHA-256 of the pane copy was the same before and after, so the run changed nothing. The pane copy was not forced. | The safety rules of this task. A conflict is the documented answer of the command, not a defect. |
| 2026-11-10 | G9 | Known fact, not a defect of the port: when the MCP client of `goal_check.py --real` ended, the ComfyUI that horus had started also stopped. The Python MCP client (`mcp` 2.3.0, `mcp/os/win32/utilities.py`) puts the server in a Windows Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` and without the breakaway flag, so every child of the server ends with the client. `scripts/start-comfyui.ps1` (`Start-Process`) is in the same Job Object, so the Python server behaves the same. After the `claude -p` run, the ComfyUI that horus started kept running. | Old code is right (standing rule). The `claude -p` run proves that ComfyUI outlives the server for Claude Code, the caller that the product serves. |
| 2026-11-10 | G9 | The `claude -p` run set `CLAUDE_CODE_MCP_AUTO_BACKGROUND_MS=1800000` for its process, so the call of about 4 minutes stayed in the foreground. | `docs/mcp.md` ("Long calls"): in headless mode a call that moves to the background loses its result when the run ends. |
| 2026-11-10 | G9 | The owner's ComfyUI ran with `--reserve-vram 1`; `horus start-comfyui` starts it without that flag, as `start-comfyui.ps1` did. Both images ran on the ComfyUI that horus started. At the end, the task stopped that ComfyUI and started it again with the owner's command line. | The task brief: ComfyUI must run again as it was found. |
| 2026-11-10 | G10 | `tests/mcp_contract.py` has the two changes of rule 4 of the spec and nothing else: the default `--server-cmd` is `horus mcp` (the name `horus`, found on the PATH), and the list of error codes is the JSON object of `<server program> errors -json` (`server_cmd[0]`). When the program does not run or prints no code, the list holds one placeholder code, so the check "docs/mcp.md lists every error code" fails and never passes on an empty list. The docstring lines that named the venv program say this now. The line `sys.path.insert(0, str(ROOT))` stays, although no `horus` package is left to import. | The coordinator allowed exactly these two changes. An empty list would make the doc check pass with nothing checked (stub check). |
| 2026-11-10 | G10 | `tests/workflow_parity.py` loses `--record`, `--live`, the table `REWRITE`, `PS_FLAGS` and the branch that ran the script. The golden files already hold the rewritten text (the rewrite ran when they were recorded), so the default mode compares the same values as before: `parity: 17 of 17 cases equal`. `make_sandbox` still comes from `tests/mcp_contract.py`. CI runs one parity command on both systems. | The two modes need the deleted `workflow.ps1` (G4 note). |
| 2026-11-10 | G10 | `tools/make_server_golden.py` and `tools/make_graph_golden.py` are deleted with the Python package. The Go tests read the committed golden files only; `go test -count=1 ./...` passes after the deletion (18 packages `ok`). `internal/herr/testdata/errors.golden.json` keeps the text `scripts/start-comfyui.ps1`, because it is the output of the old `horus/errors.py`; `TestCodesMatchPython` applies the allowed replacement of G1. | The G1 and G2 decisions: the golden files stay, the tools go. |
| 2026-11-10 | G10 | `pyproject.toml` is deleted. Only the `python-baseline` job used it (`pip install -e .`). The other CI jobs and the docs install `tests/requirements.txt` (`mcp==2.3.0`). `.gitignore` does not change: `__pycache__/` and `*.pyc` still match the files of the test tools, and `*.egg-info/` stays. | The coordinator's list. |
| 2026-11-10 | G10 | The `horus` entry of `.mcp.json` is `{"type": "stdio", "command": "horus", "args": ["mcp"]}`, without `env`. The `comfy-mcp` entry does not change. | Plan step 1: no Python environment variables. The comfy-mcp entry is Phase 3 of issue #4. |
| 2026-11-10 | G10 | `docs/setup.md` gets a step 0 "get the horus program": install Go, `go install ./cmd/horus`, and the Go bin folder on the PATH (with the PowerShell command for the user Path on Windows, and the profile line on macOS and Linux). It names `install-comfyui` and `start-comfyui` as Windows-only, and says that on macOS and Linux the user installs and starts ComfyUI and adds the comfy-mcp server by hand. Step 4 runs `horus check-setup -workflow it-illustration -full`, so the smoke test still hashes the files as the script did (G7 made size-only the default). Every command line of the docs was taken from `horus <command> -h` of the built binary. | The coordinator's points 1 and 2. `-full` keeps the old behaviour of the smoke test. |
| 2026-11-10 | G10 | Found issue #32: `horus register` adds the user-scope entry without `HORUS_ROOT`. A session outside the repo starts `horus mcp` in its own folder, the Go root rule finds no `workflows/`, and `list_workflows` answers `INTERNAL: horus workflow list failed` (measured). The Python entry served the repo from every folder, because `horus/settings.py` takes the folder of the package. #32 blocks G11 and G12. Until the fix, `docs/setup.md` and `docs/mcp.md` name the limit and give the command `claude mcp add --scope user horus --env HORUS_ROOT=<root folder of this repo> -- <path of horus> mcp`. | Standing rule: the old code is right. The fix is in `internal/scripts/register`, outside this task. |
| 2026-11-10 | G10 | On the owner's PC (after the merge), the user-scope entry `horus` is replaced with the command of the row above (with `HORUS_ROOT` set to the main checkout), not with plain `horus register`. | The coordinator asked for `horus register`. Plain `register` gives an entry that answers `INTERNAL` in every folder outside the repo (issue #32), and the old entry worked there. The old code is right. |
| 2026-11-10 | G10 | The owner's PC, outside the repo (coordinator's decisions): `%USERPROFILE%\go\bin` is appended to the user Path when it is absent (no other entry changes), and `go install ./cmd/horus` runs from the merged `main`. The two venvs under `%USERPROFILE%\.venvs` stay: `comfy-mcp` is in use, and `horus-mcp` holds the Python with `mcp==2.3.0` that the test tools use on this PC. `.env` is not touched. | `.mcp.json` starts `horus` by name, so without the folder on the Path the owner's horus server stops. |
| 2026-11-10 | G10 | `tools/goal_check.py` prints `PASS  no-python - 0 lines left`; the plan wrote `0 lines`. The tool is not changed. | The text of the committed tool. The row passes. |
| 2026-11-10 | G10 | The Go files keep their first comment that names the `.ps1` or `.py` file that they port. | Rule 2 of the spec. The `no-python` row looks for the word `powershell` in Go files, not for `.ps1`. |
| 2026-11-10 | Issue 34 | `tests/mcp_contract.py` resolves the server program one time, at the start of `run()`: a program with a path separator that is a file becomes an absolute path; a bare name is looked up on the PATH (`shutil.which`); a program that is not found stays as written, so the run fails. The session and the `errors -json` read use the same resolved program. Nothing else in the test changes: no check, no count. | The session started the program with a relative path (the MCP client resolves it against the server process), but `subprocess.run` of the error-code read did not on Windows (`WinError 2`). One resolved program for both removes the difference. The fix stays inside the place that the spec lets the contract test change. |
| 2026-11-10 | G11 | The amber-gold colour of the envelopes (the subject says sun-yellow) is accepted. | The objects, the count and the positions match. Amber is the model's reading of yellow, and no port code chooses it. The model's honest output wins. |

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

## G7

Baseline before building: `python tools/goal_check.py --no-ci` on `origin/main` (`7096ebd`) has `FAIL  setup-commands - 3 of 8 commands have a test; none for checksetup, syncworkflows, newworkflow, register, runillustration`, `goal: 3 of 8`. Commands with a passing test: 3 of 8.

Red (before building, in the main checkout at `7096ebd`):

```
$ go test ./internal/scripts/checksetup ./internal/scripts/syncworkflows ./internal/scripts/newworkflow ./internal/scripts/register ./internal/scripts/runillustration
stat <repo>\internal\scripts\syncworkflows: directory not found
FAIL	./internal/scripts/syncworkflows [setup failed]
stat <repo>\internal\scripts\newworkflow: directory not found
FAIL	./internal/scripts/newworkflow [setup failed]
stat <repo>\internal\scripts\register: directory not found
FAIL	./internal/scripts/register [setup failed]
stat <repo>\internal\scripts\runillustration: directory not found
FAIL	./internal/scripts/runillustration [setup failed]
FAIL
```

Green:

```
$ go test -count=1 ./internal/scripts/checksetup ./internal/scripts/syncworkflows ./internal/scripts/newworkflow ./internal/scripts/register ./internal/scripts/runillustration
ok  	github.com/hieplam/horus/internal/scripts/checksetup	2.592s
ok  	github.com/hieplam/horus/internal/scripts/syncworkflows	0.686s
ok  	github.com/hieplam/horus/internal/scripts/newworkflow	0.553s
ok  	github.com/hieplam/horus/internal/scripts/register	3.168s
ok  	github.com/hieplam/horus/internal/scripts/runillustration	2.068s
$ python tools/goal_check.py --no-ci
PASS  setup-commands - 8 of 8 commands have a passing test
goal: 4 of 8 rows passed
```

What the tests prove (each one reads a file on disk, the record of a fake program, or the output of the command):
- `check-setup`: with a ComfyUI folder, a fake ComfyUI server, a comfy-mcp program, a `.mcp.json` and a `horus` on the PATH, 6 `PASS` lines and `All checks passed.`. With one piece removed (the ComfyUI folder, the server, `horus` on the PATH, a `.mcp.json` entry), exactly the matching `FAIL` line and `N check(s) failed.`. The model check is tested with a fake, and the real `ExecModels` with the test binary as the child: the record shows the arguments `workflow check it-illustration -quick -json` and the folder variables.
- `sync-workflows`: the file `horus-<name>.json` in the pane folder has the bytes of the repo copy. A pane copy that differs gives `CONFLICT` and stays as it was. With `-force` it is replaced and the `.bak` holds the old bytes.
- `new-workflow`: the README on disk has the three placeholders filled. 10 names that are not kebab-case are refused and leave the folder unchanged. An existing folder is never overwritten.
- `register`: the fake `claude` records `mcp get horus` in the ask folder, then `mcp add --scope user horus -- /the/path/horus mcp`. A known server gives `SKIP:` and no add. A failed add gives an error and no `OK:` line.
- `run-illustration`: against the fake ComfyUI it writes a PNG (first 8 bytes read), prints one JSON line with `out`, `seed`, `prompt_id`, and the caption of the job is the subject as written. A failed job gives one JSON line on stderr, exit 1 and no file.

Stub check (run, then restored):
- `Sync` that does not write the pane copy: `TestSyncCopiesEachWorkflowIntoThePane`, `TestSyncReportsAConflictAndLeavesTheCopy` and `TestSyncForceReplacesAndKeepsABackup` fail.
- `Register` that runs `claude mcp list` instead of `mcp add`: `TestRegisterAddsTheServerInUserScope` and `TestRunFindsClaudeOnThePath` fail.
- `Create` that writes an empty README: `TestCreateFillsTheTemplate` and `TestRun` fail.

`go vet ./...` and `go test ./...` pass, including `TestEveryPortMapRowHasACommand` (12 commands). `GOOS=linux` and `GOOS=darwin GOARCH=arm64` builds pass. `gofmt -l .` prints nothing. `python scripts/lint-public.py`: `lint-public: 0 finding(s)`. No test starts the real `claude`, the real ComfyUI or a real image: they use temporary folders, ephemeral ports and the test binary as the fake program.

Ratchet: commands with a passing test: 3 → 8 of 8. `goal`: 3 → 4 of 8 (`setup-commands`; `graph` was already passing).

## Issue 27

Baseline before building: options of the old script that the Go command lacks: 3 (`--steps`, the workflow path, the keys `steps` and `comfy_output`).

Red (the three tests written, the source files as on `origin/main`):

```
$ go test ./internal/scripts/runillustration -run 'TestSteps|TestWorkflowPath|TestJSONKeys'
flag provided but not defined: -steps
--- FAIL: TestWorkflowPath (0.02s)
    runillustration_test.go:217: exit 1: {"error":"INVALID_ARGUMENT: invalid workflow name '...\elsewhere\it-illustration\workflow.json'","ok":false}
--- FAIL: TestJSONKeys (0.03s)
    runillustration_test.go:236: the key "steps" is missing: map[execution_seconds:3.5 height:1024 ok:true out:... prompt_id:job-1 seed:424242 width:1536]
    runillustration_test.go:236: the key "comfy_output" is missing: ...
FAIL	github.com/hieplam/horus/internal/scripts/runillustration
```

Green:

```
$ go test -count=1 -v ./internal/scripts/runillustration -run 'TestSteps|TestWorkflowPath|TestJSONKeys'
--- PASS: TestSteps (0.06s)
--- PASS: TestWorkflowPath (0.04s)
--- PASS: TestJSONKeys (0.03s)
ok  	github.com/hieplam/horus/internal/scripts/runillustration
$ horus.exe mcp contract (tests/mcp_contract.py --server-cmd <horus.exe> mcp)
contract: 22 of 30 passed
$ python tools/goal_check.py --no-ci
PASS  setup-commands - 8 of 8 commands have a passing test
```

`go vet ./...` and `go test ./...` pass, including the schema golden test of `generate_image`. `gofmt -l .` prints nothing. `python scripts/lint-public.py`: `lint-public: 0 finding(s)`.

Stub check: `TestSteps` reads the `KSampler` node of the graph that the fake ComfyUI received. A flag that is parsed and not used leaves the workflow's own value in the graph, so `TestSteps` fails.

Ratchet: options of the old script that the Go command lacks: 3 to 0. Contract checks that pass against Go: 22 to 22.

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

CI on `macos-latest`, the default run (Go against the golden files): the first run gave `parity: 1 of 17 cases equal`, with the `main.py` line as the only difference (see Decisions). After the change of the disk record and a new `--record`, on this PC: `python tests/workflow_parity.py` and `python tests/workflow_parity.py --live` both print `parity: 17 of 17 cases equal`.

After the merge of `origin/main` with G7 (`9a90f16`): `cmd/horus/cmd_not_ported.go` keeps no line (G7 and G4 each deleted theirs). On this PC: `go test ./...` passes, the default run and `--live` both print `parity: 17 of 17 cases equal`, and `python tools/goal_check.py --no-ci` prints `PASS` for `graph`, `workflow-cli`, `downloader`, `setup-commands` and `builds`, and `goal: 5 of 8 rows passed`.

## G5

Baseline before building (`origin/main` at `3944492`): the contract test against Go prints `contract: 22 of 30 passed`. `python tools/goal_check.py --no-ci` prints `goal: 5 of 8 rows passed`.

Red (the 4 tools still return the `INTERNAL` envelope of G1; the test cuts the long details after 300 characters):

```
$ go build -o bin/horus.exe ./cmd/horus
$ python tests/mcp_contract.py --server-cmd <worktree>/bin/horus.exe mcp
PASS  server identifies as horus and has instructions
PASS  tools/list has exactly the 5 tools
PASS  every tool has an outputSchema with ok, result, error
PASS  uninstall_workflow is marked destructive, list_workflows read-only
FAIL  list_workflows shows installed and partially_installed styles  -- {"ok": false, "result": null, "error": {"code": "INTERNAL", "message": "list_workflows is not ported to Go yet: the install state of each workflow comes in task G5", "retryable": false, "action_by": "human", "next_step": "This is a bug in horus. Report details to the user.", "details": {"task": "G5", "tool": "list_workflows"}}}
FAIL  describe_workflow gives subject rules, examples and 3 present files  -- {"ok": false, "result": null, "error": {"code": "INTERNAL", "message": "describe_workflow is not ported to Go yet: the state of each model file comes in task G5", "retryable": false, "action_by": "human", "next_step": "This is a bug in horus. Report details to the user.", "details": {"task": "G5", "tool": "describe_workflow"}}}
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
FAIL  install of an installed style -> ok, already_installed  -- {"ok": false, "result": null, "error": {"code": "INTERNAL", "message": "install_workflow is not ported to Go yet: the install of the model files comes in task G5", "retryable": false, "action_by": "human", "next_step": "This is a bug in horus. Report details to the user.", "details": {"task": "G5", 
FAIL  install without accept_license -> LICENSE_CONFIRMATION_REQUIRED, action_by=human, with size  -- {"ok": false, "result": null, "error": {"code": "INTERNAL", "message": "install_workflow is not ported to Go yet: the install of the model files comes in task G5", "retryable": false, "action_by": "human", "next_step": "This is a bug in horus. Report details to the user.", "details": {"task": "G5", 
FAIL  uninstall without confirm -> CONFIRMATION_REQUIRED, plan: 1 to delete, 2 shared kept; nothing deleted  -- {"ok": false, "result": null, "error": {"code": "INTERNAL", "message": "uninstall_workflow is not ported to Go yet: the removal of the model files comes in task G5", "retryable": false, "action_by": "human", "next_step": "This is a bug in horus. Report details to the user.", "details": {"task": "G5", "tool": "uninstall_workflow"}}}
FAIL  uninstall with confirm deletes the 1 unshared file and keeps the 2 shared files  -- {"ok": false, "result": null, "error": {"code": "INTERNAL", "message": "uninstall_workflow is not ported to Go yet: the removal of the model files comes in task G5", "retryable": false, "action_by": "human", "next_step": "This is a bug in horus. Report details to the user.", "details": {"task": "G5", "tool": "uninstall_workflow"}}}
FAIL  uninstall with remove_shared deletes the 2 shared files  -- {"ok": false, "result": null, "error": {"code": "INTERNAL", "message": "uninstall_workflow is not ported to Go yet: the removal of the model files comes in task G5", "retryable": false, "action_by": "human", "next_step": "This is a bug in horus. Report details to the user.", "details": {"task": "G5", "tool": "uninstall_workflow"}}}
FAIL  after uninstall, list_workflows shows both styles not_installed  -- {}
PASS  errors are the same envelope in structuredContent and in text
PASS  backend down and autostart off -> BACKEND_UNAVAILABLE, action_by=human
PASS  it-illustration: manifest has no theme and no caption style file
PASS  it-illustration: the graph holds no style words
PASS  docs/mcp.md lists every error code
contract: 22 of 30 passed
exit 1
```

Green:

```
$ go build -o bin/horus.exe ./cmd/horus
$ python tests/mcp_contract.py --server-cmd <worktree>/bin/horus.exe mcp
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
PASS  uninstall without confirm -> CONFIRMATION_REQUIRED, plan: 1 to delete, 2 shared kept; nothing deleted
PASS  uninstall with confirm deletes the 1 unshared file and keeps the 2 shared files
PASS  uninstall with remove_shared deletes the 2 shared files
PASS  after uninstall, list_workflows shows both styles not_installed
PASS  errors are the same envelope in structuredContent and in text
PASS  backend down and autostart off -> BACKEND_UNAVAILABLE, action_by=human
PASS  it-illustration: manifest has no theme and no caption style file
PASS  it-illustration: the graph holds no style words
PASS  docs/mcp.md lists every error code
contract: 30 of 30 passed
exit 0
```

```
$ go test -count=1 -v ./internal/models   (only the result lines)
--- PASS: TestRunScriptReadsTheLinesAndTheResult (0.25s)
--- PASS: TestNoResultLineIsInternal (0.04s)
--- PASS: TestListStatusAndInstallState (0.08s)
--- PASS: TestInstallMapsEachStatusToItsCode (0.48s)
--- PASS: TestRemoveMapsEachStatusToItsCode (0.17s)
--- PASS: TestCancelKillsTheProcessTree (0.21s)
--- PASS: TestPartSize (0.00s)
ok  	github.com/hieplam/horus/internal/models	2.058s
$ go test -count=1 -v ./internal/server -run "TestModelTools|TestInstallReports|TestSizeText"   (only the result lines)
--- PASS: TestModelToolsOnASandbox (0.36s)
--- PASS: TestSizeText (0.00s)
--- PASS: TestInstallReportsProgressAndACancelStopsTheChild (0.28s)
ok  	github.com/hieplam/horus/internal/server	1.213s
```

- `TestInstallMapsEachStatusToItsCode` has 15 subtests, one for each outcome of an install:
  - `ok`;
  - HTTP 401 in the detail and on stderr: `HF_TOKEN_INVALID`;
  - HTTP 403: `LICENSE_NOT_ACCEPTED`, with the license and its page;
  - not enough free space: `DISK_FULL`;
  - refusal to overwrite: `FILE_CONFLICT`;
  - another download failure: `DOWNLOAD_FAILED`;
  - `NO_NODE`: `NODE_PACK_MISSING`;
  - `BAD_SIZE` and `BAD_HASH`: `FILE_CONFLICT`;
  - `NOT_SEEN`: `BACKEND_RESTART_REQUIRED`;
  - `MISSING`: `WORKFLOW_NOT_INSTALLED`;
  - `UNLISTED`: `INTERNAL`;
  - "ComfyUI was not found": `BACKEND_NOT_INSTALLED`;
  - any other failure: `INTERNAL`.
- `TestRemoveMapsEachStatusToItsCode`: `DELETE_FAILED` gives `FILE_IN_USE`, `REFUSED` gives `FILE_CONFLICT`, any other failure gives `INTERNAL`.
- `TestCancelKillsTheProcessTree`: the child sleeps 60 s. A grandchild that the child started holds the same stdout pipe. `WaitDelay` is 30 s in the test, so the run returns before 10 s only when every process of the tree is gone. On this PC the run returned 154 ms after the cancel. The `.part` file stays.
- `TestInstallReportsProgressAndACancelStopsTheChild`: through MCP, with a `progressToken`. The tool reports `2.5e+06/172 downloading flux3-image-klein-int8-convrot.safetensors: 0.00 GB` (the size of the `.part` file, then the total of `models.json`). Then the client cancels the call. The tool returns before 10 s, and the `.part` file stays.

Same answers as the Python server. A scratch script (not committed) made two contract sandboxes. It called the 4 tools 11 times on each server (`horus-mcp` of Python, and `bin/horus.exe mcp`), in this order:

1. `list_workflows`
2. `describe_workflow absent-style`
3. `install_workflow it-illustration` (already installed)
4. `install_workflow absent-style` (no `accept_license`)
5. `uninstall_workflow absent-style`
6. `uninstall_workflow it-illustration` (the plan)
7. `uninstall_workflow it-illustration` with `confirm`
8. `describe_workflow it-illustration`
9. `uninstall_workflow it-illustration` with `confirm` and `remove_shared`
10. `uninstall_workflow it-illustration` with `confirm` (nothing left)
11. `list_workflows`

It compared the envelopes as JSON, with the sandbox folder and `seconds` made equal: `compare: 11 of 11 calls equal`.

Stub check (mutation runs, each reverted after the run):

- `InstallState` returns `installed` for every workflow: `FAIL  list_workflows shows installed and partially_installed styles`, `FAIL  after uninstall, list_workflows shows both styles not_installed  -- {"absent-style": "installed", "it-illustration": "installed"}`, `contract: 28 of 30 passed`.
- `Remove` never passes `-yes`, but reports `WOULD_DELETE` as `DELETED` (an uninstall that deletes nothing): `FAIL  uninstall with confirm deletes the 1 unshared file and keeps the 2 shared files` (the states are right, but the files are still on disk), `FAIL  uninstall with remove_shared deletes the 2 shared files`, `FAIL  after uninstall, list_workflows shows both styles not_installed  -- {"absent-style": "partially_installed", "it-illustration": "installed"}`, `contract: 27 of 30 passed`.
- `killTree` on Windows kills only the child (no `taskkill /T`): `--- FAIL: TestCancelKillsTheProcessTree (21.77s)` with `the run did not return 20 s after the cancel`.

G1 said that two checks passed for a weak reason. They now pass with real results:

- "list_workflows and describe_workflow carry no theme or look" reads a real `describe_workflow` result and two real `list_workflows` rows.
- "describe_workflow hides the ComfyUI graph (no node class names)" reads the text of a real `describe_workflow` result.

`python scripts/lint-public.py`: `lint-public: 0 finding(s)`. `gofmt -l .` prints nothing. `go test ./...` passes. `GOOS=linux` and `GOOS=darwin` `go vet ./...` pass.

Ratchet: contract checks against Go: 22 → 30 of 30 (+8; see Decisions). `goal`: 5 → 5 of 8 rows. The row `contract` also needs the CI jobs on both systems (task G8).

## G8

Baseline: the contract test against Go prints `contract: 30 of 30 passed` on this PC. The `contract` job of `ci.yml` is not strict: it ends with `|| true` and a `grep "contract: "` that passes on any count.

Red not applicable: `G2` and `G5` already reached 30. The logs of the newest run on `main` (run 42799283672, commit `4982496`, merge of PR 29), both jobs green:

```
$ gh run view 42799283672 --job 129772350576 --log | grep "contract: "     (contract windows-latest)
contract windows-latest  Contract test against the Go binary  2026-11-10T10:28:27.7871992Z contract: 30 of 30 passed
$ gh run view 42799283672 --job 129772350640 --log | grep "contract: "     (contract macos-latest)
contract macos-latest  Contract test against the Go binary  2026-11-10T10:28:43.7749360Z contract: 30 of 30 passed
```

Proof that the strict rule can fail. The old step lines on a log with 29 pass, the new rule fails:

```
$ printf 'PASS  a
contract: 29 of 30 passed
' > c29.log
$ grep "contract: " c29.log; echo "old rule exit $?"
contract: 29 of 30 passed
old rule exit 0
$ grep -q "contract: 30 of 30 passed" c29.log; echo "strict rule exit $?"
strict rule exit 1
$ : > empty.log; grep -q "contract: 30 of 30 passed" empty.log; echo "empty log exit $?"
empty log exit 1
$ printf 'contract: 30 of 30 passed
' > c30.log; grep -q "contract: 30 of 30 passed" c30.log; echo "30 log exit $?"
30 log exit 0
```

## G9

The owner's PC: Windows 11, NVIDIA GTX 1660 Ti 6 GB, 16 GB RAM, ComfyUI portable 0.44.0. Placeholders: `<repo>` is the main checkout, `<worktree>` is the G9 worktree, `<user>` is the user folder, `<comfyui>` is the ComfyUI portable folder. Python 3.12 with `mcp==2.3.0`. Go 1.27.3. Claude Code 2.1.322.

Baseline (ratchet): real images made by the Go binary: 0. `goal_check.py --real --no-ci`: 6 of 8.

### Step 1: build, and the GPU before the run

```
$ go build -o bin/horus.exe ./cmd/horus && go install ./cmd/horus
$ curl -s http://127.0.0.1:8188/queue
{"queue_running": [], "queue_pending": []}
$ nvidia-smi --query-gpu=memory.used,utilization.gpu --format=csv
memory.used [MiB], utilization.gpu [%]
0 MiB, 0 %
```

### Step 2: check-setup and the full model check

```
$ HORUS_ROOT=<repo> <worktree>/bin/horus.exe check-setup
PASS  ComfyUI folder - <comfyui>
PASS  ComfyUI server - answers at http://127.0.0.1:8188 (ComfyUI 0.44.0)
PASS  comfy-mcp environment - <user>\.venvs\comfy-mcp\Scripts\comfy-mcp.exe
PASS  .mcp.json - command points at <user>\.venvs\comfy-mcp\Scripts\comfy-mcp.exe
FAIL  horus on PATH - horus not found on the PATH. Fix: put the folder of the horus program on the PATH.
PASS  .mcp.json horus - command points at <user>\.venvs\horus-mcp\Scripts\horus-mcp.exe
1 check(s) failed.
exit 1
$ PATH="<user>/go/bin:$PATH" horus check-setup        (the folder of `go install` on the PATH of this shell only)
PASS  ComfyUI folder - <comfyui>
PASS  ComfyUI server - answers at http://127.0.0.1:8188 (ComfyUI 0.44.0)
PASS  comfy-mcp environment - <user>\.venvs\comfy-mcp\Scripts\comfy-mcp.exe
PASS  .mcp.json - command points at <user>\.venvs\comfy-mcp\Scripts\comfy-mcp.exe
PASS  horus on PATH - <user>\go\bin\horus.exe
PASS  .mcp.json horus - command points at <user>\.venvs\horus-mcp\Scripts\horus-mcp.exe
All checks passed.
exit 0
$ horus workflow list
it-illustration
  Text in, image out. The picture is 1536x1024. The model is FLUX 3 Image Klein (int8 build) running in ComfyUI. This is the first horus example.
  Models: 3 file(s), 11.02 GB (diffusion, text_encoder, vae). Installed: yes (size check).

Install: horus workflow install <name>    Check: horus workflow check <name>
exit 0
$ time horus workflow check it-illustration
Checking workflow 'it-illustration': 3 model file(s).
OK        diffusion <user>\ComfyUI-models\diffusion_models\flux3-image-klein-int8-convrot.safetensors (5827874779 bytes, SHA-256 verified)
OK        text_encoder <user>\ComfyUI-models\text_encoders\qwen3vl_4b_int8_convrot.safetensors (4858559240 bytes, SHA-256 verified)
OK        vae <user>\ComfyUI-models\vae\flux2-vae.safetensors (336211292 bytes, SHA-256 verified)
OK        models.json lists all 3 model file(s) that workflow.json loads
OK        the server has all 10 node classes of workflow.json
OK        the server lists flux3-image-klein-int8-convrot.safetensors
OK        the server lists qwen3vl_4b_int8_convrot.safetensors
OK        the server lists flux2-vae.safetensors
OK: workflow 'it-illustration' is ready.

real	0m41.762s
exit 0
$ horus sync-workflows                                 (no -force)
CONFLICT horus-it-illustration.json differs from workflows\it-illustration\workflow.json. Compare both, then run again with -force to use the repo copy.
exit 1
```

The SHA-256 of the pane copy was `741812b769b3…` before and after `sync-workflows`: the run changed nothing (see Decisions).

### Step 3: stop ComfyUI, then let horus start it

Before the stop, `/queue` had 0 running and 0 pending jobs, and the GPU used 0 MiB. The task stopped only the ComfyUI process of `<comfyui>` (found by its command line), then checked that the port did not answer:

```
CommandLine : "<comfyui>\python_embeded\python.exe" -s ComfyUI\main.py --windows-standalone-build --listen 127.0.0.1 --port 8188 --reserve-vram 1
ComfyUI does not answer: stopped
```

The Green run below started ComfyUI through the autostart (`horus start-comfyui` as a child of `horus mcp`). The server log of that run:

```
2026-11-10 17:54:36,155 horus INFO call generate_image progress_token=None
2026-11-10 17:54:36,158 horus INFO progress 0 starting the image backend (ComfyUI); the first start takes 1 to 3 minutes
2026-11-10 17:55:03,227 horus INFO progress 0 job 07ea9f4b-3826-4b10-9a6e-a94d42857ed7 submitted
2026-11-10 17:55:03,239 horus INFO progress 27 rendering, 27 s
2026-11-10 17:55:14,091 horus INFO progress 38 rendering, 38 s
(19 more progress lines, one every 10 s)
2026-11-10 17:58:34,292 horus INFO progress 238 rendering, 238 s
```

`<comfyui>\horus-comfyui.out.log` and `horus-comfyui.err.log` were written by `horus start-comfyui`. ComfyUI answered 27 s after the call. The `claude -p` run started ComfyUI again (`"started_backend": true`), because the Python MCP client ends the process tree of the server (see Decisions).

### Step 4: Red and Green of `tools/goal_check.py --real`

Red, before the check was written (commit `fedb5be`):

```
$ python tools/goal_check.py --real --no-ci --server-cmd <worktree>/bin/horus.exe mcp
PASS  contract - local 30 of 30; CI part skipped (--no-ci)
PASS  graph - TestUIToAPIGolden passes
PASS  workflow-cli - parity: 17 of 17 cases equal
PASS  downloader - 14 tests passed
PASS  setup-commands - 8 of 8 commands have a passing test
PASS  builds - 3 of 3 build
FAIL  real-image - the real-image check is not written yet (task G9)
FAIL  no-python - 2827 lines left; 18 files of Python and PowerShell remain; .mcp.json horus entry is not `horus mcp`
goal: 6 of 8 rows passed
```

Two runs of the new check failed in the check's own code, and were fixed in this task: a PNG signature with raw bytes in the source (`SyntaxError`), and `read_timeout_seconds` given as a `timedelta` (`mcp` 2.3.0 needs a number: `TypeError: unsupported operand type(s) for +: 'float' and 'datetime.timedelta'`, shown as `ExceptionGroup: unhandled errors in a TaskGroup`). The row now names the first error inside a task group.

Green (ComfyUI stopped before the run, autostart on):

```
$ HORUS_ROOT=<repo> python tools/goal_check.py --real --no-ci --real-out <repo>\.scratch\g9 --server-cmd <worktree>\bin\horus.exe mcp
PASS  contract - local 30 of 30; CI part skipped (--no-ci)
PASS  graph - TestUIToAPIGolden passes
PASS  workflow-cli - parity: 17 of 17 cases equal
PASS  downloader - 14 tests passed
PASS  setup-commands - 8 of 8 commands have a passing test
PASS  builds - 3 of 3 build
PASS  real-image - 1536x1024 <repo>\.scratch\g9\real-image-seed7-20261110-175436.png (seed 7, 246 s)
FAIL  no-python - 2827 lines left; 18 files of Python and PowerShell remain; .mcp.json horus entry is not `horus mcp`
goal: 7 of 8 rows passed

real	5m1.575s
```

`no-python` is the row of tasks G10 and G11.

Stub check. The row reads the width and the height from the IHDR chunk of the PNG. The same row against the fake ComfyUI of `tests/mcp_contract.py` (the Go server answers `ok`, and the file is the 1x1 PNG of the fake):

```
$ python stub.py <worktree>          (make_sandbox and Handler of tests/mcp_contract.py, HORUS_AUTOSTART=0, then row_real)
real-image row against the fake ComfyUI: (False, 'the PNG is 1x1, needs 1536x1024 (<temp>\\horus-stub-hgdqb5p9\\out\\real-image-seed7-20261110-180027.png)')
$ python -c "... png_size(...)"
fake PNG of the contract test: (1, 1) == REAL_SIZE -> False
empty file: None
no file: None
```

With `HORUS_AUTOSTART=0` and ComfyUI stopped, the row fails with the server's error:

```
(False, 'generate_image failed: BACKEND_UNAVAILABLE: ComfyUI does not answer at http://127.0.0.1:8188, and HORUS_AUTOSTART is off')
```

### Step 5: one image from Claude Code in headless mode

The MCP configuration (a temporary file outside the repo) names only the Go binary:

```
{"mcpServers":{"horus":{"command":"<worktree>\\bin\\horus.exe","args":["mcp"],"env":{"HORUS_ROOT":"<repo>"}}}}
```

```
$ CLAUDE_CODE_MCP_AUTO_BACKGROUND_MS=1800000 claude -p "<prompt>" --model sonnet --mcp-config <temp>\horus-mcp.json --strict-mcp-config --allowedTools mcp__horus__generate_image --output-format json
exit 0
```

The prompt asked for one `generate_image` call: workflow `it-illustration`, seed 11, `out_path` `<repo>\.scratch\g9\claude-p-seed11.png`, the subject of the contract test, wait for the result, print `ok`, `path`, `width`, `height`, `seed` and `seconds`. From the JSON output:

```
init: mcp_servers [{'name': 'horus', 'status': 'connected', 'source': 'dynamic'}] model claude-sonnet-5-5
tool_use mcp__horus__generate_image {'workflow': 'it-illustration', 'seed': 11, 'out_path': '<repo>\\.scratch\\g9\\claude-p-seed11.png', 'subject': 'On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt. On the right, a sky-blue server box takes them off the belt.'}
tool_result {"ok":true,"result":{"workflow":"it-illustration","path":"<repo>\\.scratch\\g9\\claude-p-seed11.png","bytes":2543765,"width":1536,"height":1024,"seed":11,"seconds":233.3,"backend_seconds":210.7,"started_backend":true,"job_id":"fa1de67c-e827-41bb-8d20-74aede7eca20"},"error":null}
is_error = False
subtype = success
num_turns = 3
duration_ms = 238720
result = {"ok":true,"path":"<repo>\\.scratch\\g9\\claude-p-seed11.png","width":1536,"height":1024,"seed":11,"seconds":233.3}
```

### Step 6: the two images

The subject of both: "On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt. On the right, a sky-blue server box takes them off the belt." Both files are in `<repo>\.scratch\g9\` (ignored by git). An agent opened each file and compared it with the subject.

| File | Caller | Seed | Seconds | Bytes | Width x height | SHA-256 |
|---|---|---|---|---|---|---|
| `real-image-seed7-20261110-175436.png` | `goal_check.py --real` (MCP client) | 7 | 246 (the call, with the start of ComfyUI) | 2,421,991 | 1536 x 1024 | `696f74f15e52780aecf9444c44f421cff98caf7aad0b7c95550cb64a285e25f6` |
| `claude-p-seed11.png` | `claude -p` (Claude Code 2.1.322, Sonnet) | 11 | 233.3 (`seconds`), 210.7 in ComfyUI | 2,543,765 | 1536 x 1024 | `6e39870a82b1d5fd3ed3777f5de7c762ef348a8fbcbb4e71bbaaaee9b93aedc9` |

Verdict, seed 7: **matches the subject.** A mint-green server box is on the left and a sky-blue server box is on the right. Five yellow envelopes lie flat on a purple conveyor belt that runs from the green box to the blue box. Differences: the envelopes are amber-yellow, not a bright sun-yellow, and the belt is a saturated violet, darker than lilac. No object is missing and no position is wrong.

Verdict, seed 11: **matches the subject.** A mint-green server box is on the left, a sky-blue server box is on the right, and four yellow envelopes lie on a lilac conveyor belt between them. The belt colour is nearer to lilac than in the seed 7 image. Difference: the blue box has two labels with letters that make no words ("LIEENNS", "CTAEARING"). The subject asks for no text, so this is a model artefact, not a horus defect.

The pull request has no copy of the images: `gh` cannot attach a file to a pull request, and the repo keeps PNG files only in `workflows/*/examples/`. The table above has the SHA-256 of each file.

### After the run

ComfyUI ran again with the owner's command line (`... --port 8188 --reserve-vram 1`) and answered `ComfyUI 0.44.0`.

Ratchet: real images made by the Go binary 0 → 2. `goal_check.py --real --no-ci`: 6 → 7 of 8 rows.

## G10

Placeholders: `<worktree>` is the G10 worktree, `<user>` is the user folder. Python 3.12 with `mcp==2.3.0`, Go 1.27.3. `horus` on the PATH is `<user>\go\bin\horus.exe`, made by `go install ./cmd/horus` in `<worktree>`.

Baseline (ratchet): lines of Python and PowerShell in the product: 2,827 (18 files). `goal_check.py --no-ci`: 6 of 8 rows.

### Red (commit `acf68f4`, before the change)

```
$ python tools/goal_check.py --no-ci
PASS  contract - local 30 of 30; CI part skipped (--no-ci)
PASS  graph - TestUIToAPIGolden passes
PASS  workflow-cli - parity: 17 of 17 cases equal
PASS  downloader - 14 tests passed
PASS  setup-commands - 8 of 8 commands have a passing test
PASS  builds - 3 of 3 build
FAIL  real-image - needs --real on the GPU machine
FAIL  no-python - 2827 lines left; 18 files of Python and PowerShell remain; .mcp.json horus entry is not `horus mcp`
goal: 6 of 8 rows passed
```

### What was deleted

`horus/` (8 files), `pyproject.toml`, `scripts/*.ps1` (10 files), `scripts/run-illustration.py`, `tools/make_graph_golden.py`, `tools/make_server_golden.py`. The golden files stay. After the deletion:

```
$ go vet ./... && go test -count=1 ./...
(18 lines "ok  github.com/hieplam/horus/...", 1 line "[no test files]" for internal/comfy/comfytest, no FAIL)
```

### Green

```
$ python tools/goal_check.py --no-ci
PASS  contract - local 30 of 30; CI part skipped (--no-ci)
PASS  graph - TestUIToAPIGolden passes
PASS  workflow-cli - parity: 17 of 17 cases equal
PASS  downloader - 14 tests passed
PASS  setup-commands - 8 of 8 commands have a passing test
PASS  builds - 3 of 3 build
FAIL  real-image - needs --real on the GPU machine
PASS  no-python - 0 lines left
goal: 7 of 8 rows passed
$ python tests/mcp_contract.py            (no arguments: horus mcp from the PATH)
...
PASS  docs/mcp.md lists every error code
contract: 30 of 30 passed
$ python tests/workflow_parity.py
parity: 17 of 17 cases equal
$ python scripts/lint-public.py
lint-public: 0 finding(s)
exit 0
$ gofmt -l .
(no output)
```

`real-image` fails without `--real`, as expected in this task (G9 proved it, G11 runs it again).

The test commands of `docs/mcp.md` ("Test the server"), run as written in PowerShell with a new venv `.scratch\test-venv` (`pip install -r tests\requirements.txt` installed `mcp` 2.3.0):

```
> .scratch\test-venv\Scripts\python tests\mcp_contract.py
contract: 30 of 30 passed
> .scratch\test-venv\Scripts\python tests\mcp_contract.py --server-cmd bin\horus.exe mcp
contract: 30 of 30 passed
```

### Stub check

The error codes come from the binary. With the row of `FILE_IN_USE` removed from a copy of `docs/mcp.md` (restored after the run):

```
$ python tests/mcp_contract.py
FAIL  docs/mcp.md lists every error code  -- missing: ['FILE_IN_USE']
contract: 29 of 30 passed
```

The default server command is `horus` from the PATH, and no Python server is left. With the Go bin folder removed from the PATH of the shell:

```
$ python tests/mcp_contract.py
FAIL  the server starts and answers over stdio  -- FileNotFoundError: [WinError 2] The system cannot find the file specified
contract: 0 of 30 passed
```

The `no-python` row counts the tracked files, so doing nothing leaves 2,827 lines (Red above).

### Search hits that are left

`git grep -i -e '\.ps1' -e 'horus-mcp' -e 'powershell' -e 'pyproject' -e 'run-illustration\.py' -e '\.venvs'`, without `docs/specs/` and `docs/plans/` (the spec, the plan and this log are history and keep their text):

| Where | Hit | Reason |
|---|---|---|
| `.mcp.json` lines 5 and 8 | `.venvs\comfy-mcp` | The `comfy-mcp` entry is Phase 3 of issue #4. It stays as it was. |
| `docs/setup.md` (step 2 table) | `.venvs\comfy-mcp` | The path of the comfy-mcp venv that `horus install-comfy-mcp` makes. |
| `docs/setup.md` (step 0) | `PowerShell`, `powershell` | The Windows command that adds the Go bin folder to the user Path. |
| `docs/why-flux3-klein.md` lines 62 and 63 | `PowerShell` | Two traps that the old download script found. The text says that `horus` calls no curl and no PowerShell. |
| 32 Go files under `cmd/horus/` and `internal/` (2 of them are the `herr` files of the next row) | `scripts/<name>.ps1`, `download-model.ps1`, `workflow.ps1`, `scripts/run-illustration.py` | Rule 2 of the spec: each Go file names the file that it ports. Comments that compare with the old behaviour (`{0:N2}`, the casts of `workflow.ps1`) name it too. |
| `internal/herr/herr.go`, `internal/herr/herr_test.go` | `.ps1` | The allowed difference of G1 (a `next_step` that named a `.ps1` script names the `horus` subcommand), and the test that no `next_step` names a `.ps1` script. |
| `internal/herr/testdata/errors.golden.json` | `scripts/start-comfyui.ps1` | The golden file holds the output of the old `horus/errors.py`. |
| `internal/scripts/checksetup/checksetup.go`, `internal/scripts/installcomfymcp/installcomfymcp.go` | `.venvs` | The default folder of the comfy-mcp venv. |
| `scripts/lint-public.py` | `.ps1` | The list of text suffixes that the lint reads. It is harmless and checks a `.ps1` file that someone adds. |
| `tests/workflow_parity.py` | `workflow.ps1` | The docstring says where the golden files came from. |
| `tools/goal_check.py` | `.ps1`, `powershell` | The `no-python` row itself. |

### Found

Issue #32: `horus register` adds the user-scope entry without `HORUS_ROOT`, so the tools answer `INTERNAL` in a session outside the repo. It blocks G11 and G12. The docs name the limit and give the `claude mcp add ... --env HORUS_ROOT=...` command until the fix (see Decisions).

Ratchet: lines of Python and PowerShell in the product 2,827 → 0. `goal_check.py --no-ci`: 6 → 7 of 8 rows (the row left is `real-image`, which needs `--real`).

## Issue 32

Placeholders: `<user>` is the user folder. Go 1.27.3, `claude` 2.1.322. Option syntax from `claude mcp add --help`: `-e, --env <env...>` (it takes many values, so it stands after the server name; the help example is `claude mcp add my-server -e API_KEY=xxx -- npx my-mcp-server`).

Ratchet: tools that answer outside the repo after `horus register`: 0 of 5 catalog-based answers before, all after (see the proof on the PC in the comment on #32).

### Red (the add command without `-e HORUS_ROOT=...`)

```
$ go test ./internal/scripts/register/
--- FAIL: TestRegisterAddsTheServerInUserScope
    register_test.go:106: no add command:
        args mcp add --scope user horus -- /the/path/horus mcp
--- FAIL: TestRunFindsClaudeOnThePath
        args mcp add --scope user horus -- /x/horus mcp
--- FAIL: TestRunTakesRootFromTheSettings
        args mcp add --scope user horus -- /x/horus mcp
FAIL	github.com/hieplam/horus/internal/scripts/register
```

### Green

```
$ go test -v ./internal/scripts/register/
--- PASS: TestRegisterAddsTheServerInUserScope
--- PASS: TestRegisterSkipsWhenClaudeKnowsTheServer
--- PASS: TestRegisterReportsAFailedAdd
--- PASS: TestRunFindsClaudeOnThePath
--- PASS: TestRunWithoutClaude
--- PASS: TestRegisterRefusesARootWithoutWorkflows
--- PASS: TestRunWithoutARepoFails
--- PASS: TestRunTakesRootFromTheSettings
ok  	github.com/hieplam/horus/internal/scripts/register

$ cd <temp folder outside the repo> && horus register
horus register: <bin folder> has no workflows folder, so it is not the horus repo. Run horus register inside the repo, or pass -root <repo folder>.
exit 1
```

## Issue 34

Goal row `contract`: the same count for a relative, an absolute and a PATH server command.

Red (commit `823c94d`, `bin/horus.exe` built from it, repo root of the worktree):

```
$ python tests/mcp_contract.py --server-cmd bin/horus.exe mcp
FAIL  docs/mcp.md lists every error code  -- missing: ['<bin/horus.exe errors -json failed: [WinError 2] The system cannot find the file specified>']
contract: 29 of 30 passed
```

Green (after the fix):

```
$ python tests/mcp_contract.py --server-cmd bin/horus.exe mcp
contract: 30 of 30 passed
$ python tests/mcp_contract.py --server-cmd bin\horus.exe mcp
contract: 30 of 30 passed
$ python tests/mcp_contract.py --server-cmd <absolute path>/bin/horus.exe mcp
contract: 30 of 30 passed
$ python tests/mcp_contract.py            (horus from the PATH)
contract: 30 of 30 passed
$ python tests/mcp_contract.py --server-cmd bin/nope.exe mcp
contract: 0 of 30 passed
```

Stub check: with a program that exists, answers the MCP session, and has no `errors` command, the line fails:

```
FAIL  docs/mcp.md lists every error code  -- missing: ['<...python.exe errors -json failed: Expecting value: line 1 column 1 (char 0)>']
contract: 29 of 30 passed
```

Ratchet: forms of the server command that give 30 of 30: 2 of 3 (absolute, PATH) -> 3 of 3 (plus relative).

## G11

Placeholders: `<repo>` is the main checkout, `<user>` is the user folder. Commit under test: `d7696c622be8d947e6e1e1a53140ec1300928796` (main, merge of PR 36). Clean worktree of that commit. `bin/horus.exe` built from it with `go build -o bin/horus.exe ./cmd/horus`. The PC: Windows 11, GTX 1660 Ti 6 GB, ComfyUI 0.44.0. Go 1.27.3. Python 3.12 with `mcp==2.3.0`.

Baseline (ratchet): `goal: 0 of 8 at G0` (7 of 8 after G10 without `--real`).

### Step 1: before the run

```
$ nvidia-smi --query-gpu=memory.used,utilization.gpu --format=csv
memory.used [MiB], utilization.gpu [%]
0 MiB, 0 %
$ curl -s http://127.0.0.1:8188/queue
{"queue_running": [], "queue_pending": []}
$ gh run list --branch main --limit 1
completed	success	Merge pull request #36 from hieplam/fix/register-horus-root	ci	main	push	42810496659	1m42s	2026-11-10T11:48:42Z
```

### Step 2: the goal check with the real image and the CI part

```
$ HORUS_ROOT=<repo> python tools/goal_check.py --real --real-out <repo>\.scratch\g11 --server-cmd bin/horus.exe mcp
PASS  contract - local 30 of 30; CI contract job green on windows-latest and macos-latest
PASS  graph - TestUIToAPIGolden passes
PASS  workflow-cli - parity: 17 of 17 cases equal
PASS  downloader - 14 tests passed
PASS  setup-commands - 8 of 8 commands have a passing test
PASS  builds - 3 of 3 build
PASS  real-image - 1536x1024 <repo>\.scratch\g11\real-image-seed7-20261110-185211.png (seed 7, 216 s)
PASS  no-python - 0 lines left
goal: 8 of 8 rows passed
exit 0
```

### Step 3: the image

| File | Bytes | Size | SHA-256 | Seed | Seconds |
|---|---|---|---|---|---|
| `<repo>\.scratch\g11
eal-image-seed7-20261110-185211.png` | 2,421,991 | 1536 x 1024 | `9f8fbe697dc813d1e8ca17a7152e651fd7682df008790f51e8c217e21bee6080` | 7 | 216 |

Subject of the contract test: "On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt. On the right, a sky-blue server box takes them off the belt."

Verdict: the image matches the subject.
- Left: a mint-green server box with a rack front.
- Right: a sky-blue server box with a rack front.
- Between them: a lilac (purple) conveyor belt with four envelopes on it. Three lie near the green box and one lies near the blue box.
- Colour: the envelopes are amber-gold, a little darker than "sun-yellow". The objects, their count and their positions are right. This is the model's reading of the colour, not a defect of the port (see Decisions).

### Step 4: `check --dry-run` of every closed task of the epic

```
$ epic.mjs check 7 --dry-run
PASS #7 via PR #20 (https://github.com/hieplam/horus/pull/20)
$ epic.mjs check 8 --dry-run
PASS #8 via PR #21 (https://github.com/hieplam/horus/pull/21)
$ epic.mjs check 9 --dry-run
PASS #9 via PR #24 (https://github.com/hieplam/horus/pull/24)
$ epic.mjs check 10 --dry-run
PASS #10 via PR #23 (https://github.com/hieplam/horus/pull/23)
$ epic.mjs check 11 --dry-run
PASS #11 via PR #26 (https://github.com/hieplam/horus/pull/26)
$ epic.mjs check 12 --dry-run
PASS #12 via PR #29 (https://github.com/hieplam/horus/pull/29)
$ epic.mjs check 13 --dry-run
PASS #13 via PR #22 (https://github.com/hieplam/horus/pull/22)
$ epic.mjs check 14 --dry-run
PASS #14 via PR #25 (https://github.com/hieplam/horus/pull/25)
$ epic.mjs check 15 --dry-run
PASS #15 via PR #30 (https://github.com/hieplam/horus/pull/30)
$ epic.mjs check 16 --dry-run
PASS #16 via PR #31 (https://github.com/hieplam/horus/pull/31)
$ epic.mjs check 17 --dry-run
PASS #17 via PR #33 (https://github.com/hieplam/horus/pull/33)
$ epic.mjs check 27 --dry-run
PASS #27 via PR #28 (https://github.com/hieplam/horus/pull/28)
$ epic.mjs check 32 --dry-run
PASS #32 via PR #36 (https://github.com/hieplam/horus/pull/36)
$ epic.mjs check 34 --dry-run
PASS #34 via PR #35 (https://github.com/hieplam/horus/pull/35)
```

### Step 5: supporting evidence on this tree

```
$ go test -count=1 ./...
ok  	github.com/hieplam/horus/cmd/horus	4.237s
ok  	github.com/hieplam/horus/internal/catalog	2.422s
ok  	github.com/hieplam/horus/internal/comfy	5.700s
?   	github.com/hieplam/horus/internal/comfy/comfytest	[no test files]
ok  	github.com/hieplam/horus/internal/generate	5.478s
ok  	github.com/hieplam/horus/internal/herr	1.351s
ok  	github.com/hieplam/horus/internal/models	4.983s
ok  	github.com/hieplam/horus/internal/scripts/checksetup	4.861s
ok  	github.com/hieplam/horus/internal/scripts/downloadmodel	5.435s
ok  	github.com/hieplam/horus/internal/scripts/installcomfymcp	5.901s
ok  	github.com/hieplam/horus/internal/scripts/installcomfyui	6.020s
ok  	github.com/hieplam/horus/internal/scripts/newworkflow	1.458s
ok  	github.com/hieplam/horus/internal/scripts/register	7.402s
ok  	github.com/hieplam/horus/internal/scripts/runillustration	3.963s
ok  	github.com/hieplam/horus/internal/scripts/startcomfyui	12.990s
ok  	github.com/hieplam/horus/internal/scripts/syncworkflows	2.013s
ok  	github.com/hieplam/horus/internal/scripts/workflow	3.413s
ok  	github.com/hieplam/horus/internal/server	2.886s
ok  	github.com/hieplam/horus/internal/settings	1.492s
exit 0
$ gofmt -l .
exit 0
$ python scripts/lint-public.py
lint-public: 0 finding(s)
exit 0
$ python tests/workflow_parity.py
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
```

Ratchet: `goal: 0 of 8 at G0 -> 8 of 8`. Real images made by the Go binary on main: 2 (G9) -> 3.

## Issue 38

Red, before the edit, on `origin/main` (`git show origin/main:README.md | sed -n 94,100p`). The issue's own Red command
(`git grep -n "does not" | grep -i "set yet"`) prints nothing, because "does not" and "set yet" are on two lines. The
working Red:

```
$ git show origin/main:README.md | grep -n "does not\|set yet"
81:Claude Code reads `.mcp.json` by itself, but it does not build or install the
98:this repo also needs the variable `HORUS_ROOT`, which `horus register` does not
99:set yet (setup step 5 link, quoted without the link syntax).
```

Line 81 is about `.mcp.json` and is not a hit. Lines 98 and 99 are the stale statement.

Green, after the edit. The new sentence on the branch `fix/readme-register`:

```
horus register also sets
the variable `HORUS_ROOT` to the repo folder. You do not set it by hand
```

```
$ grep -n "set yet" README.md | wc -l
0
$ grep -n "HORUS_ROOT" README.md
98:the variable `HORUS_ROOT` to the repo folder. You do not set it by hand
$ grep -n "does not\|set yet" README.md | grep -i horus   (no output)
$ python scripts/lint-public.py
lint-public: 0 finding(s)
exit 0
```

Other `horus register` statements checked against `internal/scripts/register/register.go`: `docs/mcp.md:33` and
`docs/setup.md:252` (run inside the repo, `-root`, error when no repo) match the code. No contradiction remains
outside `docs/specs` and `docs/plans`.

Stub check: deleting the section also gives no `set yet` hit, so the Green also requires the `HORUS_ROOT` sentence,
and the heading "Give other agents an image tool" is kept.

Ratchet: stale statements about `horus register` in `README.md`: 1 -> 0 (line 98 of the old text, plus its
continuation on line 99).

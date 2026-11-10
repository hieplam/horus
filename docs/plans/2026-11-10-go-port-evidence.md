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

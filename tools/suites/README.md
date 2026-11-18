# Suites of the goal check

A suite is the list of goal rows of one spec, with the check of each row. Each suite is one TOML
file in this folder. The runner (`tools/goal_runner.py`) reads the file and runs the rows:

```
python tools/goal_check.py --list                    # the suites
python tools/goal_check.py --suite <name> [--no-ci] [--real] [--only <row>...]
```

`--phase N` is the same as `--suite phaseN`. Without `--suite` and `--phase`, the runner checks `phase1`.

## Add a suite for a new spec

1. Write the goal rows in the spec first. Each row states an outcome and its oracle.
2. Copy the example below to `tools/suites/<name>.toml`. Use the name of the feature, for example `gpu-bench.toml`.
3. Give each row one or more parts. Use the check types of the table. A check that no type gives goes in
   `tools/checks/<name>.py` (see "A new check type").
4. Run `python tools/goal_check.py --list`. A wrong check type or a wrong parameter prints `cannot run` and names the row and the part.
5. Run the suite before you build. The rows must fail (the Red step). After the build, they must pass.

## The format

```toml
title = "a new feature"                    # one line: what the suite checks
spec = "docs/specs/2026-12-01-feature.md"  # the spec that holds the goal rows
real_where = "on the GPU machine"          # optional: where --real must run, for the note line
real_needs = "on the GPU machine"          # optional: the default "needs" of each real part

[[rows]]
name = "version"                           # the row name of the spec
stop_at_fail = true                        # optional: stop at the first part that fails

[[rows.parts]]
check = "command"
cmd = ["{horus}", "version"]
expect = '^horus \d+\.\d+\.\d+$'

[[rows.parts]]
check = "go_pinned_tests"
pins = [
  ["cmd/horus", "TestVersionPrintsTheVersionOfTheLinkerFlag", "T1"],
]

[rows.real]                                # optional: runs only with --real
check = "mcp_image"
needs = "on the GPU machine"
```

A row passes when each part passes. The detail of the row joins the details of the parts with `; `.
Without `--real`, a row with a real part stands on its other parts, and its detail says `real part not run`.
A part never needs `--real`: put a step that needs a GPU, a Mac or a release in `[rows.real]`.

## Check types

| `check` | Parameters | Passes when |
|---|---|---|
| `go_pinned_tests` | `pins` (list of [package folder, test name, task]), `texts` (optional: {test name: text}) | each pinned test exists, asserts something, holds its text, and reports `--- PASS`. An empty body, a skipped test and a missing name fail. |
| `go_tests` | `packages` (list), `names` (optional list) | `go test -v` of the packages passes, and tests ran. With `names`, each named test reports `--- PASS`. |
| `go_test_all` | none | `go test ./...` passes. |
| `test_count` | `baseline` (number) | the number of Go test functions is not below `baseline`. |
| `archtest` | none | the three tests of `internal/archtest` pass, and no allow list file exists. |
| `ci_green` | `jobs` (list of CI job names) | each job of the newest `ci.yml` run on HEAD is green, and no other job is red. `--no-ci` skips it. |
| `command` | `cmd` (list of words), `exit` (default 0), `expect` (optional regular expression), `timeout` (seconds) | the command ends with `exit`, and a line of its output matches `expect`. `{python}` is this Python. `{horus}` is the program of `--server-cmd` (default `horus`). |
| `files` | `paths` (list) | each file exists in the repo root. |
| `no_changes` | `commit`, `paths` (list), `what` (words for the detail) | no file of `paths` differs from `commit`. A rename with the same content is no change. |
| `contract` | `ci` (optional: list of jobs, or `false` for the local part only), `label` (optional) | all checks of `tests/mcp_contract.py` pass against `--server-cmd`. |
| `parity` | none | `tests/workflow_parity.py` passes with all cases equal. |
| `mcp_image` | none | one real image through the horus MCP server has the default size. Use it in `[rows.real]`. |

The functions are in `tools/checks/parts.py`. Each one documents its parameters.

## A new check type

When no check type fits, write a function in `tools/checks/<module>.py` and name it in the suite as
`"<module>.<function>"`:

```python
from checks import base as r

def config_has_port(a, port: int) -> tuple[bool, str]:
    text = (r.ROOT / "config.yaml").read_text(encoding="utf-8")
    ok = f"port: {port}" in text
    return ok, f"config.yaml {'names' if ok else 'does not name'} port {port}"
```

```toml
[[rows.parts]]
check = "gpubench.config_has_port"
port = 8189
```

The first parameter `a` gets the options of the command line (`a.no_ci`, `a.real`, `a.server_cmd`).
A function with no parameter gets no options. The other parameters come from the part.
Rules for the code:

- Use a part of another module through its module (`r.run`, `horus.real_image`), never with `from ... import`. A unit test can then replace the part in one place.
- Give each function a name that no other module of `tools/checks` uses. The test `OneNameOneItem` fails on a clash.
- When a check type is useful to more than one suite, put it in `tools/checks/parts.py` and add a row to the table above.

## The suites

| File | Spec |
|---|---|
| `phase1.toml` | `docs/specs/2026-11-10-go-port.md` |
| `phase2.toml` | `docs/specs/2026-11-11-restructure.md` |
| `phase3.toml` | `docs/specs/2026-11-12-macos-backend.md` |
| `phase4.toml` | `docs/specs/2026-11-15-release-install.md` |

The rows of Phases 1 to 4 were written as Python before this format. Many of them point to a function of
`tools/checks/phase<N>.py`. A new suite uses the check types of the table first.

# Spec: goal suites

Date: 2026-11-18. The decision record:
[ADR 0021](../adr/0021-goal-suites-are-go-test-packages.md). This work fits in
one pull request, so it has no epic.

> **Record of a finished phase.** All 6 goal rows passed on 2026-11-18. The
> suite `goals/2026-11-18-goal-suites/` is frozen: nobody edits it. A change to
> the goal suites gets a new spec and a new suite.

## What this work delivers

Each spec gets a goal suite: one Go test package that holds the oracle of each
goal row of that spec. `go test` runs a suite, so horus has no goal runner of
its own. A check links each suite to its spec by its name. A finished suite
stays in the tree as a record, and nobody edits it.

## Terms

- **Goal row:** one row of the table "Ratified goal" of a spec.
- **Oracle:** the code that decides PASS or FAIL for one goal row.
- **Goal suite:** the folder `goals/<spec stem>/`. It holds one test function
  for each goal row of the spec `docs/specs/<spec stem>.md`.
- **Spec stem:** the file name of a spec without `.md`.
- **Link check:** the test of the package `goals`. `go test ./...` runs it on
  each pull request. It reads the specs and the suites, and it runs no goal row.
- **Open spec:** a spec without the block "Record of a finished phase".
- **Frozen suite:** the suite of a spec that has that block.

## Ratified goal (owner, 2026-11-18)

| Row | Outcome | Oracle | Threshold | Ratchet |
|---|---|---|---|---|
| `link` | An open spec without a suite, a row without a function and a function without a row each fail `go test ./goals`. | The suite runs the tests of the link check by name. Those tests build a planted repo for each case. One test applies the link check to this repo. | Each named test reports `--- PASS`. | Checks that link a suite to its spec: 0 → 1 |
| `outside-only` | A suite that imports a horus package fails `go test ./goals`. | The suite runs the test of the link check with a planted suite that imports `internal/herr`. | The named test reports `--- PASS`. | Suites that can stop to compile after a later change of horus: not limited → 0 |
| `empty-row` | A row function that never uses `t` fails `go test ./goals`. | The suite runs the test of the link check with a planted empty function. | The named test reports `--- PASS`. | Empty row functions that pass the link check: not limited → 0 |
| `frozen-gate` | The Claude Code hook refuses a Write or an Edit of a file of a frozen suite. It allows a file of an open suite. | The suite starts `scripts/frozen-suites.py --hook` with a planted repo and both inputs. It reads `.claude/settings.json` for the hook. | Exit code 2 for the frozen suite, 0 for the open suite. A `PreToolUse` hook names the script. | Hooks that refuse an edit of a frozen suite: 0 → 1 |
| `ci-compiles` | CI compiles each suite. CI fails a pull request that changes a file of a suite that is frozen on the base commit. | The suite reads `ci.yml` for the two steps. It starts `scripts/frozen-suites.py --base` in a planted git repo with a changed frozen suite and with a changed open suite. After the push: the CI run of the pull request. | Both steps are in `ci.yml`. Exit code 1 for the frozen suite, 0 for the open suite. The CI run is green. | CI steps that read the suites: 0 → 2 |
| `workflow` | A new session finds the rules of a goal suite in `AGENTS.md`. No page names the Python goal tool as a tool of today. | The suite reads `AGENTS.md`. It searches the tracked files for the name of the Python goal tool outside `docs/specs/` and `docs/adr/`. | The section "Goal suites" has the run command. 0 files name the old tool. | Files that name the old tool as a tool of today: 5 → 0 |

**Done** means: all 6 rows pass from
`go test -tags goal -v ./goals/2026-11-18-goal-suites`, and the CI run of the
pull request is green.

## The design

- **The runner is `go test`.** A suite is a Go test package. Each file starts
  with `//go:build goal`, so `go test ./...` does not run a goal row. The path
  selects the suite: `go test -tags goal -v ./goals/<spec stem>`.
- **The link is the name.** The folder name of a suite is the spec stem. The
  row `same-behaviour` is the function `TestSameBehaviour`.
- **A suite tests horus from the outside.** It imports only the Go standard
  library. It starts programs and reads files.
- **The link check** (`goals/link_test.go`) fails for: a suite without a spec,
  an open spec with goal rows and no suite, a row without a function, a
  function without a row, a row function that never uses `t`, an import
  outside the standard library, and a file without the line `//go:build goal`.
- **The frozen gate** is `scripts/frozen-suites.py`. With `--hook` it is a
  Claude Code hook that runs before each Write or Edit: it reads the spec in
  the working tree. With `--base <commit>` it is a CI step: it reads the spec
  on the base commit, so the pull request that finishes a spec can still
  change its suite.
- **The Python goal tool of Phases 1 to 4** (`tools/`) leaves the tree. Read
  it with `git show 80609ae:tools/goal_check.py`.

## Scope

- In: the link check, the frozen gate, the two CI steps, the rules in
  `AGENTS.md`, ADR 0021, this suite.
- Out: a Go suite for Phases 1 to 4. Those specs are finished.

## Standing rules

- Never edit a frozen suite. A change to a finished feature gets a new spec
  and a new suite.
- Run only the suite of your own spec.

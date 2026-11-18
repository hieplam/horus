# 0021. A goal suite is a Go test package with the name of its spec, and it is frozen when the spec is finished

- **Status:** Accepted
- **Date:** 2026-11-18
- **Deciders:** the owner

## Context

Phases 1 to 4 had one Python goal tool, `tools/goal_check.py`: 1,605 lines
that held the runner and the oracles of the goal rows of all four specs.
[ADR 0001](0001-go-one-binary-per-platform.md) kept it in Python.

- The runner knew the oracles. It had the options of the oracles
  (`--server-cmd`, `--no-ci`, `--real`), a `--phase` option with the values 1
  to 4, and it searched the text of a result for "real part not run". A new
  spec had to change the runner.
- The names `phase1` to `phase4` did not name the spec of the rows.
- A split into a runner file and four suite files (pull request #176) moved
  the code, but the runner still held the Go test, CI job and settings logic.
  The owner closed it.
- On `main`, with a fresh build, the suites of Phases 1 to 3 gave 6 of 8, 4 of
  8 and 6 of 8 rows. Later phases changed files that an older oracle compared
  with a baseline. So a suite measures its own work, and nobody runs it after
  that work.
- CI did not run the goal tool or its 1,041 lines of unit tests.

Two facts, measured on 2026-11-18 with Go 1.27.3:

- Go refuses a hyphen in a build tag: `//go:build 2026-11-15-release-install`
  gives `invalid syntax at -`. So a spec stem cannot be a build tag.
- `go mod tidy` reads each file for all values of the build tags. A file with
  a build tag that imports a package that is no longer in the module makes
  `go mod tidy` fail.

## Decision

- The runner is `go test`. horus has no goal runner of its own.
- A goal suite is the Go test package `goals/<spec stem>/`. The folder name
  is the file name of its spec without `.md`. That name is the link.
- Each goal row of the spec is one test function: the row `same-behaviour` is
  `TestSameBehaviour`.
- Each suite file starts with `//go:build goal`. The tag has one job: it
  keeps the goal rows out of `go test ./...`. The path selects the suite:
  `go test -tags goal -v ./goals/<spec stem>`.
- A suite tests horus from the outside. It imports only the Go standard
  library, starts programs and reads files.
- The pull request of a spec adds the suite. A row that is not built yet
  fails. The task that builds the outcome gives the row its oracle.
- A piece of work runs only its own suite. No command runs all suites.
- A suite is a record, like a spec and an ADR. It stays in the tree. It is
  frozen when its spec gets the block "Record of a finished phase". A change
  to a finished feature gets a new spec and a new suite.
- The link check (`goals/link_test.go`, a part of `go test ./...`) fails for a
  suite without a spec, an open spec without a suite, a row without a
  function, a function without a row, a row function that never uses `t`, an
  import outside the standard library and a file without the build line.
- `scripts/frozen-suites.py` refuses a change to a frozen suite: as a Claude
  Code hook before each Write or Edit, and as a CI step on each pull request.
- Work that fits in one pull request gets a suite only when the owner ratifies
  goal rows for it. Then it also gets a short spec.
- The Python goal tool leaves the tree. This replaces one line of ADR 0001:
  `tools/goal_check.py` is no longer a developer tool. The other Python tools
  of that line stay.

## Consequences

- A new spec changes no runner and no CI file. It adds one folder.
- A reader finds the oracle of a goal row by the name of the spec.
- A frozen suite compiles for each later state of horus, because it imports
  no horus package. The cost: a row cannot call a Go function of horus. It
  runs a named Go test with `go test -run`, or the `horus` program.
- Each suite has its own small helpers (run a command, read a file). Suites
  share no code, so a change for one suite cannot change a frozen suite.
- `go test` prints no line "N of M rows passed". The pull request gives the
  count of `--- PASS` lines before and after.
- CI compiles the suites and does not run them. A row can need the GPU
  machine, and a row that is not built yet fails.
- The hook reads the spec in the working tree, and the CI step reads the spec
  on the base commit. So in the pull request that finishes a spec, the block
  is the last change. CI is the gate: the hook does not run for a file that a
  shell command writes, and it does not run in each agent program.
- A frozen suite can fail when someone runs it later. That is expected: it is
  the record of its own work.
- The goal tool of Phases 1 to 4 is in the history:
  `git show 80609ae:tools/goal_check.py`. Those four specs have no Go suite.
- To reverse this decision, restore `tools/` from commit `80609ae`, and take
  the section "Goal suites" out of `AGENTS.md`.

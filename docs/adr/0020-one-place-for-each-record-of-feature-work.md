# 0020. One place for each record of feature work: ADR, spec, issue, pull request

- **Status:** Accepted
- **Date:** 2026-11-18
- **Deciders:** the owner

## Context

Phases 1 to 4 each had a spec, a plan file and an evidence log. Measured on
2026-11-18 at commit `763b1bd`:

- The 4 specs had 806 lines, the 4 plans had 2,124 lines, and the 4 evidence
  logs had 6,835 lines. The Go code had 26,416 lines.
- The Verify block of one task was written in 4 places: the plan file, the
  issue, the body of the pull request and the evidence log.
- Decisions were in 3 places: the decision log of issue #4 (69 rows), the
  "Owner decisions" of a spec, and the "Decisions" table of each evidence log
  (335 rows). No decision had a "superseded" status.

The loop of an epic needs the goal table and the Verify blocks: they do the
job of the owner while the owner is away. It does not need the copies.

## Decision

Each record has one place.

| Record | Its one place |
|---|---|
| A decision that later work must follow | An ADR in `docs/adr/`, in the pull request of the work that it decides |
| The goal of a phase: the terms, the ratified goal rows, the design, the standing rules | A spec, `docs/specs/<date>-<topic>.md`, merged before the issues |
| The plan: the tasks, their order and the Verify block of each task | GitHub issues: the epic and one issue for each task |
| The proof of a task: Red, Green and Stub check with the literal output | The `## Verify` section of the pull request that closes the task |
| A decision of one task (a design gap, a standing rule) | A `## Decisions` section in the body of that pull request |

- The repo has no plan file and no evidence log. `docs/plans/` is removed.
- A spec has no task list and no decision table. It links the ADRs.
- A task that changes behaviour also changes the docs for that behaviour, in
  the same pull request. A docs-only task is for a document with no code
  change, for example the spec.
- Work that fits in one pull request needs no spec and no epic.
- When a task decision changes a rule for later work, the same pull request
  adds an ADR.
- `python scripts/lint-public.py --adr` checks the ADR folder: unique numbers,
  the five parts of each record, and the index. A Claude Code hook
  (`.claude/settings.json`) runs it after each Write or Edit of a file in
  `docs/adr/`, and CI runs it on each pull request. The agent does not need to
  remember the check.
- `AGENTS.md` and `docs/adr/README.md` give only the rule of today. The
  history of the rule is in this record.

For the epic-loop skill: the task bodies come from files in a scratch folder
(`"body"` in the manifest of `epic.mjs create`). The manifest has no `plan`
and no `evidence` entry, and `epic.mjs check <task>` runs without `--evidence`.

## Consequences

- One Verify block is in 2 places: the issue (what to prove) and the pull
  request (the proof).
- A reader finds the reason of a rule in one folder, and the index shows
  which record replaced which.
- The output of a command is not in the tree. A reader opens the pull request
  to see it.
- The plans and the evidence logs of Phases 1 to 4 are not in the tree. Read
  them from the history: `git show 763b1bd:docs/plans/<file>`. The issues of
  those phases still link to those files on `main`, and those links are dead.
  The body of each issue holds a copy of its plan section.
- To reverse this decision, restore `docs/plans/` from commit `763b1bd` and
  remove the rule from `AGENTS.md`.

## The old decision ids of issue #4

Until 2026-11-18 the decisions were rows D0 to D68 in the decision log of
[issue #4](https://github.com/hieplam/horus/issues/4). Old code comments,
issues and pull requests use those ids. This table gives the ADR that holds
each one. Each of those ADRs lists its old ids in its header.

| Old id | ADR | | Old id | ADR | | Old id | ADR |
|---|---|---|---|---|---|---|---|
| D0 | 0001 | | D23 | 0001 | | D46 | 0017 |
| D1 | 0001 | | D24 | 0010 | | D47 | 0007 |
| D2 | 0003 | | D25 | 0018 | | D48 | 0007 |
| D3 | 0003 | | D26 | 0006 | | D49 | 0007 |
| D4 | 0004 | | D27 | 0006 | | D50 | 0010 |
| D5 | 0007 | | D28 | 0006, 0005 | | D51 | 0010 |
| D6 | 0007 | | D29 | 0002 | | D52 | 0014 |
| D7 | 0005 | | D30 | 0015 | | D53 | 0009 |
| D8 | 0002 | | D31 | 0013 | | D54 | 0007 |
| D9 | 0016 | | D32 | 0016 | | D55 | 0003 |
| D10 | 0013 | | D33 | 0012 | | D56 | 0011 |
| D11 | 0002 | | D34 | 0015 | | D57 | 0011 |
| D12 | 0002 | | D35 | 0015 | | D58 | 0018 |
| D13 | 0001 | | D36 | 0013 | | D59 | 0014 |
| D14 | 0002 | | D37 | 0007, 0008 | | D60 | 0013 |
| D15 | 0001 | | D38 | 0010 | | D61 | 0015 |
| D16 | 0002 | | D39 | 0006 | | D62 | 0009, 0016 |
| D17 | 0007 | | D40 | 0018 | | D63 | 0011 |
| D18 | 0009 | | D41 | 0018 | | D64 | 0011 |
| D19 | 0002 | | D42 | 0007 | | D65 | 0012 |
| D20 | 0007 | | D43 | 0009 | | D66 | 0019 |
| D21 | 0002 | | D44 | 0008 | | D67 | 0018 |
| D22 | 0001 | | D45 | 0010 | | D68 | 0018 |

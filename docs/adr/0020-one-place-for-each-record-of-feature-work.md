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
- `scripts/lint-public.py` checks the ADR folder: unique numbers, the five
  parts of each record, and the index.

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

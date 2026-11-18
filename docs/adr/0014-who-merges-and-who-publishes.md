# 0014. An agent merges only a pull request of the owner. The owner publishes each release

- **Status:** Accepted
- **Date:** 2026-11-15
- **Deciders:** the owner
- **Old ids in issue #4:** D52, D59

## Context

This repo is public. A merged pull request from a stranger runs the code of
that stranger on the owner's machine. A release is public and cannot be taken
back. The session on the Mac uses a second account, `hoacao33`.

## Decision

- An agent merges a pull request of the account `hieplam` with squash when CI
  is green.
- A pull request from the session on the Mac comes from the account
  `hoacao33`. The owner merges it by hand.
- No agent approves or merges a pull request from a fork.
- The owner publishes each release: the owner merges the pull request of
  release-please (author `github-actions`). No agent merges it.

## Consequences

- The loop of an epic runs with no person, but a release and a pull request of
  a second account wait for the owner.
- To reverse this decision, the owner must say so. An agent never reverses it.

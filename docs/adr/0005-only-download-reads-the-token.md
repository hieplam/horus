# 0005. Only the download code reads HF_TOKEN

- **Status:** Accepted
- **Date:** 2026-11-11
- **Deciders:** the planning session of Phase 1. Phase 2 kept the rule
- **Old ids in issue #4:** D7, the token part of D28

## Context

The model files are in a gated Hugging Face repo, so a download needs a Read
token. This repo is public, and a log or a command line can show a token.

## Decision

- Only `internal/download` reads `HF_TOKEN`. It reads the token from the
  `.env` file that `settings.EnvPath()` names.
- The download code sends the token only in a header, and only to
  `huggingface.co` hosts.
- A token is never in a URL, a command line or a log.
- `go test ./internal/archtest` fails when a different package reads
  `HF_TOKEN`.

## Consequences

- A bug in a handler or a service cannot print the token, because that code
  never holds it.
- A new download source with a token needs a change of `internal/download`.
- To reverse this decision, remove the token rule from `internal/archtest`.
  Do not do this without a new ADR.

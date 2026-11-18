# 0016. The program holds the built-in workflows, and a folder can replace them

- **Status:** Accepted
- **Date:** 2026-11-15
- **Deciders:** the owner (embed), the planning session of Phase 4 (the copy)
- **Old ids in issue #4:** D9, D32, the copy part of D62

## Context

horus read the workflow folders from the repo at run time. A machine with only
the program has no repo, so `list_workflows` answered `BACKEND_NOT_INSTALLED`
("No workflows folder"). `internal/catalog` and `internal/provision` read a
folder path.

## Decision

- The build puts each folder of `workflows/` into the program with `go:embed`
  (`workflows/embed.go`), in full, with `examples/`. It does not embed
  `_template`.
- At start, horus copies the built-in workflows to
  `<user folder>/builtin-workflows`. The code still reads a folder path.
- A stamp file holds the SHA-256 of the embedded files. horus writes the copy
  again only when the stamp is different.
- `HORUS_WORKFLOWS_DIR` and a clone replace the built-in workflows.

## Consequences

- A machine with only the program lists and describes the workflows.
- A new workflow folder must be added to the `go:embed` line. The test
  `TestEveryWorkflowFolderIsEmbedded` fails when one is missing.
- A change that a person makes in `builtin-workflows` is lost at the next
  program version. A person who wants a changed workflow uses
  `HORUS_WORKFLOWS_DIR`.
- To reverse this decision, remove `workflows/embed.go`. horus then needs a
  clone again.

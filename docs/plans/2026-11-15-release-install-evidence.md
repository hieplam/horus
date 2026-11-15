# Evidence log: release, install line, full setup and uninstall (Phase 4)

Spec: [`../specs/2026-11-15-release-install.md`](../specs/2026-11-15-release-install.md).
Plan: [`2026-11-15-release-install.md`](2026-11-15-release-install.md).
Epic: [#134](https://github.com/hieplam/horus/issues/134).

Each task adds one section `## <task id>` with the literal output of its Green
commands.

## Decisions

| Date | Decision | Who | Why |
|---|---|---|---|
| 2026-11-15 | Phase 5 (`horus serve`) does not start now. | the owner | The Go MCP SDK v1.8.0 has no MCP tasks. |
| 2026-11-15 | `install_workflow` runs the whole setup plan. No sixth tool. | the owner | A caller learns no new tool. |
| 2026-11-15 | horus installs 7-Zip (`winget`) and Python 3.12 (`brew`) itself, only after a person approves the step. | the owner | A new machine has neither program. |
| 2026-11-15 | The owner turns on the Actions setting and merges the pull request of release-please (T10). | the owner | A release is public. |
| 2026-11-15 | The oracle of macOS is the runner `macos-latest`. The real run on the Mac is a follow-up outside the epic. | the planning session | This PC cannot reach the Mac, and the epic must end without a second machine. |
| 2026-11-15 | The user folder is `<home>/horus` on both systems (`HORUS_HOME`). | the planning session | One folder for a user to find and to remove. On macOS it is the install folder of decision D43. |
| 2026-11-15 | The built-in workflows are copied to `<user folder>/builtin-workflows`, and the code still reads a folder path. | the planning session | Design rule 1: no change of `internal/catalog` and `internal/provision` for a second way to read files. |
| 2026-11-15 | `horus uninstall` deletes a folder only when it has an install record, and it keeps `.env` and `outputs/`. | the planning session | Risk R8 of #4: an uninstall must never delete what horus did not install, or data of the user. |
| 2026-11-15 | A separate consent `accept_setup` with the code `SETUP_CONFIRMATION_REQUIRED` (22 codes). | the planning session | A caller that already sends `accept_license=true` must not approve a system package without a new question to the user. |
| 2026-11-15 | The release assets are plain program files and `checksums.txt`. No archive and no GoReleaser. | the planning session | Design rule 1: two `go build` commands need no tool. |

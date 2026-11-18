# 0009. The install folder, the user folder and the repo root

- **Status:** Accepted. The rule about a clone in `~/horus` is Proposed
- **Date:** 2026-11-15
- **Deciders:** the owner (the macOS install folder), the planning session of Phase 4 (the user folder and the repo root)
- **Old ids in issue #4:** D18, D43, D53, D62, risk R5

## Context

On macOS, TCC (Transparency, Consent and Control) stops a process from reading
`~/Desktop`, `~/Documents` and `~/Downloads` until the user gives access. A
machine with only the program has no repo, so `.env` and the images need a
folder outside a repo. The first root rule was: `HORUS_ROOT`, then the nearest
folder with `workflows/`, then the folder of the program.

## Decision

- **Install folder of ComfyUI.** macOS: `<home>/horus/ComfyUI`. Windows:
  `<home>/ComfyUI_windows_portable`. The rule is the pure function
  `platform.DefaultComfyDir`.
- **User folder.** `<home>/horus` on both systems. `HORUS_HOME` replaces it,
  and horus reads `HORUS_HOME` only from the environment, not from `.env`. The
  user folder holds `.env`, `outputs/` and the copy of the built-in workflows.
- **Repo root.** `HORUS_ROOT`, then the nearest folder at or above the current
  folder that has `workflows/_template/README.md`. The folder of the program is
  not a root.
- A person must not clone the repo into `~/horus` on macOS. `docs/setup.md`
  says so. A `horus doctor` check for it is issue #119.

## Consequences

- A user has one folder to find and to remove.
- The default folders are outside the TCC folders. A clone can still be in a
  TCC folder, so `horus doctor` prints a WARN line for it.
- A test must set `HORUS_ROOT` and `HORUS_HOME` to a sandbox. If it does not,
  it reads the `.env` of the real home folder.
- To reverse this decision, change `platform.DefaultComfyDir` and the settings
  package. Existing installs then need a move of their files.

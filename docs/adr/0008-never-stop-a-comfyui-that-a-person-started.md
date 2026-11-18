# 0008. Never stop a ComfyUI that a person started

- **Status:** Accepted
- **Date:** 2026-11-12
- **Deciders:** the planning sessions of Phase 2 and Phase 3. The owner ratified both phase goals
- **Old ids in issue #4:** the state file part of D37, D44

## Context

The ComfyUI of a user is the app of that user. It can hold a job of the user.
horus can start ComfyUI when the server does not answer, so horus also has a
stop command. comfy-cli 1.25.0 keeps one background ComfyUI for each user, and
`comfy stop` stops that one, also when a person started it.

## Decision

- When a backend starts ComfyUI, it writes the state file
  `horus-comfyui.state.json` with the address and the process id.
- `Stop` reads the state file. `Stop` refuses when the file is missing, when
  the address is different, or when that process does not run.
- The comfy-cli backend writes the same state file as the Windows backend. It
  takes the process id from the `--json` output of `comfy launch`. It runs
  `comfy stop` only after the state file check passes.

## Consequences

- `horus stop-comfyui` and a test can never end a ComfyUI of a person.
- A ComfyUI that horus started, with a deleted state file, must be stopped by
  the person.
- To reverse this decision, remove the check in `Stop`. Do not do this: a test
  on the owner's PC then can end the owner's ComfyUI.

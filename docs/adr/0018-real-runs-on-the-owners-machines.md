# 0018. How a real run uses the owner's machines and the CI runners

- **Status:** Accepted
- **Date:** 2026-11-16
- **Deciders:** the owner (the Mac rule and the fresh-machine rule), the planning sessions of Phases 1 to 4
- **Old ids in issue #4:** D25, D40, D41, D58, D67, D68

## Context

CI has no GPU, so a real image needs the owner's PC (NVIDIA) or the owner's
Mac (Apple M4). On the PC, the owner's ComfyUI runs on port 8188, and the
model files are in a folder that `extra_model_paths.yaml` maps, not in the
ComfyUI folder. ComfyUI 0.44 locks its database
(`ComfyUI/user/comfyui.db`), so one install folder cannot run a second
ComfyUI. The PC cannot reach the Mac.

## Decision

- A test of the real backend on the PC uses a second install folder and port
  8189. The owner's ComfyUI on port 8188 gets no job and keeps running.
- Code and tests find a model file through `extra_model_paths.yaml`. A test
  never changes the owner's model files.
- The install of the model files from nothing is proved on a machine with no
  files: the Mac in Phase 3, a CI runner after that.
- The fresh-machine run (`fresh-machine.yml` on `windows-latest` and
  `macos-latest`) runs the README commands outside a clone, installs ComfyUI
  for real, and runs `horus uninstall -yes`. It reads no secret and downloads
  no model file. A published release starts it.
- The oracle of macOS in an epic is the runner `macos-latest`. A real run on
  the Mac is a follow-up outside the epic.
- A finding of the agent acceptance that threatens no goal row is a follow-up
  outside the epic.

## Consequences

- An epic ends without a second machine and without the owner.
- A defect that shows only on a real Mac with a GPU is found later, by the
  follow-up run.
- To reverse this decision, make the real Mac run a task of each epic. Each
  epic then waits for the owner.

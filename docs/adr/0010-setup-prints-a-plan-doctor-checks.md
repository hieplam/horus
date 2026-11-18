# 0010. `horus setup` prints a plan and acts only with `-yes`. `horus doctor` checks

- **Status:** Accepted
- **Date:** 2026-11-12
- **Deciders:** the planning sessions of Phase 2 and Phase 3. The owner ratified both phase goals
- **Old ids in issue #4:** D24, D38, D45, D50, D51

## Context

A setup downloads about 13 GB and changes the machine of the user. The old
commands were `check-setup`, `install-comfyui` and `workflow install`, and a
person had to know their order.

## Decision

- `horus setup <workflow>` prints its plan and changes nothing. With `-yes` it
  runs the plan: the backend first, then the model files.
- `horus doctor` is the final name of `check-setup`. The old command names
  still work.
- `horus doctor` has three levels: PASS, WARN and FAIL. A WARN line does not
  change the exit code. On macOS it prints a WARN line for each path of the
  setup that is in `~/Desktop`, `~/Documents` or `~/Downloads`.
- `horus doctor -workflow <name>` checks the size and the SHA-256 of each
  model file. `-quick` checks only the sizes.
- Without `-server-url`, `horus doctor` checks `HORUS_COMFYUI_URL`, then
  `http://127.0.0.1:8188`.
- `docs/setup.md` is one page for Windows and macOS. The steps that are
  different are side by side.

## Consequences

- An agent can show the plan to the user before it changes the machine.
- A second `horus setup <workflow>` on a complete machine prints
  `Plan: 0 steps`. This line is the proof of a complete setup.
- A script that used the old command names still works.
- To reverse this decision, remove `-yes`. Each caller of `horus setup` then
  changes the machine with no plan first.

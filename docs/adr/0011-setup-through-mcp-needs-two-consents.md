# 0011. `install_workflow` runs the whole setup plan, after a setup consent and a license consent

- **Status:** Accepted
- **Date:** 2026-11-15
- **Deciders:** the owner (the three first bullets), the planning session of Phase 4
- **Old ids in issue #4:** D56, D57, D63, D64

## Context

A new machine has no 7-Zip (Windows) and no Python 3.12 (macOS), and the
install of ComfyUI needs them. An agent must be able to do the whole setup
through MCP. A caller that already sends `accept_license=true` must not
approve a system package with no new question to the user.

## Decision

- The tool `install_workflow` runs the whole plan of `horus setup`. horus gets
  no sixth tool.
- horus installs 7-Zip (`winget`) and Python 3.12 (`brew`) itself, and only
  after a person approves that step: `-with-packages` for `horus setup -yes`,
  and `accept_setup=true` for `install_workflow`.
- horus never removes a system package.
- A plan has a package step and a comfy-cli step only when ComfyUI is not
  installed. `-yes` with a package step and without `-with-packages` changes
  nothing and exits 1. Without `winget` or `brew`, the step is a step for a
  person.
- Without `accept_setup`, `install_workflow` changes nothing and answers
  `SETUP_CONFIRMATION_REQUIRED`. The setup consent comes first, then the
  license consent. Both come before the first change. horus asks for the
  license consent only when the plan downloads model files.

## Consequences

- A caller learns no new tool. A machine that has ComfyUI gets the same
  answers as before.
- horus has 22 error codes, and `docs/mcp.md` lists them.
- An agent must ask the user two times on a new machine.
- To reverse this decision, remove `accept_setup`. A caller then can install a
  system package with only the license consent.

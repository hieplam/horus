# 0012. `horus uninstall` removes only what horus installed

- **Status:** Accepted
- **Date:** 2026-11-15
- **Deciders:** the owner (the scope), the planning session of Phase 4 (the install record)
- **Old ids in issue #4:** D33, D65, risk R8

## Context

The owner's PC has the only copy of 11 GB of model files, in a folder that
horus did not make. An uninstall or a test of it can delete that copy. A user
can also have a ComfyUI that a different tool installed.

## Decision

- `horus uninstall` prints a plan of `REMOVE` and `KEEP` lines and changes
  nothing. It acts only with `-yes`.
- When horus makes a folder (the ComfyUI install folder, the venv of
  `horus install-comfy-mcp`), it writes the install record
  `horus-install.json` into it. `horus uninstall` deletes such a folder only
  when the record is there.
- It keeps `.env` and `outputs/`.
- It removes the model files by the rules of `horus workflow remove -force`,
  the MCP registration of both clients, the PATH entry of the install script,
  and the program file last.
- `-models` and `-mcp` limit it to one part.
- It never follows a link or a junction.
- A test of it uses a sandbox home folder or a CI runner. On the owner's PC,
  a command that changes files runs without `-yes`.

## Consequences

- A ComfyUI that a person installed stays after `horus uninstall -yes`.
- A folder that horus installed before the install record existed also stays.
  The person deletes it by hand.
- To reverse this decision, remove the record check. Do not do this: the
  uninstall then can delete data of the user.

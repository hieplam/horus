# 0007. One Backend interface, the portable build on Windows and comfy-cli on macOS

- **Status:** Accepted
- **Date:** 2026-11-12
- **Deciders:** the planning sessions of Phase 2 and Phase 3. The owner ratified both phase goals
- **Old ids in issue #4:** D5, D6, D17, D20, D37, D42, D47, D48, D49, D54

## Context

A backend is the way horus installs, starts and stops ComfyUI on one platform.
Windows used the portable build of ComfyUI for NVIDIA, which works and needs
7-Zip. macOS has no portable build. A test run on an Apple M4 with 16 GB
(issue #53) proved that `comfy install --m-series --version 0.44.0` works, and
that the int8 model files run on the Apple GPU backend (MPS). The cold run took
1033 s.

## Decision

- `internal/backend` declares the `Backend` interface with 4 methods:
  `Install`, `Start`, `Stop` and `Installed`. Its signatures hold no process
  id, no job name and no archive. The layout of an install is a private type
  of the package.
- Windows keeps the portable build, pinned to ComfyUI v0.44.0. The install
  downloads with Go `net/http` and unpacks with `7z.exe`.
- On macOS, `backend.New` gives the comfy-cli backend, pinned to the same
  version:
  - install: `comfy --workspace <dir>/ComfyUI install --m-series --version 0.44.0`;
  - start: `comfy --workspace <dir>/ComfyUI --json launch --background -- --listen 127.0.0.1 --port <port>`;
  - stop: `comfy --workspace <dir>/ComfyUI stop`.
- The comfy-cli backend finds `comfy` on the PATH, then in the venv of
  `horus install-comfy-mcp`. That venv holds comfy-cli 1.25.0 on both systems.
- To end a process tree, horus uses a Windows Job Object on Windows and a
  process group on macOS and Linux (`internal/platform`).
- On macOS, `horus install-comfyui` refuses the Windows flags `-archive-path`,
  `-seven-zip` and `-keep-archive` with exit 2. Linux and Intel Macs are not
  supported.
- `HORUS_GENERATE_TIMEOUT` keeps its default of 1800 s. The MPS text
  `MPS backend out of memory` gives the code `OUT_OF_MEMORY`.

## Consequences

- A new platform is a new type in `internal/backend`. No service changes.
- A Mac needs one install of comfy-cli, not two.
- A new ComfyUI version must be pinned and tested on both backends.
- To reverse this decision for one platform, replace its backend type. The
  interface stays.

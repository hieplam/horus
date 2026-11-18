# 0004. Call the ComfyUI REST API directly. Run comfy-cli and comfy-mcp only as separate processes

- **Status:** Accepted for the license rule. Proposed for the image path: the owner did not select an option, and the code follows the proposal
- **Date:** 2026-11-10
- **Deciders:** the planning session of Phase 1
- **Old ids in issue #4:** D4, risk R4

## Context

Two paths can make an image. On the first path, horus calls the REST API of
ComfyUI. On the second path, horus sends each call through `comfy-mcp`.
`comfy-mcp` is AGPL-3.0-or-later and `comfy-cli` is GPL-3.0-only. horus is MIT.
The `comfy model download` command of comfy-cli has no check of a pinned
SHA-256.

## Decision

- horus calls the ComfyUI REST API directly (`internal/comfy`).
- horus uses `comfy-cli` only to install, start and stop ComfyUI on macOS.
- horus runs `comfy` and `comfy-mcp` only as separate processes. horus never
  imports and never copies their code.
- horus keeps its own downloader, with a size check and a SHA-256 check.

## Consequences

- An agent that makes images never sees ComfyUI and needs no `comfy-mcp`.
- A change of the ComfyUI REST API can break horus. `internal/comfy` is the
  only package that must change.
- horus stays MIT.
- To reverse the image path, replace the adapter `internal/comfy`. The services
  call it through an interface.

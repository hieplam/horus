# 0029. horus runs as a Docker stack of 3 containers next to the ComfyUI of the host, and serves only when the 4 are up

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the owner, in the planning session of epic #180

## Context

The owner is the person who runs horus and manages its readiness. The words
of the owner: "horus have many dependencies ... as long as those 4 are up
horus is ready to serve", and "if docker down, whole horus down, agent can not
use this mcp".

- A fallback to the direct path sends every waiting agent straight to ComfyUI
  again: the overload that [ADR 0025](0025-run-each-job-through-a-broker-and-one-worker.md)
  stops.
- horus uses `http://127.0.0.1:8188` by default
  (`internal/settings/settings.go:167`): the ComfyUI of the owner. A ComfyUI in
  a container next to it makes two ComfyUIs that load models into the same
  6 GB GPU. The owner chose one ComfyUI, on the host.
- A container cannot start a process of the host, and it can write only to
  the folders that Docker mounts into it.

## Decision

- horus runs as the stack of `compose.yaml`: `horus-api`, `horus-worker` and
  `horus-db` (Postgres 18.6). `docker compose up -d` starts them together, in
  this order: `horus-db`, then `horus-worker`, then `horus-api`. Each one has
  `restart: unless-stopped`.
- ComfyUI stays on the host. A container reaches it at
  `host.docker.internal:8188`. horus never starts ComfyUI in the stack.
- horus is ready only when its 4 dependencies are up: the 3 containers and
  ComfyUI. `horus ready` runs the 4 checks.
- When a dependency is down, a new session cannot start: a stdio session
  exits, and `/mcp` answers 503. A call in an open session gets `NOT_READY`,
  with `action_by` human, because the owner manages the readiness.
- horus never falls back to the direct path.
- The worker takes no delivery while ComfyUI does not answer, so queued jobs
  keep their place.
- An `out_path` must be in a shared folder of `HORUS_SHARED_DIRS`. The stack
  mounts only those folders into `horus-api` and `horus-worker`.
- `install_workflow` in the stack downloads only model files, into
  `HORUS_MODELS_DIR`. The setup of ComfyUI stays on the host.
- On the host without `HORUS_DATABASE_URL`, `horus mcp` keeps the direct path
  of today.
- The first version builds the image of horus from a clone.

## Consequences

- When Docker is down, an agent has no horus tools.
- The cold start with one ComfyUI for each agent cannot happen in the stack.
  It stays on the direct path: a follow-up issue.
- Docker needs no GPU, so a Mac can run the stack too. The first real run is
  on the PC.
- An agent cannot ask for a PNG outside the shared folders.
- To reverse this decision, run `horus api` and `horus worker` on the host with
  `HORUS_DATABASE_URL`. The services do not change.

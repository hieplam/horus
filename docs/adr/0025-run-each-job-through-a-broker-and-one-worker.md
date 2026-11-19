# 0025. Run each job through a broker and one worker

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the owner, in the planning session of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md).

- The agent harness starts one `horus mcp` for each agent session. Each one
  submits its job to ComfyUI at once (`internal/generate/generate.go:429`).
- At peak time, calls end with `GENERATION_TIMEOUT`. The 1800 s limit counts
  from the submit, so it includes the wait in the ComfyUI queue
  (`generate.go:434`).
- Each `horus mcp` checks ComfyUI and starts it when it does not answer. No
  lock covers the time between the check and the start
  (`internal/backend/portable.go:326` and `:351`). A cold start at peak time
  can start one ComfyUI for each waiting agent. This is found in the code, and
  no run has shown it.
- ComfyUI knows only its own jobs. A second backend, for example the speech
  worker of [ADR 0022](0022-speech-runs-in-a-python-speech-worker.md), loads
  its own models, and nothing makes the two backends wait for each other.
- [ADR 0003](0003-stdio-transport-http-later.md) says: "horus has no daemon.
  ComfyUI is already a shared server with a job queue."

## Decision

- An agent call that makes an output becomes a job. The API process writes
  the job to the job store and publishes one message to a broker.
- One worker, `horus worker`, takes the messages and runs the jobs. An
  advisory lock in Postgres lets only one worker run.
- `HORUS_WORKER_CONCURRENCY` (default 1) is the number of slots: the jobs that
  the worker runs at the same time, for all backends together.
- The API process never calls a backend for a job. The worker never answers
  an MCP call.
- A job waits in the queue up to `HORUS_QUEUE_TIMEOUT` (default 7200 s). Its
  run has `HORUS_GENERATE_TIMEOUT` (default 1800 s) from the start of the run.
- A call whose estimated start is after its deadline, or a call when
  `HORUS_QUEUE_MAX` (default 20) jobs wait, gets `QUEUE_FULL` at once.
- A queued job with no waiter for 60 s becomes abandoned and never runs.
- `generate_image` keeps its contract: it waits until the PNG is on disk, and
  its new fields are optional.
- The first version runs a job at most one time. A second delivery of a job
  that a stopped worker held ends the job with `WORKER_LOST`.
- This replaces the line "horus has no daemon" of ADR 0003.

## Consequences

- ComfyUI never holds more horus jobs than the worker has slots.
- At peak time a caller waits longer in the queue and gets progress, not an
  error after 30 minutes. The GPU makes the same number of images in an hour.
- The worker is the one place that can decide which models are in memory. A
  speech job can go through the same broker later, as a work queue of its
  own. That is a follow-up issue, not a part of epic #180.
- The broker needs a running Postgres: see
  [ADR 0029](0029-horus-runs-as-a-stack-of-three-containers.md).
- To reverse this decision, remove the worker and the broker. The direct path
  of today stays, so `horus mcp` on the host keeps working.

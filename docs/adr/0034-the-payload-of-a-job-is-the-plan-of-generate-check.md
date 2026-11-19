# 0034. The payload of a job is the plan of generate.Check, and the worker holds a delivery until ComfyUI answers

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the agent of task #200, by the standing rules of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md), the
sections "The parts", "One job, step by step", "Readiness" and "Data in
horus-db".

- The API process checks the arguments of a call at once, and the worker runs
  the job later. `generate.Generate` did both steps in one call.
- [ADR 0027](0027-the-idempotency-key-and-the-idempotent-consumer.md): the API
  process fixes the output path at insert time, and the worker saves
  `backend_job_id` before it waits.
- The spec: "The worker takes no delivery while ComfyUI is down, so queued
  jobs keep their place." The broker gives a delivery when a slot is free, and
  the service `internal/jobs` runs a delivery only after the move from
  `queued` to `running`.
- The interface `jobs.Store` had no method for the heartbeat row in
  `horus.workers`, and none for the deletion of old jobs after
  `HORUS_JOB_RETENTION_DAYS`.
- The worker never starts ComfyUI: the owner starts it.

## Decision

- `generate.Check` checks a request and gives a `generate.Plan`: the workflow,
  the subject with its spaces joined, the seed (Check fixes a random seed),
  the width and the height after the defaults, the absolute output path and
  the step count. `generate.Run` runs a plan. `generate.Generate` is Check,
  then Run. The direct path calls Generate.
- The payload of a job of the tool `generate_image` is the JSON of
  `generate.Plan`. The API process makes it with Check. The worker decodes it
  and calls Run with `Options.NoStart` and `Options.Submitted`.
- The worker saves `backend_job_id` before it waits, with the event
  `job.submitted`.
- The gate of ComfyUI is `jobs.Consumer.Ready`. `Handle` calls it before the
  sweep and the move. While it waits, the broker holds the delivery for the
  worker, and the delivery counts toward the prefetch. The job stays queued at
  its place. A worker that stops during the wait leaves the job queued, and the
  next worker runs it.
- `jobs.Store` has `Heartbeat` and `DeleteOld`. `Heartbeat` writes the row of
  the worker with `heartbeat_at` at the time of the store. `DeleteOld` deletes
  each ended job whose `finished_at` is older than the age, with its events,
  and each worker row with an older heartbeat. It never deletes a queued or a
  running job. The store contract has the checks `heartbeat` and `delete_old`.
- Each start of `horus worker` gets a new worker id, a UUIDv7. A running job
  with another worker id runs on a worker that is gone.
- `WORKER_LOST` stays the code of the error of a job row
  (`jobs.CodeWorkerLost`). The task that gives the error of a job to a caller
  adds the code to `internal/herr` and to the error table of `docs/mcp.md`.

## Consequences

- The API process (task #202) calls `generate.Check` for each call and puts
  the plan in the payload. It refuses a bad argument before it inserts a job.
- A new field of `generate.Plan` needs a zero value that keeps the old
  behaviour, because a queued job can hold a plan of an older program.
- The readiness check of `horus-worker` and the Horus UI read the newest
  worker row from `Summary.Worker`.
- To reverse this decision, put the arguments of the call in the payload and
  let the worker call `generate.Generate`. Then the API process cannot refuse
  a bad argument at once, and the default output path is fixed by the worker.

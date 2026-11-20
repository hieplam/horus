# 0039. In the stack, generate_image takes and gives host paths in shared folders, and horus stack-mounts writes the mounts

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the agent of task #202, by the standing rules of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md), the
sections "One job, step by step", "Settings" and "MCP contract".

- [ADR 0029](0029-horus-runs-as-a-stack-of-three-containers.md): an
  `out_path` must be in a shared folder of `HORUS_SHARED_DIRS`, and the stack
  mounts only those folders into `horus-api` and `horus-worker`.
- An agent runs on the host. Its `out_path` is a path of the host, for example
  `C:\Users\you\Pictures\horus\cover.png`. The API process and the worker run in
  Linux containers. `generate.Check` refuses a Windows path there: it is not
  an absolute path of Linux.
- Docker Compose makes a mount only from a fixed line of a compose file. It
  cannot make one mount for each pair of a list in a variable.
- [ADR 0027](0027-the-idempotency-key-and-the-idempotent-consumer.md): the
  fingerprint holds the cleaned host path, in lower case on Windows and
  macOS. A container cannot tell a macOS host from a Linux host by a path.
- [ADR 0025](0025-run-each-job-through-a-broker-and-one-worker.md): the API
  process never calls a backend for a job. `generate_speech` of epic #181 runs
  its speech worker in the process of `horus mcp`, on the direct path.
- The estimated start of a queued job uses 240 s for one run until a job
  succeeded. The peak tool divides each time by 10. With a queue timeout of
  720 s and 240 s for each run, only 4 of 12 calls start before their
  deadline, so 8 calls get `QUEUE_FULL`.
- The goal row `queue-progress` needs a progress message at least every 10 s
  at the agent. A message every 10 s exactly comes through `docker exec` with a
  delay of some milliseconds, so a gap can be 10.003 s.

## Decision

- `HORUS_SHARED_DIRS` holds pairs `<host folder>=<container folder>`, with
  `;` between two pairs. The last `=` of a pair ends the host folder. Each
  container folder is one folder under `/shared/`.
- `horus stack-mounts`, on the host in the clone, writes
  `compose.override.yaml` next to `compose.yaml`: one bind mount of each pair
  into `horus-api` and `horus-worker`, and the settings `HORUS_SHARED_DIRS`,
  `HORUS_OUTPUT_DIR` (the container folder of the first pair) and
  `HORUS_HOST_OS` (the system of the host) of the two containers.
  `docker compose up -d` reads the file. Git and the image ignore it.
- Without `HORUS_SHARED_DIRS`, the one pair is the output folder of horus on
  the host and `/shared/outputs`.
- A test stack names its compose files with `COMPOSE_FILE`, so it never reads
  the `compose.override.yaml` of the owner.
- The API process changes the `out_path` of the host to the container path
  before `generate.Check`. The plan in the job row holds the container path.
  The result gives the host path, cleaned, with the drive letter in upper
  case.
- An `out_path` outside the shared folders gets `INVALID_ARGUMENT` with
  `details.shared_dirs`. A stack without a shared folder gets
  `BACKEND_NOT_INSTALLED` with a `next_step` that names `horus stack-mounts`.
- In the result and in the details of an error of a job, `job_id` is the
  horus job id, and `backend_job_id` is the id of the job in ComfyUI.
- In the stack, `generate_speech` stays in the list of tools and answers
  `BACKEND_UNAVAILABLE`, not retryable, with a `next_step` that sends the user
  to the horus program of the host for speech. The API process starts no
  speech worker. Issue #235 puts `generate_speech` through the broker.
- `HORUS_RUN_ESTIMATE` (default 240) is the run time in seconds that the
  estimated start uses until a job succeeded.
- A cancel of the call, or the end of its session, cancels the job: a queued
  job moves to `cancelled`, and the worker stops a running job.
- The first progress message of a call is the place of its job at the insert
  (`jobs.Service.SubmitStatus`). Then the waiter reports its progress at each
  read of the job store (every 5 s), and writes `waiter_seen_at` every 10 s.

## Consequences

- An agent gives and gets the paths of its own machine, in the stack and on
  the direct path.
- After a change of `HORUS_SHARED_DIRS`, the owner runs `horus stack-mounts`
  again, then `docker compose up -d`. Without the file, `generate_image` in the
  stack refuses each call with a clear `next_step`.
- An agent of the stack cannot make speech until issue #235.
- A test that runs the stack with a time scale sets `HORUS_RUN_ESTIMATE` with
  the other times.
- To reverse the path rules, mount one fixed folder in `compose.yaml` and
  accept only container paths. Then each agent must know the paths of the
  container.

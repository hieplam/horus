# 0036. One readiness report for horus ready, /readyz and horus mcp, and a health check for each container

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the agent of task #201, by the standing rules of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md), the
sections "Readiness", "Two ways in", "Settings" and "Code layout".
[ADR 0029](0029-horus-runs-as-a-stack-of-three-containers.md): horus serves
only when its 4 dependencies are up.

- Three places need the readiness: `horus ready`, `/readyz` of `horus api`
  and `horus mcp` in the stack. Three copies of the 4 checks can give three
  answers.
- The spec gives `horus-db` the check "`pg_isready`, and the schema has the
  version of the program". The worker applies the schema after `horus-db` is
  healthy. So a Docker health check of `horus-db` that needs the schema never
  passes, and the worker never starts.
- The check of `horus-api` in its own health check is `horus ready`. A check
  of `horus-api` that asks `/readyz` runs the other 3 checks again, and waits
  for the slowest one.
- The worker lock is a session advisory lock. A check that takes the lock to
  test it can make a worker that starts at the same time fail.
- A container reaches the host at `host.docker.internal`. Docker Desktop gives
  a container the loopback of the host there, so a fake ComfyUI on
  127.0.0.1 of the host answers (measured on the PC: `wget` from a container
  to a server bound to 127.0.0.1). Docker on Linux needs an address of the
  bridge or 0.0.0.0.
- `HORUS_DATABASE_URL` with a password shows the password in `docker
  inspect`, in a log of the URL and in an error.

## Decision

- One function, `mcpserver.Stack.Ready`, runs the 4 checks at the same time,
  each one in at most 2 s, and joins them with `jobdata.ReadinessOf`.
  `horus ready`, `/readyz` and `horus mcp` use it. A new place that needs the
  readiness uses it too.
- The checks of `horus-db` and `horus-worker` are `jobs.Service.Checks`, on
  the interface `jobs.Health` (`CheckSchema`, `WorkerLockHeld`) that
  `internal/postgres` implements. `WorkerLockHeld` reads `pg_locks` and takes
  no lock.
- The Docker health checks: `horus-db` is `pg_isready`. `horus-worker` is
  `horus ready -only horus-worker`. `horus-api` is `horus ready`. The schema
  version is a part of the check of `horus-db` in the report, not of its
  Docker health check.
- The check of `horus-api` asks `GET /livez` of `horus api`, which runs no
  check. In the process of `horus api` itself, the check is up.
- `horus mcp` with `HORUS_DATABASE_URL` runs the report at its start and at
  the start of each call. A failed start writes the report to the standard
  error and exits 1. A call while a dependency is down gets `NOT_READY`.
- `horus api` and `horus mcp` open the database pool without a connection
  (`postgres.Open`). A process that starts while `horus-db` is down reports
  it, and works when `horus-db` is up again.
- The password of the stack is `HORUS_DB_PASSWORD` in the `.env`.
  `compose.yaml` gives it to the containers as `PGPASSWORD`, and
  `HORUS_DATABASE_URL` holds no password.
- Each name of the stack comes from `HORUS_STACK`: the containers, the volume
  and the image. A test uses `HORUS_STACK=horus_test` and
  `docker compose -p horus_test`.
- The goal suite uses a fake ComfyUI on 127.0.0.1 of the host, not a
  container of a profile `test`. The real ComfyUI is on the host too, so the
  path of the fake is the path of the real one.

## Consequences

- `/mcp` (task #204) and the Horus UI (task #205) take their readiness from
  `Stack.Ready`, and give the same answer as `horus ready`.
- A goal row that runs the stack needs Docker Desktop. On Docker for Linux,
  the fake must listen on an address of the bridge.
- `horus ready` on the host without `HORUS_DATABASE_URL` reports `horus-db`
  and `horus-worker` as down. Run it in the stack.
- To reverse this decision, give each place its own checks, and give the
  container `horus-db` a health check that needs the schema after the worker
  applies it.

# 0042. The health check of horus-db asks over TCP, and horus waits for no database at its start

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the agent of task #257, by the standing rules of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md), the
section "Readiness".
[ADR 0036](0036-one-readiness-report-for-horus-ready-readyz-and-horus-mcp.md):
the Docker health check of `horus-db` is `pg_isready`, and Docker Compose
starts `horus-worker` when `horus-db` is healthy.

- On a new volume, the image `postgres:18.6` first runs a temporary server to
  make the database. The temporary server listens only on the Unix socket.
  The image then stops it and starts the server that listens on TCP.
- `pg_isready` without `-h` asks the Unix socket. So the health check
  `pg_isready -U horus -d horus` passed while the temporary server ran.
  Measured on the PC, every 50 ms: the health check gave exit 0 for 648 ms
  while `pg_isready -h 127.0.0.1` gave exit 2.
- `horus worker` connects over TCP, and exits with 1 when the database
  refuses the connection. When Docker Compose started `horus-worker` in that
  time, `docker compose up -d --wait` ended with exit code 1 ("container
  horus-worker is unhealthy"), also when a later restart of the container
  worked. One start in about 40 failed (issue #257).
- A second way to fix it is a wait in `horus worker`: try the database again
  at its start. Then two places hold the start order, and the health check
  of `horus-db` stays wrong for each other reader of it.

## Decision

- The health check of `horus-db` asks the database over TCP:
  `pg_isready -h 127.0.0.1 -U horus -d horus`. `horus-db` is healthy only
  when the database accepts a connection the way `horus worker` and
  `horus api` connect.
- The start order stays in one place: `depends_on` with
  `condition: service_healthy` in `compose.yaml`. `horus worker` gets no
  wait and no retry for the database at its start.
- The row `stack-starts-together` of the goal suite measures the health
  check that `compose.yaml` gives `horus-db`, in a container of its own on a
  new volume, next to `pg_isready -h 127.0.0.1`. It fails when the health
  check passes while the TCP check does not.

## Consequences

- A start of the stack on a new volume does not start `horus-worker` before
  the database accepts TCP.
- A change of the health check of `horus-db` must pass the row
  `stack-starts-together`. A health check that always passes fails it.
- A health check of a new container asks its service the way the clients of
  the service connect.
- `horus worker` still exits when the database is down at its start. Docker
  starts it again (`restart: unless-stopped`).
- To reverse this decision, remove `-h 127.0.0.1` and make `horus worker`
  wait for the database at its start.

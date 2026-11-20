# 0037. The worker checks its worker lock at each heartbeat, and takes it again when its session ended

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the agent of task #224, by the standing rules of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md), the
sections "Readiness" and "The broker interface".

- [ADR 0025](0025-run-each-job-through-a-broker-and-one-worker.md): an
  advisory lock in Postgres lets only one worker run.
- The Postgres job store takes the lock as a session lock on its own
  connection. When the session ends, Postgres releases the lock. A restart of
  `horus-db` ends each session.
- `horus worker` took the lock one time, at its start. After a restart of
  `horus-db`, the heartbeat and the broker worked again, but no session held
  the worker lock. The readiness check of `horus-worker` ("a heartbeat newer
  than 15 s, and it holds the worker lock") failed until a person restarted
  the container. A second worker could take the lock while the first one ran
  (issue #224).
- `jobs.Store.WorkerLock` gave only a release function. The worker could not
  ask if it still held the lock.
- The goal row `ready-when-four-up` needs horus ready again in 30 s or less
  after the part is up. The restart delay of Docker doubles from 100 ms up to
  1 minute while `horus-db` is down, so an exit and a restart by Docker can
  miss 30 s after a long stop.

## Decision

- `jobs.Store.WorkerLock` gives a `jobdata.Lock`: `Check` and `Release`.
  `Check` gives nil while the lock holds the one-worker lock. Else it gives
  an error that wraps `jobdata.ErrLockLost`.
- The release of a lost lock never frees the lock of the next holder.
- The Postgres lock runs `Check` on the session of the lock: it reads
  `pg_locks` for its own process. An error of the session is a lost lock.
- At each heartbeat (5 s), the worker checks its lock first. A worker without
  the lock writes no heartbeat.
- When the lock is lost, the gate starts no new delivery. The queued jobs keep
  their place. A job that runs goes on.
- The worker takes the lock again on a new session at once, then every 2 s
  (`worker.Config.LockRetry`). It does not exit, so the delay of Docker does
  not count.
- When another worker holds the lock at that time, the worker stops its jobs
  and exits with 1 (`worker.LockedError`). So two workers never run at the
  same time.
- The store contract has the check `worker_lock_lost`. Each job store passes
  it. Its hook ends the session of the lock: `EndLockSession` in the
  in-memory store, `pg_terminate_backend` in the Postgres store.

## Consequences

- After a restart of `horus-db`, the worker holds the lock again about 2 s
  after Postgres answers, and the readiness check passes at its next run.
- A new job store implements `Check`. A store without a session (for example
  a lock with a lease) gives `ErrLockLost` when the lease ended.
- A job that runs while the lock is lost can end with `WORKER_LOST` when
  another worker takes the lock first.
- A test that ends a Postgres session limits `pg_terminate_backend` to the
  database of the test, because other tests use the same server at the same
  time.
- To reverse this decision, give `WorkerLock` a release function again, and
  let the worker exit when its heartbeat fails. Then Docker starts it again,
  and the delay of Docker counts toward the 30 s.

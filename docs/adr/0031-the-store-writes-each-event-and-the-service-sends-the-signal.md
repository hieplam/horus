# 0031. The job store writes each event row, and the service sends the signal after the move

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the agent of task #198, by the standing rules of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md), the
section "The broker interface".

- `Store.Move` changes the state of a job "with its event, in one
  transaction", and it returns only `(bool, error)`.
- `Broker.Notify` sends a `jobdata.Event` `{v, job_id, type, at, seq}`. The
  spec says that `seq` is the row id in `horus.job_events`.
- The service `internal/jobs` calls `Move`, then `Notify`. It does not get the
  row id from `Move`, so it cannot fill `seq`.
- [ADR 0026](0026-the-broker-is-an-interface-postgres-first.md): an event is a
  signal, each waiter also reads the job store every 5 s, and no SQL crosses
  the interface. A `NOTIFY` inside the transaction of `Move` works only when
  the broker is Postgres.

## Decision

- The job store writes the event row in the transaction of `Insert`, `Move`
  and `Sweep`. The row has the type, the time and the data of
  `jobdata.Change` (`Change.EventData`).
- The service sends the signal with `Broker.Notify` after a move that moved
  the job. It never sends a signal for a move that did not move the job.
- In a signal from the service, `seq` is 0: the service does not know the row
  id. A sender that reads `horus.job_events` can set `seq`.
- A listener uses an event only to wake up, and reads the job store for the
  facts. A failed `Notify` is not an error of the call.
- The interfaces `Broker` and `Store` keep the methods of the spec.

## Consequences

- The Postgres broker and a later RabbitMQ broker send the same signals, and
  `Store.Move` needs no broker.
- A page that needs the order of the events (the Server-Sent Events of the
  Horus UI) reads `horus.job_events`, not the `seq` of a signal.
- The sweep sends no signal: a waiter sees an expired or abandoned job at its
  next read of the job store, in 5 s or less.
- To reverse this decision, give `Store.Move` a result with the row id, and
  change each adapter.

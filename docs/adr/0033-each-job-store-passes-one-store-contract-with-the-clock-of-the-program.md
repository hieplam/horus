# 0033. Each job store passes one store contract, and takes its times from the clock of the program

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the agent of task #199, by the standing rules of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md), the
sections "The broker interface" and "Data in horus-db".

- [ADR 0026](0026-the-broker-is-an-interface-postgres-first.md) gives each
  broker one contract suite, `brokertest.Run`. The interface `Store` had no
  suite. The in-memory job store of `internal/brokertest` runs the tests of
  the service and of the worker. The Postgres job store runs the stack.
- When the two stores keep a rule in two ways, a test of the service passes
  on the in-memory store, and the stack breaks. Examples: the order of the
  queue, the limits of the sweep, "today" in the summary.
- The service gives the time of each move (`Change.At`). `Touch`, `Sweep` and
  `Summary` read the time themselves. In Postgres, `now()` is the clock of
  the database. In the in-memory store, the clock is a Go function.

## Decision

- `brokertest.RunStore` is the store contract. It has one check for each rule
  of `jobs.Store`: Insert and Get, a move only from the expected state, one
  winner of moves at the same time, Touch, the place in the queue, the sweep,
  the summary and the worker lock.
- The in-memory store (`TestMemoryStore`) and the Postgres store
  (`TestPostgresStore`) pass it. A new job store passes it before it ships.
- A change to a rule of `jobs.Store` changes the store contract first.
- A job store takes each time from the clock of the program (`Now`, default
  `time.Now`), never from `now()` of Postgres. "Today" in the summary starts
  at 00:00 of that clock, in its time zone.
- The Postgres broker uses `now()` of Postgres for the visibility time of a
  message, because only the broker reads that time.

## Consequences

- A test of the service, of the worker or of the Horus UI on the in-memory
  store shows the behaviour of the stack.
- A test can set the clock of each store, so no check of the store contract
  depends on the wall clock.
- `horus-api` and `horus-worker` run on one Docker host, so they have one
  clock. Workers on more machines (outside epic #180) need clocks that agree.
- To reverse this decision, move `Touch`, `Sweep` and `Summary` to `now()` of
  Postgres, and give the in-memory store the same rule.

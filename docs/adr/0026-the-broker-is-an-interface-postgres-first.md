# 0026. The broker is an interface. Postgres is the first broker, and the job store stays in Postgres

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the owner, in the planning session of epic #180

## Context

The owner, on the design document of epic #180: "use postgres as a
lightweight message broker. We can change to RabbitMQ in the future if we
need to ... communicate via interface, so when we need to change we just need
to change the implementation."

- A broker such as RabbitMQ deletes a message after its ack. It cannot show
  or remove a message in the middle of a queue, and it has no lookup by key.
- The Horus UI needs the jobs that ended. The queue position, a cancel of a
  queued job and the dedupe of [ADR 0027](0027-the-idempotency-key-and-the-idempotent-consumer.md)
  need the other two.
- Postgres and RabbitMQ both deliver a message again when its consumer stops
  before the ack (at-least-once).

## Decision

- The service `internal/jobs` declares two interfaces: `Broker` (publish,
  consume with a prefetch, ack, nack, notify, listen) and `Store` (the jobs,
  their states and their events).
- `internal/postgres` implements both. The Postgres broker uses its own table
  `horus.messages` and the channels `horus_messages` and `horus_events`. It
  never reads a job table. The job store never reads the message table.
- The job store stays in Postgres for every broker. It is the record of truth.
- A message carries only `v`, `job_id`, `tool` and `backend`. The job store
  holds the arguments and the state.
- An event is a signal. The job store keeps each event. The broker only wakes
  the processes that listen, and each waiter also reads the job store every
  5 s.
- No SQL, no channel name and no exchange name crosses the interface.
- One contract suite, `internal/brokertest`, runs on each broker. The
  in-memory broker of the tests and the Postgres broker pass it. A new broker
  passes it before it ships.

## Consequences

- A swap to RabbitMQ adds one container, the package `internal/rabbitmq` and
  the setting `HORUS_BROKER`. It changes no service, no handler and no table.
- The first version has two Postgres tables for one job: its row and its
  message. That is the cost of a broker that can change.
- The field of a message is `tool`, not `kind`, because `kind` is the kind of
  a workflow ([ADR 0023](0023-a-workflow-has-a-kind-and-each-kind-has-its-tool.md)).
- To reverse this decision, let the worker read `horus.jobs` with
  `FOR UPDATE SKIP LOCKED` and remove `Broker`. A later swap to RabbitMQ then
  changes the services.

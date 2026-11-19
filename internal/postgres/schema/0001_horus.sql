-- The schema horus, version 1: the job store and the table of the Postgres
-- broker. The spec: docs/specs/2026-11-19-job-queue.md, "Data in horus-db".
-- Migrate applies this file one time, in its transaction, after it made the
-- schema horus and the table horus.schema_version.
--
-- An empty string, a zero time and a nil value of the program are NULL in a
-- row. The broker also uses two channels, which are not tables:
-- horus_messages (the payload is the name of a work queue) and horus_events
-- (the payload is the JSON of a jobdata.Event).

-- The job store: one row for each job.
CREATE TABLE horus.jobs (
    id                  uuid PRIMARY KEY, -- UUIDv7
    tool                text,
    workflow            text,
    backend             text,
    work_queue          text,
    resource_key        text,
    payload             jsonb,
    request_hash        text,
    idempotency_key     text,
    state               text NOT NULL CHECK (state IN ('queued', 'running', 'succeeded', 'failed', 'cancelled', 'expired', 'abandoned')),
    attempt             integer NOT NULL DEFAULT 0,
    created_at          timestamptz NOT NULL,
    deadline            timestamptz,
    waiter_seen_at      timestamptz,
    started_at          timestamptz,
    finished_at         timestamptz,
    worker_id           text,
    backend_job_id      text,
    cancel_requested_at timestamptz,
    progress            jsonb,
    result              jsonb,
    error               jsonb,
    caller              text
);

-- The dedupe of ADR 0027 reads the newest job with a key.
CREATE INDEX jobs_idempotency_key ON horus.jobs (idempotency_key, created_at);
-- The queue: the place of a job, and the queued jobs oldest first.
CREATE INDEX jobs_queued ON horus.jobs (created_at, id) WHERE state = 'queued';
-- The recent jobs and the counts of today.
CREATE INDEX jobs_finished ON horus.jobs (finished_at);

-- The events of each job, in order. The job store writes a row in the
-- transaction of each change of a job.
CREATE TABLE horus.job_events (
    id     bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    job_id uuid NOT NULL REFERENCES horus.jobs (id) ON DELETE CASCADE,
    at     timestamptz NOT NULL,
    type   text NOT NULL,
    data   jsonb
);

CREATE INDEX job_events_job ON horus.job_events (job_id, id);

-- The worker and its heartbeat.
CREATE TABLE horus.workers (
    id                  text PRIMARY KEY,
    host                text,
    pid                 integer,
    version             text,
    slots               integer,
    started_at          timestamptz,
    heartbeat_at        timestamptz,
    loaded_resource_key text
);

-- The messages of the Postgres broker only. A consumer hides a message that
-- it holds until visible_at. Each delivery adds 1 to deliveries, and the value
-- identifies the delivery in an ack, a nack and an extension.
CREATE TABLE horus.messages (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    queue      text NOT NULL,
    body       jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    visible_at timestamptz NOT NULL DEFAULT now(),
    deliveries integer NOT NULL DEFAULT 0
);

CREATE INDEX messages_queue ON horus.messages (queue, id);

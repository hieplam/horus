# 0027. The idempotency key of a job, and the idempotent consumer

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the owner, in the planning session of epic #180

## Context

The owner: think about idempotency now, and design the key and its recipe
now, so that a later use of the key changes no design. Do not apply it yet.

A job can run two times for three causes:

- the same request comes two times, for example a call again after an error;
- the same message comes two times, because every broker is at-least-once;
- the same job runs two times, after an automatic retry.

## Decision

- Each job row holds `request_hash` (the request fingerprint) and
  `idempotency_key` from the first version.
- The fingerprint is the SHA-256 of a JSON text with no spaces and with the
  keys in this order: `v` (1), `tool`, `workflow`, `workflow_rev` (the SHA-256
  of `manifest.json`, one 0 byte and `workflow.json`), `subject` (its words
  joined by one space), `seed` (decimal text, or null when the caller gave
  none), `width` and `height` (after the defaults), `out_path` (the cleaned
  host path, in lower case on Windows and macOS, or null) and `overwrite`.
- The key is `"k1:" + SHA-256("generate_image\n" + the key of the caller)`
  when the caller gives `idempotency_key`. Else it is `"f1:" + fingerprint`
  when the caller gives a seed. Else it is null, because the caller asked for
  a new random image.
- The dedupe is off. Two equal calls make two jobs.
- The worker is an idempotent consumer now: it runs a delivery only when the
  job store moves the job from `queued` to `running`. A second delivery then
  does nothing.
- The worker saves `backend_job_id` before it waits, and the API process fixes
  the output path at insert time, for the later retry.
- Go tests hold 12 fixed vectors of the recipe. The Go code is the source of
  the values.

## Consequences

- Turning on the dedupe later adds one step in the insert transaction (an
  advisory lock on the hash of the key, then a read of the newest job with
  the key), the error code `IDEMPOTENCY_KEY_REUSED` and the result field
  `deduplicated`. It changes no column and no recipe.
- A new recipe gets a new `v` and new prefixes, so an old key never matches.
- The Horus UI shows the count of jobs that share a key: the measurement for
  the decision to turn on the dedupe.
- To reverse this decision, drop the two columns. Nothing reads them yet.

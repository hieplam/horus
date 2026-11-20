# 0038. The Horus UI renders on the server, reads the depth of the broker, and checks the Host with the published port

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the agent of task #205, by the standing rules of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md), the
section "The Horus UI".
[ADR 0028](0028-the-horus-ui-is-a-read-only-page-of-horus-api.md): the page is
read-only, it has the 11 parts of the reference, it refuses a `Host` header
that is not `127.0.0.1:<port>` or `localhost:<port>`, and it has no build step.

- The reference shows the count of the messages of the broker ("work queue
  jobs.comfyui · 3 messages"). `jobs.Broker` had no method that counts
  messages. A count from the job store is not the count of the broker.
- The panel "workflows" shows, for each workflow, its jobs of today.
  `jobdata.Summary` had only the counts of all workflows.
- In the stack, `horus api` listened on 8190 in the container, and
  `compose.yaml` published it on `HORUS_API_PORT` of the host. A browser sends
  the port of the host in the `Host` header. With `HORUS_API_PORT=18190`, the
  container did not know that port, so a check of the `Host` refused each
  request of the page. `/mcp` (task #204) needs the same check.
- A page that renders in the browser from the JSON needs a second renderer,
  and a headless screenshot can show the page before the data.

## Decision

- `horus api` renders the page on the server with `html/template`, from one
  view (`webui.Build`, a pure function). `/api/summary` gives the same view as
  JSON. A small script reads the page again at each event of `/api/events`
  and every 5 s.
- `jobs.Broker` has the method `Depth(ctx, queue)`: the count of the messages
  of the queue that wait for a delivery. A message that a consumer holds
  does not count. The broker contract has the check `depth`. The Postgres
  broker counts the visible rows of `horus.messages`. A RabbitMQ broker gives
  `message_count` of a passive `queue.declare`.
- `jobdata.Summary` has `WorkflowToday`: for each workflow, the count of its
  jobs that ended as `succeeded` or `failed` since 00:00. The store contract
  checks it.
- The check of the `Host` header accepts the port of `HORUS_API_PORT` and the
  port of `-listen`. In the stack, `horus api` listens on `HORUS_API_PORT` in
  the container, and `compose.yaml` publishes the same port on 127.0.0.1 of
  the host.
- `/readyz` and `/livez` answer to any `Host`: the health checks of the
  containers ask them.

## Consequences

- A test reads the page without a browser: the HTML has the numbers.
- A new broker implements `Depth` and passes the check `depth`.
- `/mcp` (task #204) checks the `Host` header with the same ports
  (`webui.HostAllowed`).
- A change of the port of the stack is one setting, `HORUS_API_PORT`, for the
  host and the container.
- To reverse this decision, render the page in the browser from
  `/api/summary`, remove `Depth` from `jobs.Broker`, and publish a fixed port
  of the container again.

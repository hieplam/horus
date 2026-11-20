# 0043. The Horus UI shows the clock times of the time zone of the browser, counts today from its 00:00, and shows host paths

- **Status:** Accepted
- **Date:** 2026-11-21
- **Deciders:** the agent of the issues #264 and #265, by the standing rules of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md), the
section "The Horus UI", and the goal row `ui-matches-mockup`.
[ADR 0038](0038-the-horus-ui-renders-on-the-server-and-checks-the-published-port.md):
`horus api` renders the page on the server.
[ADR 0039](0039-generate-image-in-the-stack-uses-shared-folders-of-host-paths.md):
a job row holds the container path of its PNG, and the result of
`generate_image` gives the host path.

The agent acceptance on the PC of the owner (UTC+7) found two defects:

- The containers of the stack run at UTC. The page formatted each clock time
  in the time zone of the process `horus api`, and the job store started
  "today" at 00:00 of that zone. The page showed `19:37` for a job that ended
  at 02:37 on the clock of the PC, with no zone name. The counts of today
  started at 07:00 of the PC.
- The panel "recent" showed the container path of a PNG, for example
  `/shared/acceptance/a.png`. That path does not exist on the PC.

Two designs give the right clock times:

- A setting of the stack, for example `TZ` in the `.env`. The owner must add
  it. `docker compose up -d` without it shows UTC times. A wrong value also
  shows wrong times, and the page cannot tell.
- The browser gives its time zone. The browser always knows the zone of the
  person who looks at the page. No setting can be missing or wrong.

The code that changes a container path to a host path was in
`internal/mcpserver`. `internal/webui` is a handler too, and a handler does
not import another handler.

## Decision

- The page and `/api/summary` take the query parameter `tz`: the IANA name of
  the time zone of the browser, for example `Asia/Ho_Chi_Minh`. A script in
  the head of the page reads the name from the browser and loads the page
  with it, before the page shows.
- With a time zone that horus knows, `horus api` formats each clock time in
  that zone, and "today" starts at 00:00 of that zone. The tiles "succeeded
  today" and "failed today", the column "Today" and the row "Same key today"
  use the same start.
- Without the parameter, or with a name that horus does not know, `horus api`
  uses the zone of its process, and it puts the name of that zone after each
  clock time: `19:37 UTC`. The page never shows a clock time of another zone
  without its name.
- The stack has no time zone setting.
- The handler computes the start of today and gives it to the job store:
  `Store.Summary(ctx, today)`. The zero time means 00:00 of the day of the
  clock of the store, for a caller that reads no count of today. The store
  contract checks a start that is not 00:00 of the clock.
- The program holds the time zone data (`time/tzdata`), so that it knows the
  zone of a browser on each machine.
- `/api/summary` gives `zone`, `zone_from` (`browser` or `horus api`) and
  `today_from`.
- The page and `/api/summary` show the host path of a PNG. They use the
  mapping that `generate_image` uses: `jobs.Paths`, in the service
  `internal/jobs`. A path in no shared folder stays as it is.

## Consequences

- The owner starts the stack with `docker compose up -d` and sees the times
  of the PC, with no new step.
- Two people in two time zones see two different counts of today for the
  same job store. Each count is right for its reader.
- A client of `/api/summary` that gives no `tz` gets the counts of the day of
  the process, and `zone_from` says so.
- A new clock time on the page uses `when` with the zone of the request. A
  new count of today uses `today_from`.
- A new caller that needs a host path uses `jobs.Paths`. No second mapping.
- The program file grows by about 450 kB (the time zone data).
- To reverse this decision, remove the parameter `tz` and the script, give
  the containers a `TZ` setting in `compose.yaml`, and add the step to
  `docs/setup.md`.

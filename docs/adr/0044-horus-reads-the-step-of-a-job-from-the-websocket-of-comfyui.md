# 0044. Read the step of a running job from the WebSocket of ComfyUI, and write it to the job row

- **Status:** Accepted
- **Date:** 2026-11-21
- **Deciders:** the agent of the issue #266, by the standing rules of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md), the
section "The Horus UI", and the goal row `ui-matches-mockup`. The reference of
the page, [`2026-11-19-job-queue-ui.html`](../specs/2026-11-19-job-queue-ui.html),
shows `step 9 of 15` and a bar at 60 % in the panel "Running". The owner
ratified the reference.
[ADR 0004](0004-direct-rest-and-separate-processes.md): horus calls the
ComfyUI REST API directly.

The agent acceptance on the PC of the owner found that, with a real ComfyUI
(0.44.0), the panel "Running" showed no step and an empty bar for the whole
run. The cause:

- horus read the state of a job with `GET /history/<id>` and `GET /queue`.
  The progress of a run was the text `rendering, N s`, with a total of 0.
- ComfyUI 0.44.0 gives the step of a node only on its WebSocket
  `/ws?clientId=<id>`. A message of the type `progress` holds `value`, `max`,
  `prompt_id` and `node`. ComfyUI sends it only to the client id of the job.
  No REST route gives it: `GET /api/jobs/<id>`, `GET /history/<id>` and
  `GET /queue` have no step. Measured on the PC, 2026-11-21, with the workflow
  `it-illustration` and 8 steps: 8 `progress` messages of the `KSampler` node,
  one each 9 s to 13 s, and no `progress` message of another node.
- The tests did not find the defect. The fixture of the page wrote a step and
  a total to the job row itself, and no test ran a worker with a ComfyUI that
  reports a step.

Two designs make the panel true:

- horus reads the step from the WebSocket, and the worker writes it to the
  job row.
- The bar shows the run time as a part of the run estimate, and the reference
  and the docs change to that.

The second design changes an artifact that the owner ratified, and its bar is
an estimate: it is wrong for the first job after a start (the model load) and
for a workflow with another step count.

The Go standard library has no WebSocket client.

## Decision

- horus reads the step of a running job from the WebSocket of ComfyUI. The
  reference and the docs stay as the owner ratified them.
- Only the adapter `internal/comfy` talks to the WebSocket. `Submit` opens one
  WebSocket with the client id of the job, before it sends the graph. The
  adapter keeps the last `progress` message of each node of the job. It closes
  the WebSocket when ComfyUI sends the end of the job
  (`execution_success`, `execution_error`, `execution_interrupted`), when the
  context of `Submit` ends, at `Cancel`, and when `History` reports the end of
  the job.
- The service `internal/generate` declares what it needs in its interface
  `ComfyUI`: `Step(jobID, nodeID) (value, total, ok)`. It does no input and no
  output. The service asks for the step of the `KSampler` node of the graph
  at each poll of the job (every 2 s). A workflow without a `KSampler` node
  has no step.
- A new step makes a report at once. The text of the report names the step:
  `rendering, step 9 of 15, 42 s`. The progress value of a report stays the
  time of the run in seconds, with a total of 0, because the progress value of
  an MCP notification must only rise.
- The step goes to the caller of the service in `generate.Options.Step`. The
  worker writes it to the progress of the job row: `value` 9, `total` 15, and
  the text of the report. Without a step the total is 0.
- The Horus UI shows `step 9 of 15` and a bar only for a job row with a total
  above 0. Else it shows the text of the run and an empty bar, and the hover
  text of the bar says that ComfyUI reports no step.
- A WebSocket never fails a job. When ComfyUI refuses it, the job runs
  without a step. When it ends while the job runs, the adapter drops the
  steps of the job, because a step that no longer changes is not the step of
  the job. horus does not open the WebSocket of a job again.
- The WebSocket client is the Go module `github.com/coder/websocket`
  (v1.8.15, ISC license, no dependency of its own). The fake ComfyUI of the
  tests (`internal/comfy/comfytest`) uses it for the server side.
- The fake ComfyUI gives a step only on its WebSocket, as ComfyUI does. A test
  of the step runs a worker and reads the page two times
  (`TestRunningStep` of `cmd/horus`), and the goal row `ui-matches-mockup`
  runs it.

## Consequences

- The panel "Running" and the progress text of `generate_image`, on the
  direct path and in the stack, show the step that ComfyUI reports.
- While ComfyUI loads the models (20 s before step 1 in the measured run, more when the model files are not in memory),
  the job has no step, and the bar is empty.
- The worker writes one more progress row for each step. A run of 15 steps
  adds 15 writes to the job store.
- One job holds one more connection to ComfyUI while it runs.
- `comfy.Client` holds state now: the open WebSockets, by job id. A caller
  that submits a job must use the same client for `Step`, `History` and
  `Cancel` of that job. Do not copy a `comfy.Client`.
- A new adapter of `generate.ComfyUI` (a fake, another backend) must give
  `Step`. An adapter that cannot read a step gives `ok` false.
- A change of the WebSocket messages of a later ComfyUI makes the bar empty,
  and no job fails. The file `internal/comfy/steps.go` holds the whole wire
  format. Read the messages from a live ComfyUI before you change it.
- horus has a third direct Go module. A later WebSocket need (previews, the
  node that runs) uses the same module.
- To reverse this decision, remove `steps.go`, the method `Step` and
  `Options.Step`, remove the module, and change the reference and
  `docs/mcp.md` to a bar that the owner ratifies again.

# Roadmap

horus starts as a repo that an agent reads. The goal is a tool that any agent
can call when it needs an image.

## Now: v0, the starter kit

- Flow 1: [a fresh Windows PC or Mac to a working setup](docs/setup.md).
  On macOS, `horus` installs, starts and stops ComfyUI through comfy-cli
  (Phase 3 of [issue #4](https://github.com/hieplam/horus/issues/4)). The real
  run of the release on a Mac is [issue #168](https://github.com/hieplam/horus/issues/168).
- Flow 2: [a style description to a tested workflow](docs/style-to-workflow.md).
- One example workflow: [`it-illustration`](workflows/it-illustration/README.md).
- **A release and one install line** (Phase 4 of [issue #4](https://github.com/hieplam/horus/issues/4)).
  The first release is 0.1.0 (2026-11-16). Each release has the `horus` program for Windows and macOS and a list of
  SHA-256 hashes. One line installs it and puts it on the PATH. The program holds
  the default workflows, so a user needs no clone. `horus setup <workflow> -yes`
  does every step of the setup, and it installs a system package only with
  `-with-packages`. `horus register -client codex` adds the server to Codex.
  `horus uninstall` removes what horus installed. See the [README](README.md#install-horus).

## Next

- **More example workflows.** Each one has a `models.json`, a verified run and
  example images. Candidates: a photo-style workflow and an image-to-image edit.
- **Linux setup.** `horus` builds and runs on Linux, but
  `horus install-comfyui`, `horus start-comfyui` and `horus stop-comfyui` answer
  "not supported on this OS yet" there.
- **Video.** Short text-to-video and image-to-video clips on consumer GPUs.

## Now: v0.1, horus as an MCP server

An agent that needs an image does not read docs. It calls a tool.
[`docs/mcp.md`](docs/mcp.md) describes the server.

- `list_workflows` and `describe_workflow` show the styles, what each is good
  for, its size limits and how to write the subject.
- `install_workflow` and `uninstall_workflow` manage the model files, with a
  license confirmation and a delete confirmation. On a machine without ComfyUI,
  `install_workflow` also runs the setup, after a setup confirmation.
- `generate_image(workflow, subject, ...)` waits until the PNG is on disk.
- Every result is one envelope with typed errors (`code`, `retryable`,
  `action_by`, `next_step`).

## Next: the job queue

[Epic #180](https://github.com/hieplam/horus/issues/180), spec
[`2026-11-19-job-queue.md`](docs/specs/2026-11-19-job-queue.md). horus runs as
a Docker stack of 3 containers next to the ComfyUI of the host: `horus-api`,
`horus-worker` and `horus-db`. All agents share one queue and one worker, so
ComfyUI never gets more jobs than the worker has slots. horus serves only when
its 4 dependencies are up. Agents connect over stdio or over HTTP. A read-only
page, the Horus UI, shows the queue.

## Later

- **MCP tasks for long calls.** Claude Code moves a long MCP call to the
  background after 2 minutes, so a direct call works today. Claude Code also
  supports MCP tasks (protocol revision 2026-07-28): the server returns a task
  id at once and the client polls it. That removes the 30-minute idle limit for
  long installs. Not built: it needs an SDK and harnesses that speak that
  revision.
- **`add_style(description)`**: run Flow 2 from the server and save the new workflow.

Ideas and pull requests are welcome.

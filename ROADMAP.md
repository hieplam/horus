# Roadmap

horus starts as a repo that an agent reads. The goal is a tool that any agent
can call when it needs an image.

## Now: v0, the starter kit

- Flow 1: [a fresh Windows PC to a working setup](docs/setup.md).
- Flow 2: [a style description to a tested workflow](docs/style-to-workflow.md).
- One example workflow: [`it-illustration`](workflows/it-illustration/README.md).

## Next

- **More example workflows.** Each one has a `models.json`, a verified run and
  example images. Candidates: a photo-style workflow and an image-to-image edit.
- **Linux and macOS setup.** The scripts are PowerShell only today.
- **Video.** Short text-to-video and image-to-video clips on consumer GPUs.

## Now: v0.1, horus as an MCP server

An agent that needs an image does not read docs. It calls a tool.
[`docs/mcp.md`](docs/mcp.md) describes the server.

- `list_workflows` and `describe_workflow` show the styles, what each is good
  for, its size limits and how to write the subject.
- `install_workflow` and `uninstall_workflow` manage the model files, with a
  license confirmation and a delete confirmation.
- `generate_image(workflow, subject, ...)` waits until the PNG is on disk.
- Every result is one envelope with typed errors (`code`, `retryable`,
  `action_by`, `next_step`).

## Later

- **MCP tasks for long calls.** Claude Code moves a long MCP call to the
  background after 2 minutes, so a direct call works today. Claude Code also
  supports MCP tasks (protocol revision 2026-07-28): the server returns a task
  id at once and the client polls it. That removes the 30-minute idle limit for
  long installs. Not built: it needs an SDK and harnesses that speak that
  revision.
- **`add_style(description)`**: run Flow 2 from the server and save the new workflow.
- **A queue view**: which jobs of other agents wait on the same GPU.

Ideas and pull requests are welcome.

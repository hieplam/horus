# 0023. A workflow has a kind, and each kind has its own tool on the one server

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the owner, in the planning session of epic #181

## Context

The view of the owner: horus is the one program in front of all local AI
models of a machine, images today and speech next. An agent must not learn a
second server for a second kind of output.

- Until now each workflow made an image, and the server had the tool
  `generate_image`.
- An image call has a subject, a size and a seed. A speech call has a text and
  a voice. One tool for both needs arguments that are wrong for one of them.
- `list_workflows`, `describe_workflow`, `install_workflow` and
  `uninstall_workflow` do the same job for each kind.

## Decision

- One server, `horus mcp`, serves each kind of output.
- `manifest.json` has the field `kind`: `image` or `speech`. A manifest
  without it is of the kind `image`.
- Each kind has one tool that makes its output: `generate_image` for `image`,
  `generate_speech` for `speech`.
- The four other tools serve each kind.
- A tool that gets a workflow of the other kind answers `INVALID_ARGUMENT`,
  and its `next_step` names the right tool.
- The arguments of a tool name only what the caller owns: the text and the
  voice for speech. They name no engine, no Python, no ffmpeg and no loudness.
- A speech workflow sends the text to the model as written. It adds no words
  and removes no words.

## Consequences

- `horus register` does not change: an agent that has horus has speech.
- A third kind, for example video, adds one value of `kind` and one tool.
- Code that reads a manifest must read `kind` before it reads `graph` or
  `subject`, because a speech manifest has neither.
- To reverse this decision, move the speech tool to a second server. Each
  agent then needs a second registration.

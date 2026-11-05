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

## Later: horus as an MCP server

An agent that needs an image will not read docs. It calls a tool.

- `horus.generate(style, subject, size, seed)` returns an image path.
- `horus.styles()` lists the workflows that are installed and verified.
- `horus.add_style(description)` runs Flow 2 and saves the new workflow.

The server wraps the same workflows and scripts that are in this repo. The
`generate()` function in [`scripts/run-illustration.py`](scripts/run-illustration.py)
is the first piece of that interface.

Ideas and pull requests are welcome.

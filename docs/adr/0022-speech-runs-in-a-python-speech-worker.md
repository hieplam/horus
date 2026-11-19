# 0022. Speech runs in a Python speech worker that `horus mcp` starts and keeps

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the owner, in the planning session of epic #181

## Context

horus makes an image through ComfyUI, and the `horus` program needs no Python
for that ([ADR 0001](0001-go-one-binary-per-platform.md)). The owner wants
horus to make speech too
([the spec](../specs/2026-11-20-text-to-speech.md)).

- The speech model VieNeu-TTS v3 Turbo runs only through the Python package
  `vieneu`. It has no ComfyUI node and no Go library.
- A new Python process loads the model in 15 to 20 s on the PC (Ryzen 7
  4800H). A 5 s sentence then takes about 21 s in total.
- The test repo `my-tss` sets the loudness and the file format with ffmpeg.
- The job queue of issue #180 will give all agent sessions one worker. The
  owner did not ratify it yet, and the owner decided: speech first.

## Decision

- Speech runs in a speech worker: one Python process that holds the models in
  memory.
- The `horus` program holds the Python file of the speech worker. horus starts
  it with the Python of a venv that horus makes.
- `horus mcp` starts the speech worker at the first `generate_speech` call and
  keeps it until `horus mcp` ends.
- The two processes talk through the standard input and output of the worker,
  one JSON line for each message.
- A speech workflow needs Python 3.12 and ffmpeg on the machine. The setup
  plan installs them as system package steps, under the setup consent.
- An image workflow still needs no Python.
- `internal/download` downloads the model files and checks each SHA-256. The
  speech worker gives the model the local folders, so no Python package
  downloads a model.

## Consequences

- A second speech call of the same session does not load the models again.
- Each `horus mcp` process has its own speech worker. Three sessions hold
  three copies of the models in memory. Issue #180 removes this.
- The repo has one Python file that is part of the product. A change to it
  needs a test with the real models, because the Go tests use a fake worker.
- The pinned versions of the Python packages are a new thing to update.
- To reverse this decision, remove the tool `generate_speech`, the speech
  worker and the speech steps of the setup plan. An image workflow does not
  change.

# Spec: text to speech through the horus MCP server

Date: 2026-11-20. The epic: [issue #181](https://github.com/hieplam/horus/issues/181).
The decision records: [ADR 0022](../adr/0022-speech-runs-in-a-python-speech-worker.md),
[ADR 0023](../adr/0023-a-workflow-has-a-kind-and-each-kind-has-its-tool.md) and
[ADR 0024](../adr/0024-generate-speech-returns-a-transcript.md).

## What this work delivers

An agent calls the tool `generate_speech` of the horus MCP server with a text
and a voice name. It gets one audio file on this machine, and the transcript
of that file. The agent does not know the speech model, the Python packages,
ffmpeg or the loudness step. horus hides them, as it hides ComfyUI for an
image.

The source is the test repo `my-tss` of the owner. It speaks one text into
one file with VieNeu on the CPU.

## Terms

- **TTS (text to speech):** a model reads a text and writes an audio file.
- **VieNeu:** the TTS model VieNeu-TTS v3 Turbo (Apache-2.0). It speaks
  Vietnamese, and English words in a Vietnamese text. It runs on the CPU
  through the Python package `vieneu`.
- **Speech recognition model:** a model that reads an audio file and writes
  its words. It is the reverse of TTS.
- **Transcript:** the words that the speech recognition model reads from the
  audio file that `generate_speech` made.
- **Speech worker:** the Python process that holds VieNeu and the speech
  recognition model in memory. `horus mcp` starts it.
- **Speech venv:** the Python virtual environment that holds the packages of
  the speech worker. horus makes it.
- **Kind:** what a workflow makes: `image` or `speech`.
- **LUFS (Loudness Units relative to Full Scale):** the unit of the loudness
  of an audio file.
- **Preset voice:** a voice that the VieNeu model files hold. There are 25.
- **Share of wrong words:** the words of the text that the transcript does
  not have, divided by the words of the text. Both texts are in lower case
  and without punctuation.

## Ratified goal (owner, 2026-11-20)

| Row | Outcome | Oracle | Threshold | Ratchet |
|---|---|---|---|---|
| `speech-real` | `generate_speech` with a Vietnamese sentence returns an audio file and a transcript with the words of the text. | End-to-end run on the PC with the real models: 5 fixed sentences through the MCP server. | Each file exists. Its loudness is -18 LUFS ± 0.5. The share of wrong words of each sentence is at most the limit that the baseline task measures and writes into this spec. | Speech files that an agent makes through horus: 0 → 5 |
| `speech-hidden` | The caller gives only a text and a voice name. | The contract test reads the schema of `generate_speech`. | 0 arguments that name an engine, Python, ffmpeg or the loudness. | Arguments that show the speech setup: 0, and it stays 0 |
| `speech-install` | On a machine with no speech setup, `install_workflow` with both consents makes `generate_speech` work. The uninstall removes what horus installed. | End-to-end run in a sandbox home folder: install, one speech file, `uninstall_workflow`, then `horus uninstall -yes`. | Before both consents: 0 changes on disk. After the uninstall: 0 files of the speech setup in the sandbox. | Steps that a person does by hand to set up speech: 3 (venv, `pip install`, ffmpeg) → 0 |
| `speech-errors` | Each failure of a speech call gives an error code with `action_by` and `next_step`. | The contract test with a fake speech worker. | One check for each failure of the table "Failures" below. Each check passes. | Speech failures with a check: 0 → 8 |
| `speech-contract` | The checks of the image tools still pass. | `tests/mcp_contract.py`. | 37 of 37, plus the speech checks. All pass. | Contract checks: 37 → 37 plus the speech checks |
| `speech-macos` | The program builds on macOS, and the contract test passes there. | The CI run on `main`. | The macOS jobs are green. | CI jobs on macOS that run a speech check: 0 → 1 |

**Done** means: all 6 rows pass from
`go test -tags goal -v ./goals/2026-11-20-text-to-speech` on the PC, and an
agent that has only the horus MCP server makes 5 speech files and checks each
file by its transcript.

After this ratification, the owner is not asked to verify or decide anything.
There is one exception: the owner approves the speech recognition model before
its first download ([rule 9 of `AGENTS.md`](../../AGENTS.md#rules)). A finding
becomes an issue that blocks the task that it threatens.

## The design

### One server, one new tool

The server stays `horus mcp`. It gets one new tool:

```
generate_speech(workflow, text, voice?, out_path?, overwrite=false)
```

| Argument | What it is |
|---|---|
| `workflow` | A workflow of the kind `speech` from `list_workflows`. |
| `text` | The text to speak. horus sends it to the model as written. |
| `voice` | A voice name from `describe_workflow`. Default: the default voice of the workflow. |
| `out_path` | An absolute path that ends with `.wav` or `.mp3`. Default: a new `.wav` file in the horus output folder. |
| `overwrite` | Replace `out_path` when it exists. |

The result:

| Field | What it is |
|---|---|
| `workflow`, `voice` | What the call used. |
| `path`, `bytes`, `format` | The audio file. `.wav` is 48 kHz, 16-bit, mono. `.mp3` is 44.1 kHz, 128 kbps, mono. |
| `audio_seconds` | The length of the audio. |
| `transcript` | The words that the speech recognition model read from the file. |
| `seconds` | The wall time of the call. |
| `started_worker` | True: this call had to start the speech worker first. |

- The loudness of each file is -18 LUFS. The caller cannot change it.
- horus does not compare the transcript with the text. The caller compares
  them and decides.
- The tool sends a progress notification every 10 s.
- One speech worker makes one file at a time. A second call waits.
- `HORUS_GENERATE_TIMEOUT` also limits a speech call.

### The kind of a workflow

- `manifest.json` gets the field `kind`: `image` or `speech`. A manifest
  without it is of the kind `image`.
- `list_workflows` shows the kind of each workflow.
- `describe_workflow` of a speech workflow shows the voices, the default
  voice, the tags that the model performs, the limits of the text, the
  formats, the license and the state of each model file. It shows no graph.
- `generate_image` with a speech workflow, and `generate_speech` with an image
  workflow, answer `INVALID_ARGUMENT`. The `next_step` names the right tool.
- A speech manifest has `text` where an image manifest has `subject`, and it
  has no `graph` and no `output` size.

The first speech workflow is the folder `workflows/vi-speech/`: `manifest.json`,
`models.json` (the byte size and the SHA-256 of each model file) and
`README.md`.

### The text

- The text is the whole input. horus adds no words and removes no words.
- VieNeu performs 3 tags: `[cười]`, `[thở dài]` and `[hắng giọng]`. It reads
  another `[tag]` aloud. So horus refuses a text with another tag:
  `INVALID_ARGUMENT`, and `details.allowed_tags` lists the 3 tags.
- The manifest gives `text.min_chars` and `text.max_chars`. The baseline task
  measures the longest text that works and sets `max_chars`.

### The speech worker

- The speech worker is one Python file that the `horus` program holds. horus
  writes it to the user folder and starts it with the Python of the speech
  venv.
- `horus mcp` starts the speech worker at the first `generate_speech` call. It
  keeps the worker until `horus mcp` ends. Then the worker ends too.
- `horus mcp` and the speech worker talk through the standard input and
  output of the worker: one JSON line for each request and one for each
  answer.
- The worker loads VieNeu and the speech recognition model one time. It gives
  VieNeu the local model folders (`onnx_dir` and `codec_dir`), so the `vieneu`
  package downloads nothing.
- The worker runs ffmpeg for the loudness and the file format.
- Each `horus mcp` process has its own speech worker. Three agent sessions
  hold three copies of the models. The job queue of
  [issue #180](https://github.com/hieplam/horus/issues/180) later gives all
  sessions one worker.

In the three layers: the handler `internal/mcpserver` calls the new service
`internal/speak`. The service declares the interface of the speech worker. A
new adapter starts the Python process and implements that interface. A test
gives the service a fake.

### Install and uninstall

`install_workflow` of a speech workflow runs a setup plan with the two
consents of [ADR 0011](../adr/0011-setup-through-mcp-needs-two-consents.md).
The steps, in order:

1. A system package step for Python 3.12, only when the machine has none
   (`winget` on Windows, `brew` on macOS).
2. A system package step for ffmpeg, only when `ffmpeg` and `ffprobe` are not
   on the PATH.
3. The speech venv, with the pinned versions of the Python packages. horus
   writes its install record (`horus-install.json`) there.
4. The model files. `internal/download` downloads each file and checks its
   byte size and its SHA-256.

- `horus setup vi-speech` prints the same plan, and `-yes` runs it. A system
  package step runs only with `-with-packages`.
- `horus doctor -workflow vi-speech` checks Python, ffmpeg, the speech venv and
  the model files.
- `uninstall_workflow` deletes the model files of the workflow.
- `horus uninstall -yes` also deletes the speech venv, because it has the
  install record. It does not remove a system package.
- A speech workflow needs no ComfyUI. Its plan has no ComfyUI step.

### The models

| Model | Job | License | Size |
|---|---|---|---|
| `pnnbao-ump/VieNeu-TTS-v3-Turbo` | Makes the speech | Apache-2.0 | 494 MB in the Hugging Face cache of the PC |
| `OpenMOSS-Team/MOSS-Audio-Tokenizer-Nano-ONNX` | Turns the output of VieNeu into audio | Apache-2.0 | 86 MB |
| `Systran/faster-whisper-small` (candidate) | Reads the transcript | MIT | 484 MB |

The speech recognition model is a candidate. The baseline task measures it on
the PC, and the owner approves it before its first download. If the owner
refuses it, the baseline task names the next candidate.

### Failures

| Failure | Code | Action by |
|---|---|---|
| The text is empty, too short or too long | `INVALID_ARGUMENT` | caller |
| The text has a tag that VieNeu does not perform | `INVALID_ARGUMENT` | caller |
| The voice is not a voice of the workflow (`details.known` lists the voices) | `INVALID_ARGUMENT` | caller |
| `out_path` is not absolute, or does not end with `.wav` or `.mp3` | `INVALID_ARGUMENT` | caller |
| The workflow is of the kind `image` | `INVALID_ARGUMENT` | caller |
| `out_path` exists and `overwrite` is false | `OUTPUT_EXISTS` | caller |
| The speech venv, ffmpeg or a model file is missing | `WORKFLOW_NOT_INSTALLED` | caller |
| The speech worker stops or reports an error while it makes the file | `GENERATION_FAILED` | retry |

The work adds no error code.

### Docs

Each task that changes behaviour changes its docs in the same pull request:
`docs/mcp.md` (the tool, the diagrams, the settings), `docs/setup.md`,
`docs/models.md`, `docs/concepts.md`, `AGENTS.md`, `README.md`, `ROADMAP.md`
and `workflows/vi-speech/README.md`.

## Scope

- In: the tool `generate_speech`, the kind of a workflow, the workflow
  `vi-speech` with the 25 preset voices, the speech worker, the transcript,
  the install, the uninstall, the doctor checks, the docs, the goal suite.
- Out, each one a follow-up outside the epic:
  - a voice from a clip of the user (a voice folder);
  - one speech worker for all agent sessions (issue #180);
  - a second language or a second TTS model;
  - speech on the GPU;
  - the real run on the Mac ([ADR 0018](../adr/0018-real-runs-on-the-owners-machines.md)).

## Standing rules

- A flaky test gets a fix of its cause. No retry, no longer wait, no weaker
  check.
- Never weaken or delete a test to make a check pass.
- When a command of a task no longer fits the tree, use the command that
  proves the same goal row, and record why in the pull request.
- A finding outside the goal rows becomes a follow-up issue outside the epic.
- Never run an uninstall with the real home folder. Use a sandbox.
- Run only this suite: `go test -tags goal -v ./goals/2026-11-20-text-to-speech`.

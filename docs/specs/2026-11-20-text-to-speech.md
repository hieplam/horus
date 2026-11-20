# Spec: text to speech through the horus MCP server

Date: 2026-11-20. The epic: [issue #181](https://github.com/hieplam/horus/issues/181).
The decision records: [ADR 0022](../adr/0022-speech-runs-in-a-python-speech-worker.md),
[ADR 0023](../adr/0023-a-workflow-has-a-kind-and-each-kind-has-its-tool.md) and
[ADR 0024](../adr/0024-generate-speech-returns-a-transcript.md).

> **Record of a finished phase.** All 6 goal rows passed on 2026-11-20 on the
> PC ([pull request #233](https://github.com/hieplam/horus/pull/233)). On the
> same day, an agent that had only the horus MCP server made 5 speech files and
> checked each file by its transcript
> ([issue #193](https://github.com/hieplam/horus/issues/193)). The acceptance
> does not prove the tone or the quality of the voice: no person listened. The
> suite `goals/2026-11-20-text-to-speech/` is frozen: nobody edits it. A change
> to text to speech gets a new spec and a new suite.

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
- **Preset voice:** a voice that the Python package `vieneu` holds. There are 25.
- **Share of wrong words:** the words of the text that the transcript does
  not have, divided by the words of the text. Both texts are in lower case
  and without punctuation.

## Ratified goal (owner, 2026-11-20)

| Row | Outcome | Oracle | Threshold | Ratchet |
|---|---|---|---|---|
| `speech-real` | `generate_speech` with a Vietnamese sentence returns an audio file and a transcript with the words of the text. | End-to-end run on the PC with the real models: 5 fixed sentences through the MCP server. | Each file exists. Its loudness is -18 LUFS ± 0.5. The share of wrong words of each sentence is at most 0.344 (the section "Measured baseline"). | Speech files that an agent makes through horus: 0 → 5 |
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
  writes it to the speech folder ([ADR 0032](../adr/0032-the-speech-setup-is-one-folder-in-the-user-folder.md))
  and starts it with the Python of the speech
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
   writes its install record (`horus-install.json`) there right after the venv
   is made, and writes it again with the list of packages after the pip install
   ([ADR 0035](../adr/0035-the-install-record-of-the-speech-venv-lists-its-packages.md)).
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
| `pnnbao-ump/VieNeu-TTS-v3-Turbo` | Makes the speech | Apache-2.0 | 518,063,957 bytes, 9 files |
| `OpenMOSS-Team/MOSS-Audio-Tokenizer-Nano-ONNX` | Turns the output of VieNeu into audio | Apache-2.0 | 90,572,161 bytes, 6 files |
| `Systran/faster-whisper-small` | Reads the transcript | MIT | 486,212,372 bytes, 4 files |

- The owner approved the speech recognition model on 2026-11-20.
- [`workflows/vi-speech/models.json`](../../workflows/vi-speech/models.json)
  lists the 19 files. Each file names its own repo, its revision and its
  license, because the files come from 3 repos.
- The 25 preset voices are in the Python package `vieneu`, not in the model
  files.
- The class of `vieneu` 3.8.4 passes `onnx_dir` to its engine, and it does not
  pass `codec_dir`. The speech worker adds `codec_dir` to the call of the
  engine, as `tests/speech/baseline.py` does.

### Measured baseline

Measured on 2026-11-20 with `tests/speech/baseline.py` on the PC: AMD Ryzen 7
4800H, CPU only, Windows 11, `vieneu` 3.8.4, `faster-whisper` 1.2.1, the voice
`Hải Đăng`. The models came from a local folder, with `HF_HUB_OFFLINE=1` and
an empty Hugging Face cache.

| Fact | Value |
|---|---|
| Share of wrong words | The section "The spread of the share of wrong words" below |
| Limit of the row `speech-real` | 0.344: the worst share of 240 files (0.294) plus a margin of 0.05 |
| Loudness of the 10 files | -18.0 to -17.9 LUFS |
| Time of one call, a sentence of 3 to 6 s of audio | 5.2 to 8.2 s: about half to speak, half to read the transcript |
| Load time of both models, files in the disk cache of the OS | 4.1 to 6.5 s |
| Memory of one process with both models, short texts | 1,973 MB at the peak |
| Memory with a text of 3,999 characters | 5,255 MB at the peak |
| Longest text that works | 3,999 characters: 210 s of audio, a call of 272 s, share of wrong words 0.014 |

- `text.max_chars` of the workflow is 4,000.
- The speech model does not give the same audio for the same text two times.
  So the share of wrong words changes from run to run.

#### The spread of the share of wrong words

The spread is the values that the share of wrong words of one sentence takes
in different runs. 2 runs (10 files) are too few to set a limit from it. So the
spread was measured again on 2026-11-20, on the same PC, in two measurements:

- **The cause measurement.** A scratch script with the steps of the speech
  worker (the same models, the same loudness step) made 60 files: 12 runs of
  the 5 sentences. The speech recognition model then read the same 60 files
  with each setting of the table "Settings of the transcript step" below.
- **The limit measurement.** `tests/speech/baseline.py --runs 12` and then
  `--runs 24` made 180 other files: 36 runs of the 5 sentences. The baseline
  reads each file with the function `transcribe` of the speech worker, so it
  measures the transcript step of the product.

The share of wrong words with the transcript step of the product (0.5 s of
silence before the audio), from the 240 files of both measurements:

| Sentence | Runs | Median | Worst |
|---|---:|---:|---:|
| `plain` | 48 | 0.000 | 0.125 |
| `commas` | 48 | 0.088 | 0.294 |
| `english-words` | 48 | 0.000 | 0.154 |
| `tag` | 48 | 0.000 | 0.188 |
| `two-sentences` | 48 | 0.060 | 0.200 |

The same 60 files of the cause measurement, read with the transcript step
before the change (no silence):

| Sentence | Runs | Median | Worst |
|---|---:|---:|---:|
| `plain` | 12 | 0.000 | 0.062 |
| `commas` | 12 | 0.088 | 0.235 |
| `english-words` | 12 | 0.000 | 0.077 |
| `tag` | 12 | 0.062 | 0.188 |
| `two-sentences` | 12 | 0.080 | 0.240 |

The oracle of the row `speech-real` (`TestSpeechReal`), 13 runs through the MCP
server with a build that has this transcript step, 65 files:

| Sentence | Runs | Median | Worst |
|---|---:|---:|---:|
| `plain` | 13 | 0.062 | 0.125 |
| `commas` | 13 | 0.059 | 0.235 |
| `english-words` | 13 | 0.000 | 0.154 |
| `tag` | 13 | 0.000 | 0.188 |
| `two-sentences` | 13 | 0.080 | 0.200 |

- **The limit.** The worst share of the 240 files is 0.294. The limit of the
  row `speech-real` is 0.294 + 0.05 = 0.344. The 65 files of the oracle are not
  part of the baseline: they check the limit.
- **What a caller must expect.** The transcript of a right audio file can
  differ from the text. In 133 of the 240 files, the transcript had all the
  words of the text. The worst transcript did not have 5 of the 17 words
  of its text (0.294): `Mấy chú nhận yêu cầu, xử lý giữ liệu, rồi trà kết quả
  về trong người dùng.` Each of the 5 is a word with another tone or another
  letter. So a caller must expect up to about 3 words in 10 to
  differ, and the audio can be right. A share above 0.344 is outside the
  measured spread, so the caller treats that file as wrong. `describe_workflow`
  of `vi-speech` gives these numbers in `text.rules`.
- **A wrong file gives a much larger share.** The transcript of each file,
  compared with the text of one of the 4 other sentences, gives a share of
  0.846 or more. So the limit 0.344 still tells a right file from a wrong file.

What the cause measurement shows:

- **The same file gives the same transcript.** The speech recognition model
  read each file two times with the same setting. The two transcripts were the
  same for 60 of 60 files. So the spread comes from the speech model. VieNeu
  picks each part of its output at random (its sampling, temperature 0.8), and
  it gives a different audio file at each call.
- **The same few words fail.** Before the change, 6 words give 31 of the 62
  wrong words: `cho` read as `trong` (6 of 12 runs), `và` read as `vào` (6),
  `lỗi` read as `lối` (6), `nhờ` read as `nhiều` (5), `dữ` read as `giữ` (4)
  and a dropped `ổn` (4). Most of them are a word with another tone or another
  letter that sounds almost the same. In the Northern accent of the voice
  `Hải Đăng`, `d` and `gi` sound the same. Not verified: that the audio has the
  right word. A person did not listen.
- **The commas are not the cause.** In the sentence `commas`, 3 of its 21
  wrong words are next to a comma.
- **The start of the audio is not the cause.** Each file starts with 50 to
  140 ms of silence. The first word of a sentence is wrong in 3 of the 60 files.
  With 0.5 s of silence before the audio, it is also wrong in 3 of 60. In the
  first 60 files of the limit measurement, it is wrong in 9 files without the
  silence and in 8 with it.
- **No setting of the transcript step removes the spread.** The best setting
  lowers the mean share from 0.054 to 0.045.

Settings of the transcript step, each on the same 60 files of the cause
measurement. The last column counts the files with a better share and the
files with a worse share than the setting before.

| Setting | Mean share | Worst share | Files above 0.17 | Better, worse |
|---|---:|---:|---:|---:|
| Beam size 5, `condition_on_previous_text=False` (the setting before) | 0.054 | 0.240 | 5 | - |
| The same, a second reading | 0.054 | 0.240 | 5 | 0, 0 |
| 0.25 s of silence before the audio | 0.053 | 0.235 | 4 | 9, 9 |
| **0.5 s of silence before the audio (the setting now)** | **0.045** | **0.188** | **3** | **12, 4** |
| 1 s of silence before the audio | 0.047 | 0.235 | 4 | 11, 3 |
| 0.5 s of silence, `without_timestamps=True` | 0.045 | 0.188 | 3 | 13, 4 |
| `vad_filter=True` | 0.054 | 0.240 | 5 | 0, 0 |
| Beam size 10 | 0.052 | 0.235 | 4 | 2, 0 |
| `without_timestamps=True` | 0.050 | 0.235 | 4 | 5, 0 |
| A fixed `initial_prompt` with no word of the 5 sentences | 0.057 | 0.240 | 5 | 2, 3 |
| `compute_type=float32` (int8 before) | 0.048 | 0.240 | 2 | 7, 1 |
| 0.5 s of silence, `compute_type=float32` | 0.045 | 0.200 | 4 | 10, 2 |

The limit measurement confirms the setting. Its first 60 files (`--runs 12`),
read without the silence, give a mean share of 0.049. With the silence, they
give 0.042: the share is better in 11 files and worse in 6. Over these 60 files
and the 60 files of the cause measurement, the silence gives a better share in
23 files and a worse share in 10.

The speech worker reads each file with the 0.5 s of silence (the function
`transcribe` of `worker.py`). The silence is only in the input of the speech
recognition model. The audio file that the caller gets has no added silence.

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

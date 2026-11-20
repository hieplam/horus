# vi-speech

Text in, audio out. The workflow speaks a Vietnamese text in one of 25 preset
voices. English words inside a Vietnamese text also work. The model is
VieNeu-TTS v3 Turbo. It runs on the CPU, so the workflow needs no GPU and no
ComfyUI. Its kind is `speech`.

Model family: **VieNeu-TTS v3 Turbo** (text to speech), with the speech
recognition model **Whisper small** (reads the transcript) and the audio codec
**MOSS Audio Tokenizer Nano**.

## What works today

| Part | State |
|---|---|
| The folder, `manifest.json` and `models.json` | Done. The program holds the folder. |
| `list_workflows` and `describe_workflow` show `vi-speech` | Done. The kind is `speech`. `describe_workflow` shows the text limits, the voices, the tags and the formats. |
| The tool `generate_speech` | Not in the program yet. |
| `install_workflow`, `horus setup` and `horus doctor` for this workflow | Not in the program yet. |
| A real run through the horus MCP server | Not recorded yet. See the section "Evidence". |

## Files in this folder

| File | What it is |
|---|---|
| `manifest.json` | The public contract: summary, text limits, the 25 voices, the default voice, the tags, the formats and the measured timing. It has no graph, no subject and no output size. |
| `models.json` | The 19 model files with repo, revision, license, byte size and SHA-256. The files come from 3 repos. |
| `README.md` | This page. |

## What the caller gives

The caller gives a text and a voice name. Nothing else.

| Item | Value |
|---|---|
| Text length | 2 to 4,000 characters. horus sends the text to the model as written. |
| Voice | One of the 25 names in `manifest.json`. The default is `Hải Đăng`. |
| Tags | `[cười]` (laughs), `[thở dài]` (sighs), `[hắng giọng]` (clears the throat). The model reads any other `[tag]` aloud. |
| Formats | `wav` and `mp3`. |

Write the text with all Vietnamese tone marks. The model does not give the same
audio for the same text two times. So the caller compares the transcript of
the result with its text. Up to about 3 words in 10 of the text can be missing
from the transcript while the audio can be right: the speech recognition model
writes some words with another tone or another letter that sounds almost the
same (`lối` for `lỗi`). A share of wrong words above 0.344 is outside the
measured spread, so treat that file as wrong. `text.rules` of
`describe_workflow` gives these numbers.

## The voices

The 25 preset voices come from the Python package `vieneu` 3.8.4, file
`vieneu/assets/voices_v3_turbo.json`. `manifest.json` holds the names exactly as
that file writes them: `Adam bựa`, `Trúc Ly`, `Thiện Minh`, `Mai Anh`,
`Hải Đăng`, `Thùy Dung`, `Thiền Tâm Đức`, `Ngọc Huyền`, `Quang Sơn`,
`Ngọc Trân`, `Minh Đức`, `Phạm Tuyên`, `Thái Sơn`, `Xuân Vĩnh`, `Thanh Bình`,
`Ngọc Linh`, `Đoan Trang`, `Thục Đoan`, `Minh Triết`, `Mỹ Duyên`, `Quỳnh Anh`,
`Đức Trí`, `Kim Thanh`, `Adam`, `Quốc Tuấn`.

## Models

| Role | Repo | License | Files | Bytes |
|---|---|---|---:|---:|
| Speech model | `pnnbao-ump/VieNeu-TTS-v3-Turbo` | Apache-2.0 | 9 | 518,063,957 |
| Audio codec | `OpenMOSS-Team/MOSS-Audio-Tokenizer-Nano-ONNX` | Apache-2.0 | 6 | 90,572,161 |
| Speech recognition | `Systran/faster-whisper-small` | MIT | 4 | 486,212,372 |

The three sets work only together: the codec turns the output of the speech
model into audio, and the speech recognition model reads the transcript. The
byte size and the SHA-256 of each file are in [`models.json`](models.json) and
in [`docs/models.md`](../../docs/models.md#vietnamese-speech-vieneu-tts-v3-turbo-with-whisper-small).

## Measured numbers

Measured on 2026-11-20 with [`tests/speech/baseline.py`](../../tests/speech/baseline.py):
AMD Ryzen 7 4800H, CPU only, Windows 11. The details are in the section
"Measured baseline" of
[the spec](../../docs/specs/2026-11-20-text-to-speech.md#measured-baseline).

| Fact | Value |
|---|---|
| Time of one call, a sentence of 3 to 6 s of audio | 5.2 to 8.2 s |
| Time of one call, a text of 3,999 characters (210 s of audio) | 272 s |
| Load time of both models | 4.1 to 6.5 s |
| Memory at the peak, short text | 1,973 MB |
| Memory at the peak, a text of 3,999 characters | 5,255 MB |
| Share of wrong words, 240 files of the 5 test sentences | 0 in 133 files, 0.294 at worst. The limit of the real run is 0.344. |

## Evidence

A workflow is not done until it has run end to end through the horus MCP
server. No such run exists yet. The real run on the PC is the issue
[#190](https://github.com/hieplam/horus/issues/190) of the epic
[#181](https://github.com/hieplam/horus/issues/181). That run fills this
section.

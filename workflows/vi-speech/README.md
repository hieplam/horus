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
| The tool `generate_speech` | Done. |
| `install_workflow`, `horus setup` and `horus doctor` for this workflow | Done. |
| A real run through the horus MCP server | Recorded in the section "Evidence". One of 3 runs is above the limit on one sentence. |

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

The real run on the PC, through the horus MCP server, on 2026-11-20.
Hardware: AMD Ryzen 7 4800H, CPU only, Windows 11. The install used
`install_workflow` with both consents (219 s, 19 model files). The run is
`HORUS_SPEECH_REAL=1 go test -tags goal -v -run 'TestSpeechReal$' ./goals/2026-11-20-text-to-speech`.
The test starts `horus mcp` through [`tests/speech/real_run.py`](../../tests/speech/real_run.py),
calls `generate_speech` for the 5 sentences of
[`tests/speech/sentences.json`](../../tests/speech/sentences.json), measures the
loudness of each file with ffmpeg, and computes the share of wrong words from the
transcript of the tool. The limit is 0.17.

Three runs. Each cell is the share of wrong words, then the seconds of the call.
The first call of each run also starts the speech worker.

| Sentence | Run 1 | Run 2 | Run 3 |
|---|---|---|---|
| `plain` | 0.000, 17.3 s | 0.000, 12.2 s | 0.000, 15.3 s |
| `commas` | **0.235**, 7.7 s | 0.118, 6.3 s | 0.118, 7.7 s |
| `english-words` | 0.000, 7.4 s | 0.077, 6.1 s | 0.000, 6.8 s |
| `tag` | 0.000, 7.5 s | 0.000, 7.6 s | 0.000, 9.3 s |
| `two-sentences` | 0.040, 9.9 s | 0.120, 8.5 s | 0.000, 10.6 s |

The loudness of all 15 files is -18.0 to -17.9 LUFS. Run 1 failed the
sentence `commas` (0.235 is above 0.17). Runs 2 and 3 passed.

Transcripts of run 1 (the run with the failure) and the differences from the text:

| Sentence | Difference between the text and the transcript |
|---|---|
| `plain` | None. |
| `commas` | `Máy` read as `Nói`, `trả kết quả về cho` read as `trả kép quản về trong`. |
| `english-words` | `cloud` read as `Cloud` (the same word). |
| `tag` | None. The tag `[cười]` is not in the transcript, as expected. |
| `two-sentences` | `và` read as `vào`. |

Runs 2 and 3 differ from the text in the same way: a word with another tone or
another vowel (for example `Mấy` for `Máy`). The model gives a different audio
at each call, so the differences change from run to run.

Two example files are in [`examples/`](examples/): `plain.mp3` and `tag.mp3`
(run 3, 64 kbps).

What this evidence does not prove: nobody listened to the files. The check of
the words is the transcript. It does not prove the tone, the rhythm or the
quality of the voice.

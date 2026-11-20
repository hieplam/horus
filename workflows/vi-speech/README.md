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
| A real run through the horus MCP server | Done. See the section "Evidence". |

## Files in this folder

| File | What it is |
|---|---|
| `manifest.json` | The public contract: summary, text limits, the 25 voices, the default voice, the tags, the formats and the measured timing. It has no graph, no subject and no output size. |
| `models.json` | The 19 model files with repo, revision, license, byte size and SHA-256. The files come from 3 repos. |
| `examples/` | `plain.mp3` and `tag.mp3`: two example audio files from the real run. |
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
transcript of the tool. It reads the limit, 0.344, from the row `speech-real` of
[the spec](../../docs/specs/2026-11-20-text-to-speech.md).

Three runs, all passed. Each cell is the share of wrong words, then the seconds
of the call. The first call of each run also starts the speech worker.

| Sentence | Run 1 | Run 2 | Run 3 |
|---|---|---|---|
| `plain` | 0.000, 14.3 s | 0.125, 14.6 s | 0.000, 12.2 s |
| `commas` | 0.176, 7.1 s | 0.235, 7.7 s | 0.059, 7.0 s |
| `english-words` | 0.000, 6.8 s | 0.000, 7.2 s | 0.000, 6.5 s |
| `tag` | 0.062, 7.5 s | 0.000, 7.2 s | 0.000, 7.0 s |
| `two-sentences` | 0.120, 8.7 s | 0.000, 9.8 s | 0.080, 8.7 s |

The loudness of all 15 files is -18.0 to -17.9 LUFS.

Differences between a text and its transcript, from these runs:

| Sentence | Difference |
|---|---|
| `plain` | A word with another vowel or tone in one run. |
| `commas` | The words `Máy`, `dữ liệu`, `cho người` read as `Mái`, `giữ liệu`, `trong nơi`. The comma sentence changes most from run to run. |
| `english-words` | `cloud` is read as `cloud`. No difference. |
| `tag` | None, or one word with another tone. The tag `[cười]` is not in the transcript, as expected. |
| `two-sentences` | `và` read as `vào`, `ổn định` read as `định`, `lượng` read as `lương`. One run joins the two sentences with a comma. |

The speech model gives a different audio at each call, so the differences change
from run to run. Two example files are in [`examples/`](examples/): `plain.mp3`
and `tag.mp3` (64 kbps).

What this evidence does not prove: nobody listened to the files. The check of
the words is the transcript. It does not prove the tone, the rhythm or the
quality of the voice.

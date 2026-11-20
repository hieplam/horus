"""The speech worker of horus: one Python process that holds the speech model and the speech recognition model.

The `horus` program holds this file (internal/speechworker, go:embed). horus writes it to the user folder and starts
it with the Python of the speech venv:

    <venv python> -I worker.py --models <folder>

The model folder has three folders: `vieneu` (config.json, denoiser.onnx, onnx_update/), `codec` and
`whisper-small`. workflows/vi-speech/models.json lists the files. The worker loads each model from that folder only:
it sets HF_HUB_OFFLINE=1 and an empty Hugging Face cache, so a Python package cannot download a model.

The protocol: one JSON line for each message, UTF-8, on the standard input and output of the worker.

    after the load   {"ready": true, "voices": 25, "load_seconds": 5.1}
                     {"ready": false, "error": "..."}  and the worker exits 1
    request          {"id": 1, "op": "voices"}
    answer           {"id": 1, "ok": true, "voices": [{"id": "...", "label": "..."}], "default": "..."}
    request          {"id": 2, "op": "speak", "text": "...", "voice": "...", "out": "<absolute path>", "format": "wav"}
    answer           {"id": 2, "ok": true, "path": "...", "bytes": 1, "format": "wav", "audio_seconds": 3.6,
                      "lufs": -18.0, "transcript": "...", "speak_seconds": 2.5, "read_seconds": 3.3}
    failed request   {"id": 2, "ok": false, "error": "..."}  and the worker reads the next request

The worker answers the requests one at a time, in order. It ends at the end of its standard input. The standard
output carries only the protocol: the worker sends the output of the Python packages to the standard error.

`speak` sends the text to the model as written. It only joins the white space. The audio file gets the loudness
-18 LUFS (ffmpeg, the filter ebur128). The format `wav` is 48 kHz, 16-bit, mono. The format `mp3` is 44.1 kHz,
128 kbps, mono. The worker writes the file to `<out>.part` and then renames it, so a failed request leaves no file at
`out`. Then the speech recognition model reads the file and gives the transcript. The input of that model is the
audio of the file after 0.5 s of silence: with the silence, the model reads fewer wrong words. The silence is not in
the audio file.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

TARGET_LUFS = -18.0
PEAK_LIMIT = 0.891  # -1 dBFS, so the gain never clips
LEAD_SILENCE = 0.5  # seconds of silence before the audio in the input of the speech recognition model
FORMATS = {  # the ffmpeg arguments of each format; -f names the format, because the file name ends with .part
    "wav": ["-ar", "48000", "-ac", "1", "-c:a", "pcm_s16le", "-f", "wav"],
    "mp3": ["-ar", "44100", "-ac", "1", "-b:a", "128k", "-f", "mp3"],
}


class Refused(Exception):
    """A request that the worker cannot do. The worker answers it with ok=false and reads the next request."""


def tool(name: str) -> str:
    exe = shutil.which(name)
    if not exe:
        raise Refused(f"{name} is not on the PATH")
    return exe


def run(args: list[str], **kwargs) -> subprocess.CompletedProcess:
    """Run a program with no standard input. The standard input of the worker carries the requests, and ffmpeg
    reads its standard input when it gets one."""
    return subprocess.run(args, stdin=subprocess.DEVNULL, capture_output=True, **kwargs)


def ffmpeg(*args: str) -> list[str]:
    return [tool("ffmpeg"), "-nostdin", *args]


def lufs(path: Path) -> float | None:
    """The integrated loudness of an audio file, or None when ffmpeg cannot measure it."""
    r = run(ffmpeg("-nostats", "-i", str(path), "-af", "ebur128=framelog=quiet", "-f", "null", "-"),
            text=True, encoding="utf-8", errors="replace")
    hits = re.findall(r"I:\s+(-?[\d.]+) LUFS", r.stderr)
    return float(hits[-1]) if hits else None


def audio_seconds(path: Path) -> float:
    r = run([tool("ffprobe"), "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", str(path)],
            text=True, check=True)
    return float(r.stdout.strip())


def encode(take: Path, out: Path, fmt: list[str]) -> None:
    """Write `take` to `out` in the format `fmt` at TARGET_LUFS. The limiter and the encoder change the level a
    little, so the result is measured and corrected two more times."""
    gain = TARGET_LUFS - (lufs(take) or TARGET_LUFS)
    for _ in range(3):
        r = run(ffmpeg("-y", "-loglevel", "error", "-i", str(take),
                       "-af", f"volume={gain:.2f}dB,alimiter=limit={PEAK_LIMIT}", *fmt, str(out)),
                text=True, encoding="utf-8", errors="replace")
        if r.returncode != 0:
            raise Refused(f"ffmpeg did not write the audio file: {r.stderr.strip()[-500:]}")
        measured = lufs(out)
        if measured is None or abs(measured - TARGET_LUFS) <= 0.15:
            return
        gain += TARGET_LUFS - measured


def samples_16k(path: Path):
    """The input of the speech recognition model: LEAD_SILENCE seconds of silence, then the audio of the file as
    16 kHz mono float samples. ffmpeg decodes the file, so the result does not depend on the audio reader of the
    faster-whisper package. The silence is only in this input, not in the audio file."""
    import numpy as np

    r = run(ffmpeg("-loglevel", "error", "-i", str(path), "-ar", "16000", "-ac", "1", "-f", "f32le", "-"), check=True)
    lead = np.zeros(round(16000 * LEAD_SILENCE), dtype=np.float32)
    return np.concatenate([lead, np.frombuffer(r.stdout, dtype=np.float32)])


def transcribe(stt, path: Path) -> str:
    """The transcript of the audio file at path: the words that the speech recognition model reads from it.
    tests/speech/baseline.py calls this function too, so the measured baseline uses these settings."""
    segments, _info = stt.transcribe(samples_16k(path), language="vi", beam_size=5, condition_on_previous_text=False)
    return " ".join(s.text.strip() for s in segments).strip()


def load_vieneu(models: Path):
    """Load VieNeu from the local folders. The class of vieneu 3.8.4 passes `onnx_dir` to its engine but not
    `codec_dir`, so this adds `codec_dir` to the call of the engine for the time of the load."""
    from vieneu import Vieneu
    from vieneu._v3_turbo_engine import onnx_runtime_lite

    engine_init = onnx_runtime_lite.OnnxV3LiteEngine.__init__

    def init_with_codec(self, *args, **kwargs):
        kwargs.setdefault("codec_dir", str(models / "codec"))
        engine_init(self, *args, **kwargs)

    onnx_runtime_lite.OnnxV3LiteEngine.__init__ = init_with_codec
    try:
        return Vieneu(mode="v3turbo", backbone_repo=str(models / "vieneu"),
                      onnx_dir=str(models / "vieneu" / "onnx_update"), precision="fp32")
    finally:
        onnx_runtime_lite.OnnxV3LiteEngine.__init__ = engine_init


class Worker:
    """The two models, loaded one time, and the requests that use them."""

    def __init__(self, models: Path, scratch: Path):
        self.scratch = scratch
        self.tts = load_vieneu(models)
        from faster_whisper import WhisperModel

        self.stt = WhisperModel(str(models / "whisper-small"), device="cpu", compute_type="int8")
        self.voices = [{"id": voice_id, "label": label} for label, voice_id in self.tts.list_preset_voices()]
        ids = [v["id"] for v in self.voices]
        default = getattr(self.tts, "_default_voice", None)
        self.default = default if default in ids else (ids[0] if ids else "")

    def answer_voices(self, _req: dict) -> dict:
        return {"voices": self.voices, "default": self.default}

    def answer_speak(self, req: dict) -> dict:
        text = re.sub(r"\s+", " ", str(req.get("text") or "")).strip()
        voice = req.get("voice") or self.default
        fmt = req.get("format") or "wav"
        out = Path(str(req.get("out") or ""))
        if not text:
            raise Refused("the text has no words")
        if voice not in [v["id"] for v in self.voices]:
            raise Refused(f"no preset voice {voice!r}")
        if fmt not in FORMATS:
            raise Refused(f"no format {fmt!r}: the formats are {', '.join(FORMATS)}")
        if not out.is_absolute():
            raise Refused(f"out is not an absolute path: {str(out)!r}")
        out.parent.mkdir(parents=True, exist_ok=True)

        started = time.monotonic()
        take = self.scratch / "take.wav"
        self.tts.save(self.tts.infer(text, voice=voice), str(take))
        speak_seconds = time.monotonic() - started
        part = out.with_name(out.name + ".part")
        try:
            encode(take, part, FORMATS[fmt])
            os.replace(part, out)
        finally:
            part.unlink(missing_ok=True)
            take.unlink(missing_ok=True)

        started = time.monotonic()
        transcript = transcribe(self.stt, out)
        return {"path": str(out), "bytes": out.stat().st_size, "format": fmt, "audio_seconds": round(audio_seconds(out), 2),
                "lufs": lufs(out), "transcript": transcript, "speak_seconds": round(speak_seconds, 1),
                "read_seconds": round(time.monotonic() - started, 1)}


def protocol_stream():
    """The standard output for the protocol only. The file descriptor 1 then points to the standard error, so a
    print of a Python package, or of its C code, cannot break a protocol line."""
    proto = open(os.dup(sys.stdout.fileno()), "w", encoding="utf-8", newline="\n")
    sys.stdout.flush()
    os.dup2(sys.stderr.fileno(), sys.stdout.fileno())
    sys.stdout = sys.stderr
    return proto


def main() -> int:
    ap = argparse.ArgumentParser(description="the speech worker of horus")
    ap.add_argument("--models", type=Path, required=True, help="the folder with vieneu/, codec/ and whisper-small/")
    args = ap.parse_args()
    proto = protocol_stream()

    def send(msg: dict) -> None:
        proto.write(json.dumps(msg, ensure_ascii=True) + "\n")
        proto.flush()

    with tempfile.TemporaryDirectory(prefix="horus-speech-") as tmp:
        scratch = Path(tmp)
        os.environ["HF_HUB_OFFLINE"] = "1"  # a download fails, so each model comes from the local folder
        os.environ["HF_HOME"] = str(scratch / "empty-hf-cache")
        os.environ.setdefault("HF_HUB_DISABLE_SYMLINKS", "1")
        started = time.monotonic()
        try:
            worker = Worker(args.models.resolve(), scratch)
        except Exception as exc:  # a missing file, a missing package: the caller gets the reason
            send({"ready": False, "error": f"{type(exc).__name__}: {exc}"})
            return 1
        send({"ready": True, "voices": len(worker.voices), "load_seconds": round(time.monotonic() - started, 1)})

        ops = {"voices": worker.answer_voices, "speak": worker.answer_speak}
        for raw in sys.stdin.buffer:
            line = raw.decode("utf-8", errors="replace").strip()
            if not line:
                continue
            rid = None
            try:
                req = json.loads(line)
                rid = req.get("id") if isinstance(req, dict) else None
                op = ops.get(req.get("op")) if isinstance(req, dict) else None
                if op is None:
                    raise Refused(f"no request {line[:80]!r}: the requests are {', '.join(ops)}")
                send({"id": rid, "ok": True, **op(req)})
            except Exception as exc:  # the worker keeps running for the next request
                msg = str(exc) if isinstance(exc, Refused) else f"{type(exc).__name__}: {exc}"
                send({"id": rid, "ok": False, "error": msg})
    return 0


if __name__ == "__main__":
    sys.exit(main())

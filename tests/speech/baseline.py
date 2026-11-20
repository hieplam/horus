"""baseline: measure the speech models of the workflow vi-speech on this machine.

It speaks the 5 fixed sentences of tests/speech/sentences.json with VieNeu, reads each file back with the speech
recognition model through the function `transcribe` of internal/speechworker/worker.py, and prints the facts that docs/specs/2026-11-20-text-to-speech.md needs: the share of wrong
words, the loudness, the seconds of each call, the load time, the memory and the longest text that works.

Run it with the Python of a venv that has the packages `vieneu` and `faster-whisper`, with ffmpeg on the PATH:

    <venv python> tests/speech/baseline.py --models <folder> [--out <folder>] [--runs N] [--long]

--runs N speaks each sentence N times and ends with a table: for each sentence, the runs, the median and the worst
share of wrong words. The spread of the spec comes from `--runs 12`. The folder of --out keeps each file, and
results.jsonl in it keeps the transcript and the share of each file.

The model folder has three folders: `vieneu` (config.json, denoiser.onnx, onnx_update/), `codec` and
`whisper-small`. The files and their SHA-256 are in workflows/vi-speech/models.json. The script loads each model
from that folder only: it sets HF_HUB_OFFLINE=1 and an empty Hugging Face cache, so a download fails.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import statistics
import subprocess
import sys
import tempfile
import time
import unicodedata
from collections import Counter
from pathlib import Path

HERE = Path(__file__).resolve().parent
WORKER = HERE.parents[1] / "internal" / "speechworker" / "worker.py"
TARGET_LUFS = -18.0
PEAK_LIMIT = 0.891  # -1 dBFS, so the gain never clips
WAV = ["-ar", "48000", "-ac", "1", "-c:a", "pcm_s16le"]
TAGS = {"[cười]", "[thở dài]", "[hắng giọng]"}  # the tags that VieNeu performs


def tool(name: str) -> str:
    exe = shutil.which(name)
    if not exe:
        sys.exit(f"baseline: {name} is not on the PATH")
    return exe


def lufs(path: Path) -> float | None:
    """The integrated loudness of an audio file, or None when ffmpeg cannot measure it."""
    r = subprocess.run([tool("ffmpeg"), "-nostats", "-i", str(path), "-af", "ebur128=framelog=quiet", "-f", "null", "-"],
                       capture_output=True, text=True, encoding="utf-8", errors="replace")
    hits = re.findall(r"I:\s+(-?[\d.]+) LUFS", r.stderr)
    return float(hits[-1]) if hits else None


def audio_seconds(path: Path) -> float:
    r = subprocess.run([tool("ffprobe"), "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", str(path)],
                       capture_output=True, text=True, check=True)
    return float(r.stdout.strip())


def encode(take: Path, out: Path) -> None:
    """Write `take` as a 48 kHz mono WAV at TARGET_LUFS. The limiter changes the level a little, so the result
    is measured and corrected two more times."""
    gain = TARGET_LUFS - (lufs(take) or TARGET_LUFS)
    for _ in range(3):
        subprocess.run([tool("ffmpeg"), "-y", "-loglevel", "error", "-i", str(take),
                        "-af", f"volume={gain:.2f}dB,alimiter=limit={PEAK_LIMIT}", *WAV, str(out)], check=True)
        measured = lufs(out)
        if measured is None or abs(measured - TARGET_LUFS) <= 0.15:
            return
        gain += TARGET_LUFS - measured


def worker_module():
    """The file of the speech worker as a module. The baseline reads each file with its function `transcribe`, so
    the measured share of wrong words comes from the transcript step of the product."""
    import importlib.util

    spec = importlib.util.spec_from_file_location("horus_speech_worker", WORKER)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def words(text: str) -> list[str]:
    """The words of a text: no tags, lower case, no punctuation."""
    text = re.sub(r"\[[^\]]*\]", " ", text)
    text = unicodedata.normalize("NFC", text).lower()
    return re.findall(r"[^\W_]+", text, flags=re.UNICODE)


def wrong_share(text: str, transcript: str) -> float:
    """The words of the text that the transcript does not have, divided by the words of the text."""
    want, got = Counter(words(text)), Counter(words(transcript))
    total = sum(want.values())
    return round(sum((want - got).values()) / total, 3) if total else 1.0


def peak_memory_mb() -> float:
    """The largest amount of memory that this process held."""
    if os.name == "nt":
        import ctypes
        from ctypes import wintypes

        class Counters(ctypes.Structure):
            _fields_ = [("cb", wintypes.DWORD), ("PageFaultCount", wintypes.DWORD),
                        ("PeakWorkingSetSize", ctypes.c_size_t), ("WorkingSetSize", ctypes.c_size_t),
                        ("QuotaPeakPagedPoolUsage", ctypes.c_size_t), ("QuotaPagedPoolUsage", ctypes.c_size_t),
                        ("QuotaPeakNonPagedPoolUsage", ctypes.c_size_t), ("QuotaNonPagedPoolUsage", ctypes.c_size_t),
                        ("PagefileUsage", ctypes.c_size_t), ("PeakPagefileUsage", ctypes.c_size_t)]

        c = Counters()
        c.cb = ctypes.sizeof(c)
        kernel32 = ctypes.WinDLL("kernel32")
        kernel32.GetCurrentProcess.restype = wintypes.HANDLE
        psapi = ctypes.WinDLL("psapi")
        psapi.GetProcessMemoryInfo.argtypes = [wintypes.HANDLE, ctypes.POINTER(Counters), wintypes.DWORD]
        psapi.GetProcessMemoryInfo(kernel32.GetCurrentProcess(), ctypes.byref(c), c.cb)
        return round(c.PeakWorkingSetSize / 1e6, 0)
    import resource

    peak = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
    return round(peak / (1e6 if sys.platform == "darwin" else 1e3), 0)


def load_vieneu(models: Path):
    """Load VieNeu from the local folders. The class of vieneu 3.8.4 passes `onnx_dir` to its engine but not
    `codec_dir`, so this adds `codec_dir` to the call of the engine."""
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


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--models", type=Path, required=True, help="the folder with vieneu/, codec/ and whisper-small/")
    ap.add_argument("--out", type=Path, default=None, help="where the audio files go; default: a temp folder")
    ap.add_argument("--voice", default=None, help="a preset voice; default: the default voice of the package")
    ap.add_argument("--runs", type=int, default=1, help="speak each sentence this many times (the spread of the spec: 12)")
    ap.add_argument("--long", action="store_true", help="also measure the longest text that works (slow)")
    args = ap.parse_args()
    models = args.models.resolve()
    for part in ("vieneu/onnx_update/config.json", "codec", "whisper-small/model.bin"):
        if not (models / part).exists():
            sys.exit(f"baseline: {models / part} does not exist")
    sys.stdout.reconfigure(encoding="utf-8")

    scratch = Path(tempfile.mkdtemp(prefix="horus-speech-baseline-"))
    out_dir = (args.out or scratch / "out").resolve()
    out_dir.mkdir(parents=True, exist_ok=True)
    os.environ["HF_HUB_OFFLINE"] = "1"  # a download fails, so each model comes from the local folder
    os.environ["HF_HOME"] = str(scratch / "empty-hf-cache")
    os.environ.setdefault("HF_HUB_DISABLE_SYMLINKS", "1")

    started = time.monotonic()
    tts = load_vieneu(models)
    tts_load = time.monotonic() - started
    from faster_whisper import WhisperModel

    started = time.monotonic()
    stt = WhisperModel(str(models / "whisper-small"), device="cpu", compute_type="int8")
    stt_load = time.monotonic() - started
    worker = worker_module()
    voices = [voice_id for _label, voice_id in tts.list_preset_voices()]
    voice = args.voice or getattr(tts, "_default_voice", None) or voices[0]
    print(f"load: vieneu {tts_load:.1f} s, speech recognition {stt_load:.1f} s, voices {len(voices)}, voice '{voice}'")

    def speak(text: str, out: Path) -> dict:
        """One call as the speech worker will do it: audio, loudness, transcript."""
        clean = re.sub(r"\[[^\]]*\]", lambda m: m.group(0) if m.group(0) in TAGS else " ", text)
        started = time.monotonic()
        take = scratch / "take.wav"
        tts.save(tts.infer(re.sub(r"\s+", " ", clean).strip(), voice=voice), str(take))
        speak_s = time.monotonic() - started
        encode(take, out)
        started_read = time.monotonic()
        transcript = worker.transcribe(stt, out)
        return {"seconds": round(time.monotonic() - started, 1), "speak_seconds": round(speak_s, 1),
                "read_seconds": round(time.monotonic() - started_read, 1), "audio_seconds": round(audio_seconds(out), 2),
                "lufs": lufs(out), "transcript": transcript, "wrong": wrong_share(text, transcript)}

    sentences = json.loads((HERE / "sentences.json").read_text(encoding="utf-8"))["sentences"]
    made, worst = 0, 0.0
    shares: dict[str, list[float]] = {s["id"]: [] for s in sentences}
    results = (out_dir / "results.jsonl").open("w", encoding="utf-8")
    for run in range(1, args.runs + 1):
        for s in sentences:
            name = s["id"] if args.runs == 1 else f"{s['id']}-{run:02d}"
            out = out_dir / f"{name}.wav"
            r = speak(s["text"], out)
            ok = out.is_file() and r["lufs"] is not None and abs(r["lufs"] - TARGET_LUFS) <= 0.5 and r["transcript"]
            made += 1 if ok else 0
            worst = max(worst, r["wrong"])
            shares[s["id"]].append(r["wrong"])
            results.write(json.dumps({"run": run, "id": s["id"], "file": out.name, **r}, ensure_ascii=False) + "\n")
            results.flush()
            print(f"{name}: {'ok' if ok else 'FAIL'}, wrong words {r['wrong']}, loudness {r['lufs']} LUFS, "
                  f"audio {r['audio_seconds']} s, call {r['seconds']} s (speak {r['speak_seconds']} s, read {r['read_seconds']} s)")
            print(f"  text:       {s['text']}")
            print(f"  transcript: {r['transcript']}")
    results.close()
    print(f"\nshare of wrong words, {args.runs} runs of each sentence:\n")
    print("| Sentence | Runs | Median | Worst |\n|---|---:|---:|---:|")
    for sid, vals in shares.items():
        print(f"| `{sid}` | {len(vals)} | {statistics.median(vals):.3f} | {max(vals):.3f} |")
    print()

    if args.long:
        base = " ".join(s["text"] for s in sentences if "[" not in s["text"])
        longest = 0
        for size in (500, 1000, 2000, 4000):
            text = (base + " ") * (size // len(base) + 1)
            text = text[:size].rsplit(" ", 1)[0]
            r = speak(text, out_dir / f"long-{size}.wav")
            print(f"long {len(text)} chars: wrong words {r['wrong']}, audio {r['audio_seconds']} s, call {r['seconds']} s")
            if r["wrong"] > worst + 0.05:
                break
            longest = len(text)
        print(f"longest text that works: {longest} chars")

    print(f"memory: peak {peak_memory_mb():.0f} MB in one process with both models")
    print(f"baseline: {made} of {len(sentences) * args.runs} files, worst share of wrong words {worst}")
    print(f"files: {out_dir} (each file, its transcript and its share: results.jsonl)")
    return 0 if made == len(sentences) * args.runs else 1


if __name__ == "__main__":
    sys.exit(main())

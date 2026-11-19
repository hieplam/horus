"""worker_real: a test of the speech worker (internal/speechworker/worker.py) with the real models.

It starts the worker as horus starts it, sends `voices` and one `speak` request with the first fixed sentence of
tests/speech/sentences.json, and checks the answer and the file. It measures the loudness and the format of the file
with ffmpeg and ffprobe, not from the answer of the worker. It runs only with HORUS_SPEECH_REAL=1, because it needs
the speech venv, the model files and about 2 GB of memory:

    HORUS_SPEECH_REAL=1 <speech venv python> tests/speech/worker_real.py --models <folder> [--format wav|mp3]

--python is the Python that starts the worker; the default is the Python that runs this test. The model folder has
the folders vieneu/, codec/ and whisper-small/ (workflows/vi-speech/models.json); the default is the value of
HORUS_SPEECH_MODELS. ffmpeg and ffprobe must be on the PATH. On success the last line is:

    worker: 1 file, loudness in -18.5..-17.5, transcript not empty
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
import threading
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
WORKER = ROOT / "internal" / "speechworker" / "worker.py"
LOW, HIGH = -18.5, -17.5
FORMATS = {"wav": {"sample_rate": "48000", "channels": "1", "codec_name": "pcm_s16le"},
           "mp3": {"sample_rate": "44100", "channels": "1", "codec_name": "mp3"}}


def probe(path: Path) -> dict:
    """The facts of the first audio stream of the file, from ffprobe."""
    r = subprocess.run([shutil.which("ffprobe") or "ffprobe", "-v", "error", "-select_streams", "a:0",
                        "-show_entries", "stream=sample_rate,channels,codec_name,bit_rate", "-of", "json", str(path)],
                       capture_output=True, text=True, check=True)
    return json.loads(r.stdout)["streams"][0]


def lufs(path: Path) -> float | None:
    r = subprocess.run([shutil.which("ffmpeg") or "ffmpeg", "-nostdin", "-nostats", "-i", str(path),
                        "-af", "ebur128=framelog=quiet", "-f", "null", "-"],
                       capture_output=True, text=True, encoding="utf-8", errors="replace")
    hits = re.findall(r"I:\s+(-?[\d.]+) LUFS", r.stderr)
    return float(hits[-1]) if hits else None


class Worker:
    """The worker process and its protocol lines."""

    def __init__(self, python: str, models: Path, log: Path):
        self.log = open(log, "w", encoding="utf-8", errors="replace")
        self.proc = subprocess.Popen([python, "-I", str(WORKER), "--models", str(models)], stdin=subprocess.PIPE,
                                     stdout=subprocess.PIPE, stderr=self.log)
        self.next_id = 0

    def line(self, timeout: float) -> dict:
        """The next line of the standard output. Each line must be one JSON object."""
        got: list[bytes] = []
        t = threading.Thread(target=lambda: got.append(self.proc.stdout.readline()), daemon=True)
        t.start()
        t.join(timeout)
        if not got:
            raise SystemExit(f"worker_real: FAIL: no line from the worker in {timeout:.0f} s")
        if not got[0]:
            raise SystemExit(f"worker_real: FAIL: the worker stopped with exit code {self.proc.wait()}")
        try:
            msg = json.loads(got[0].decode("utf-8"))
        except ValueError:
            raise SystemExit(f"worker_real: FAIL: a line that is not JSON on the standard output: {got[0][:200]!r}")
        if not isinstance(msg, dict):
            raise SystemExit(f"worker_real: FAIL: a line that is not a JSON object: {got[0][:200]!r}")
        return msg

    def call(self, req: dict, timeout: float) -> dict:
        self.next_id += 1
        req = {"id": self.next_id, **req}
        self.proc.stdin.write((json.dumps(req, ensure_ascii=False) + "\n").encode("utf-8"))
        self.proc.stdin.flush()
        ans = self.line(timeout)
        if ans.get("id") != self.next_id:
            raise SystemExit(f"worker_real: FAIL: the answer has the id {ans.get('id')}, want {self.next_id}")
        if not ans.get("ok"):
            raise SystemExit(f"worker_real: FAIL: the worker answered an error: {ans.get('error')}")
        return ans

    def close(self) -> int:
        self.proc.stdin.close()
        try:
            return self.proc.wait(30)
        except subprocess.TimeoutExpired:
            self.proc.kill()
            return -1
        finally:
            self.log.close()


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--python", default=sys.executable, help="the Python of the speech venv; default: this Python")
    ap.add_argument("--models", type=Path, default=os.environ.get("HORUS_SPEECH_MODELS"),
                    help="the folder with vieneu/, codec/ and whisper-small/; default: HORUS_SPEECH_MODELS")
    ap.add_argument("--format", choices=sorted(FORMATS), default="wav")
    ap.add_argument("--keep", type=Path, default=None, help="copy the audio file and the log of the worker to this folder")
    args = ap.parse_args()
    sys.stdout.reconfigure(encoding="utf-8")
    if os.environ.get("HORUS_SPEECH_REAL") != "1":
        print("worker_real: skipped: set HORUS_SPEECH_REAL=1 to run the test with the real models")
        return 0
    if args.models is None or not (args.models / "whisper-small" / "model.bin").is_file():
        sys.exit("worker_real: give --models (or HORUS_SPEECH_MODELS): the folder with vieneu/, codec/ and whisper-small/")
    for name in ("ffmpeg", "ffprobe"):
        if not shutil.which(name):
            sys.exit(f"worker_real: {name} is not on the PATH")

    text = json.loads((ROOT / "tests" / "speech" / "sentences.json").read_text(encoding="utf-8"))["sentences"][0]["text"]
    with tempfile.TemporaryDirectory(prefix="horus-worker-real-") as tmp:
        out = Path(tmp) / "out" / f"plain.{args.format}"
        log = Path(tmp) / "worker.stderr.log"
        started = time.monotonic()
        w = Worker(args.python, args.models.resolve(), log)
        try:
            ready = w.line(300)
            if ready.get("ready") is not True:
                raise SystemExit(f"worker_real: FAIL: the worker did not load the models: {ready}")
            print(f"ready: {ready['voices']} voices, load {ready['load_seconds']} s, start {time.monotonic() - started:.1f} s")
            v = w.call({"op": "voices"}, 60)
            ids = [x["id"] for x in v["voices"]]
            print(f"voices: {len(ids)}, default {v['default']!r}")
            if v["default"] not in ids:
                raise SystemExit("worker_real: FAIL: the default voice is not in the voices")
            ans = w.call({"op": "speak", "text": text, "voice": v["default"], "out": str(out), "format": args.format}, 600)
        finally:
            code = w.close()
            if args.keep:
                args.keep.mkdir(parents=True, exist_ok=True)
                shutil.copy(log, args.keep / "worker.stderr.log")
                if out.is_file():
                    shutil.copy(out, args.keep / out.name)
        print(f"answer: audio {ans['audio_seconds']} s, {ans['bytes']} bytes, lufs {ans['lufs']}, "
              f"speak {ans['speak_seconds']} s, read {ans['read_seconds']} s")
        print(f"  text:       {text}")
        print(f"  transcript: {ans['transcript']}")
        files = [p for p in out.parent.iterdir() if p.is_file()]
        measured = lufs(out) if out.is_file() else None
        facts = probe(out) if out.is_file() else {}
        print(f"file: {[p.name for p in files]}, measured loudness {measured} LUFS, {facts}, worker exit code {code}")
        want = FORMATS[args.format]
        checks = {
            "1 file": files == [out] and Path(ans["path"]) == out and ans["bytes"] == out.stat().st_size,
            f"loudness in {LOW}..{HIGH}": measured is not None and LOW <= measured <= HIGH,
            "transcript not empty": bool(ans["transcript"].strip()),
            f"format {args.format}": all(str(facts.get(k)) == val for k, val in want.items()),
            "worker exit code 0 at the end of its input": code == 0,
        }
        for name, ok in checks.items():
            if not ok:
                print(f"worker_real: FAIL: {name}")
        if not all(checks.values()):
            return 1
    print(f"worker: 1 file, loudness in {LOW}..{HIGH}, transcript not empty")
    return 0


if __name__ == "__main__":
    sys.exit(main())

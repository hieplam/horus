#!/usr/bin/env python3
"""The peak tool: many agents call generate_image at the same time.

    python tests/peak_load.py --target host --server-cmd bin/horus.exe mcp --calls 12 --leave 3,6

The tool starts --calls MCP sessions at the same time, one `horus mcp` process
for each, and calls generate_image once in each session. A fake ComfyUI in this
process (on a free port) answers the backend calls. The fake runs at most
--parallel jobs at the same time (default 1, as ComfyUI) and holds the others
in its queue. One job takes 240 / --scale seconds.

--scale divides every time. With the default 10, an image takes 24 s,
HORUS_GENERATE_TIMEOUT is 1800 / 10 s and HORUS_QUEUE_TIMEOUT is 7200 / 10 s.
--leave 3,6 --leave-at 60 ends the server process of the calls 3 and 6 (number
1 is the first call) hard, 60 / --scale seconds after the calls start. This is
an agent that leaves.

Call n sends its call (n - 1) x 0.1 s after call 1, so the number of a call
is its order: call 1 is the first call.

Targets:
  host   starts the server program that --server-cmd names (the direct path).
  stack  starts `docker exec -i <stack>-api horus mcp`. The stack must reach the
         fake as its ComfyUI: start the stack with HORUS_COMFYUI_URL
         http://host.docker.internal:<--port>. The tool waits up to
         --ready-timeout seconds until `docker exec <stack>-api horus ready`
         exits 0, then starts the calls. --out-dir must be a shared folder of
         the stack (HORUS_SHARED_DIRS).

    python tests/peak_load.py --target stack --stack horus_test --port 18188 --out-dir <shared folder> --calls 12 --leave 3,6

--listen is the address of the fake: 127.0.0.1 (the default) for Docker
Desktop, which gives a container the loopback of the host at
host.docker.internal. Docker on Linux needs --listen 0.0.0.0.

The last line of stdout is one JSON line. All seconds are wall-clock seconds
that the tool measured; the tool does not multiply them by --scale.

  png                      calls that returned a PNG file on disk
  generation_timeout       calls that ended with GENERATION_TIMEOUT
  queue_full               calls that ended with QUEUE_FULL
  wait_for_error_seconds   the longest wait of a call that ended with an error
  gpu_seconds_for_nobody   seconds that the fake ran a job of a call that had
                           left, counted from the time that the call left
  max_jobs_in_comfyui      the most jobs that the fake held at one time,
                           running and waiting
  max_running              the most jobs that ran at the same time

The details go to stderr and to --log (one line for each request of the fake).
The exit code is 0 when the run ended, whatever the numbers. Python 3.12 with
tests/requirements.txt.
"""

from __future__ import annotations

import argparse
import asyncio
import json
import os
import signal
import subprocess
import sys
import tempfile
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import mcp.client.stdio as mcp_stdio  # noqa: E402
from mcp import ClientSession, StdioServerParameters  # noqa: E402
from mcp.client.stdio import stdio_client  # noqa: E402

from mcp_contract import FIXTURE, SUBJECT, env_of, make_sandbox, resolve_program, tiny_png  # noqa: E402

JOB_SECONDS = 240.0  # the time of one image before --scale
GENERATE_TIMEOUT = 1800  # HORUS_GENERATE_TIMEOUT before --scale
QUEUE_TIMEOUT = 7200  # HORUS_QUEUE_TIMEOUT before --scale
RUN_ESTIMATE = 240  # HORUS_RUN_ESTIMATE before --scale: the run time of one job that the estimated start uses
SEED_BASE = 1000  # call n uses the seed SEED_BASE + n, so the fake knows which call sent a job
CALL_GAP = 0.1  # call n sends its call (n - 1) * CALL_GAP seconds after call 1, so the number of a call is its order


class FakeComfy:
    """A fake ComfyUI with a queue that runs at most `parallel` jobs at the same time.

    The state moves with the clock: each request first calls advance(), which
    finishes the jobs whose end time has passed and starts the next queued job
    at the end time of the job before it.
    """

    def __init__(self, parallel: int, job_seconds: float, log_path: Path) -> None:
        self.parallel, self.job_seconds = parallel, job_seconds
        self.lock = threading.Lock()
        self.jobs: dict[str, dict] = {}
        self.order: list[str] = []  # prompt ids in submit order
        self.t0 = time.monotonic()
        self.counter = 0
        self.max_held = 0
        self.max_running = 0
        self.posts_prompt = 0
        self.log = open(log_path, "w", encoding="utf-8")

    def now(self) -> float:
        return time.monotonic() - self.t0

    def write_log(self, line: str) -> None:
        self.log.write(f"{self.now():9.3f} {line}\n")
        self.log.flush()

    def _running(self) -> list[dict]:
        return [j for j in self.jobs.values() if j["state"] == "running"]

    def _queued(self) -> list[dict]:
        return sorted((j for j in self.jobs.values() if j["state"] == "queued"), key=lambda j: j["number"])

    def _start(self, job: dict, at: float) -> None:
        job["state"], job["start"], job["end"] = "running", at, at + self.job_seconds

    def _count(self) -> None:
        running, queued = len(self._running()), len(self._queued())
        self.max_running = max(self.max_running, running)
        self.max_held = max(self.max_held, running + queued)

    def advance(self, now: float) -> None:
        while True:
            running = self._running()
            if running:
                first = min(running, key=lambda j: j["end"])
                if first["end"] <= now:
                    first["state"] = "done"
                    queued = self._queued()
                    if queued:
                        self._start(queued[0], first["end"])
                    continue
            queued = self._queued()
            if queued and len(running) < self.parallel:
                self._start(queued[0], max(queued[0]["submit"], 0.0))
                continue
            return

    def submit(self, graph: dict) -> tuple[str, int]:
        with self.lock:
            now = self.now()
            self.advance(now)
            self.counter += 1
            self.posts_prompt += 1
            pid = f"job-{self.counter}"
            seed = next((n["inputs"].get("seed") for n in graph.values()
                         if isinstance(n, dict) and n.get("class_type") == "KSampler"), None)
            self.jobs[pid] = {"id": pid, "number": self.counter, "seed": seed, "submit": now,
                              "state": "queued", "start": None, "end": None}
            self.order.append(pid)
            self.advance(now)
            self._count()
            self.write_log(f"POST /prompt {pid} seed={seed} held={len(self._running()) + len(self._queued())}")
            return pid, self.counter

    def interrupt(self, pid: str | None) -> None:
        with self.lock:
            now = self.now()
            self.advance(now)
            for j in self._running():
                if pid in (None, j["id"]):
                    j["state"], j["end"] = "interrupted", now
            self.advance(now)
            self.write_log(f"POST /interrupt {pid}")

    def delete(self, ids: list[str]) -> None:
        with self.lock:
            now = self.now()
            self.advance(now)
            for i in ids:
                j = self.jobs.get(i)
                if j and j["state"] == "queued":
                    j["state"] = "deleted"
            self.write_log(f"POST /queue delete {ids}")

    def queue(self) -> dict:
        with self.lock:
            self.advance(self.now())
            row = lambda j: [j["number"], j["id"], {}]  # noqa: E731
            return {"queue_running": [row(j) for j in self._running()], "queue_pending": [row(j) for j in self._queued()]}

    def history(self, pid: str) -> dict:
        with self.lock:
            self.advance(self.now())
            j = self.jobs.get(pid)
            if not j or j["state"] != "done":
                return {}
            return {pid: {"status": {"status_str": "success", "completed": True, "messages": [
                ["execution_start", {"timestamp": int(j["start"] * 1000)}],
                ["execution_success", {"timestamp": int(j["end"] * 1000)}]]},
                "outputs": {"9": {"images": [{"filename": f"{pid}.png", "subfolder": "", "type": "output"}]}}}}

    def idle(self) -> bool:
        with self.lock:
            self.advance(self.now())
            return not self._running() and not self._queued()

    def seconds_for_nobody(self, left_at: dict[int, float]) -> float:
        """Seconds that jobs of calls that left ran after the call left. left_at: call number -> time."""
        total = 0.0
        with self.lock:
            self.advance(self.now())
            for j in self.jobs.values():
                call = (j["seed"] or 0) - SEED_BASE
                if call in left_at and j["start"] is not None:
                    total += max(0.0, j["end"] - max(j["start"], left_at[call]))
        return total


class Server(ThreadingHTTPServer):
    request_queue_size = 128  # the default of 5 refuses some of 12 connections that arrive together
    daemon_threads = True


def make_handler(fake: FakeComfy):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *a):
            pass

        def _send(self, obj, code=200, raw: bytes | None = None, ctype="application/json"):
            body = raw if raw is not None else json.dumps(obj).encode()
            self.send_response(code)
            self.send_header("Content-Type", ctype)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self):
            p = self.path
            if p == "/system_stats":
                return self._send({"system": {"comfyui_version": "fake"}})
            if p == "/object_info":
                return self._send(FIXTURE)
            if p.startswith("/history/"):
                return self._send(fake.history(p.split("/")[-1]))
            if p == "/queue":
                return self._send(fake.queue())
            if p.startswith("/view"):
                return self._send(None, raw=tiny_png(), ctype="image/png")
            return self._send({"error": "not found"}, 404)

        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", 0))) or b"{}")
            if self.path == "/prompt":
                pid, number = fake.submit(body["prompt"])
                return self._send({"prompt_id": pid, "number": number, "node_errors": {}})
            if self.path == "/interrupt":
                fake.interrupt(body.get("prompt_id"))
                return self._send({})
            if self.path == "/queue":
                fake.delete(body.get("delete", []))
                return self._send({})
            if self.path == "/free":
                return self._send({})
            return self._send({"error": "not found"}, 404)

    return Handler


# The process of each session, by call number. The tool must end a server process hard, and
# stdio_client does not give its process back, so a spy on the spawn function records it.
PROCS: dict[int, object] = {}
_spawn = mcp_stdio._create_platform_compatible_process


async def _spy_spawn(command, args, env=None, errlog=sys.stderr, cwd=None):
    process = await _spawn(command, args, env, errlog, cwd)
    if env and env.get("HORUS_PEAK_CALL"):
        PROCS[int(env["HORUS_PEAK_CALL"])] = process
    return process


mcp_stdio._create_platform_compatible_process = _spy_spawn


def kill_hard(process) -> None:
    """End the process and its children at once, with no chance to clean up."""
    if os.name == "nt":
        subprocess.run(["taskkill", "/F", "/T", "/PID", str(process.pid)], capture_output=True)
    else:
        os.kill(process.pid, signal.SIGKILL)


async def one_call(n: int, a, params: StdioServerParameters, out: Path, barrier: asyncio.Barrier,
                   result: dict, errlog) -> None:
    """One agent: start a session, wait for the others, call generate_image once."""
    try:
        async with stdio_client(params, errlog=errlog) as (read, write):
            async with ClientSession(read, write) as s:
                await s.initialize()
                await barrier.wait()
                await asyncio.sleep((n - 1) * CALL_GAP)
                result["start"] = time.monotonic()
                try:
                    res = await asyncio.wait_for(
                        s.call_tool("generate_image", {"workflow": "it-illustration", "subject": SUBJECT,
                                                       "out_path": str(out), "seed": SEED_BASE + n}),
                        timeout=GENERATE_TIMEOUT / a.scale + QUEUE_TIMEOUT / a.scale + 120)
                except asyncio.TimeoutError:
                    result.update(kind="no_answer", end=time.monotonic())
                    return
                result["end"] = time.monotonic()
                env = env_of(res)
                if env.get("ok") and out.is_file() and out.read_bytes()[:8] == b"\x89PNG\r\n\x1a\n":
                    result["kind"] = "png"
                elif env.get("ok"):
                    result["kind"] = "ok_without_file"
                else:
                    result["kind"] = (env.get("error") or {}).get("code", "UNKNOWN_ERROR")
                    result["message"] = str((env.get("error") or {}).get("message", ""))[:160]
    except asyncio.CancelledError:
        raise
    except BaseException as exc:  # the server process ended, or it never started
        result.setdefault("end", time.monotonic())
        result["kind"] = "left" if result.get("left") else f"session_error: {type(exc).__name__}: {exc}"[:200]
        if not result.get("left"):
            barrier.abort()


async def leaver(n: int, delay: float, result: dict, left_at: dict[int, float], fake: FakeComfy) -> None:
    """End the server process of call n hard, delay seconds after the calls started."""
    while "start" not in result:  # the clock starts when the call starts
        await asyncio.sleep(0.05)
    await asyncio.sleep(max(0.0, result["start"] + delay - time.monotonic()))
    if "kind" in result or n not in PROCS:  # the call already ended: no agent leaves
        return
    result["left"] = True
    left_at[n] = fake.now()
    fake.write_log(f"agent of call {n} leaves: server process {PROCS[n].pid} ended")
    kill_hard(PROCS[n])


def wait_ready(stack: str, within: float) -> None:
    """Wait until `docker exec <stack>-api horus ready` exits 0: the stack reaches the fake as its ComfyUI."""
    until, last = time.monotonic() + within, ""
    while True:
        r = subprocess.run(["docker", "exec", f"{stack}-api", "horus", "ready"], capture_output=True, text=True)
        if r.returncode == 0:
            return
        last = (r.stdout + r.stderr).strip()
        if time.monotonic() > until:
            raise SystemExit(f"the stack {stack} is not ready after {within:g} s: {last}")
        time.sleep(1)


async def run(a, fake: FakeComfy, port: int, tmp: Path) -> dict:
    sandbox = make_sandbox(tmp)
    if a.target == "host":
        cmd = [resolve_program(a.server_cmd[0]), *a.server_cmd[1:]]
    else:
        cmd = ["docker", "exec", "-i", f"{a.stack}-api", "horus", "mcp"]
        wait_ready(a.stack, a.ready_timeout)
    env = {
        "HORUS_WORKFLOWS_DIR": sandbox["workflows"], "HORUS_COMFYUI_DIR": sandbox["comfy"],
        "HORUS_MODELS_DIR": sandbox["models"], "HORUS_TEXT_ENCODER_DIR": str(Path(sandbox["models"]) / "text_encoders"),
        "HORUS_COMFYUI_URL": f"http://127.0.0.1:{port}", "HORUS_AUTOSTART": "0",
        "HORUS_OUTPUT_DIR": str(tmp / "out"),
        "HORUS_GENERATE_TIMEOUT": str(round(GENERATE_TIMEOUT / a.scale)),
        "HORUS_QUEUE_TIMEOUT": str(round(QUEUE_TIMEOUT / a.scale)),
        "HORUS_RUN_ESTIMATE": str(round(RUN_ESTIMATE / a.scale)),
        "PYTHONUTF8": "1",
    }
    out_dir = Path(a.out_dir) if a.out_dir else tmp / "images"
    leave = {int(x) for x in a.leave.split(",") if x.strip()}
    results = {n: {} for n in range(1, a.calls + 1)}
    left_at: dict[int, float] = {}
    barrier = asyncio.Barrier(a.calls)
    errlogs = [open(tmp / f"server-{n}.stderr.log", "w", encoding="utf-8") for n in results]
    tasks = []
    for n in results:
        if a.target == "host":
            params = StdioServerParameters(command=cmd[0], args=cmd[1:], env={**env, "HORUS_PEAK_CALL": str(n)})
        else:
            args = [x for k, v in env.items() if k.startswith("HORUS_") and k not in
                    ("HORUS_COMFYUI_URL", "HORUS_COMFYUI_DIR", "HORUS_MODELS_DIR", "HORUS_TEXT_ENCODER_DIR",
                     "HORUS_WORKFLOWS_DIR", "HORUS_OUTPUT_DIR") for x in ("-e", f"{k}={v}")]
            params = StdioServerParameters(command=cmd[0], args=[*cmd[1:3], *args, *cmd[3:]],
                                           env={"HORUS_PEAK_CALL": str(n)})
        tasks.append(asyncio.create_task(one_call(n, a, params, out_dir / f"call-{n}.png", barrier,
                                                  results[n], errlogs[n - 1])))
    leavers = [asyncio.create_task(leaver(n, a.leave_at / a.scale, results[n], left_at, fake)) for n in sorted(leave) if n in results]
    await asyncio.gather(*tasks, return_exceptions=True)
    # Jobs that nobody waits for still run in the fake. Wait until the fake is idle.
    drain_until = time.monotonic() + a.calls * fake.job_seconds + 60
    while not fake.idle() and time.monotonic() < drain_until:
        await asyncio.sleep(0.5)
    for t in leavers:
        t.cancel()
    for f in errlogs:
        f.close()

    kinds = [r.get("kind", "no_answer") for r in results.values()]
    errors = [r["end"] - r["start"] for r in results.values()
              if r.get("kind") not in (None, "png", "left") and "start" in r and "end" in r]
    for n, r in results.items():
        wait = f"{r['end'] - r['start']:.1f}s" if "start" in r and "end" in r else "-"
        print(f"call {n:2d}: {r.get('kind', 'no_answer'):<22} wait {wait} {r.get('message', '')}", file=sys.stderr)
    print(f"fake: POST /prompt {fake.posts_prompt}, calls {a.calls}, left {sorted(left_at)}", file=sys.stderr)
    return {
        "png": kinds.count("png"),
        "generation_timeout": kinds.count("GENERATION_TIMEOUT"),
        "queue_full": kinds.count("QUEUE_FULL"),
        "wait_for_error_seconds": round(max(errors, default=0.0), 1),
        "gpu_seconds_for_nobody": round(fake.seconds_for_nobody(left_at), 1),
        "max_jobs_in_comfyui": fake.max_held,
        "max_running": fake.max_running,
    }


def main() -> int:
    ap = argparse.ArgumentParser(description="Many agents call generate_image at the same time.")
    ap.add_argument("--target", choices=["host", "stack"], default="host")
    ap.add_argument("--server-cmd", nargs="+", default=["horus", "mcp"], help="the program and its arguments (host)")
    ap.add_argument("--stack", default="horus_test", help="the compose project name (stack)")
    ap.add_argument("--calls", type=int, default=12)
    ap.add_argument("--parallel", type=int, default=1, help="jobs that the fake ComfyUI runs at one time")
    ap.add_argument("--scale", type=float, default=10, help="divides every time")
    ap.add_argument("--leave", default="", help="call numbers whose agent leaves, for example 3,6")
    ap.add_argument("--leave-at", type=float, default=60, help="seconds before --scale after which they leave")
    ap.add_argument("--log", default="", help="the log of the requests of the fake")
    ap.add_argument("--port", type=int, default=0, help="the port of the fake (default: a free port)")
    ap.add_argument("--listen", default="127.0.0.1", help="the address of the fake (Docker on Linux: 0.0.0.0)")
    ap.add_argument("--out-dir", default="", help="the folder of the PNG files (stack: a shared folder; default: a temporary folder)")
    ap.add_argument("--ready-timeout", type=float, default=180, help="stack: the most seconds to wait until horus ready exits 0")
    a = ap.parse_args()
    with tempfile.TemporaryDirectory(prefix="horus-peak-") as td:
        tmp = Path(td)
        log = Path(a.log) if a.log else tmp / "fake.log"
        fake = FakeComfy(a.parallel, JOB_SECONDS / a.scale, log)
        httpd = Server((a.listen, a.port), make_handler(fake))
        threading.Thread(target=httpd.serve_forever, daemon=True).start()
        port = httpd.server_address[1]
        print(f"fake ComfyUI on port {port}; one job takes {fake.job_seconds:g} s; log {log}", file=sys.stderr)
        line = asyncio.run(run(a, fake, port, tmp))
        httpd.shutdown()
        fake.log.close()
    print(json.dumps(line))
    return 0


if __name__ == "__main__":
    sys.exit(main())

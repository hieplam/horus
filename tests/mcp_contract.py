#!/usr/bin/env python3
"""Contract test of the horus MCP server. No GPU, no network, no real model files.

    python tests/mcp_contract.py [--server-cmd <exe> ...] [--report <file>]      (Python 3.12 with mcp==2.3.0, and Go)
    python tests/mcp_contract.py --stack <name> --port <port> --shared-dir <folder> --models-dir <folder>
                                 --workflows-dir <folder> [--http <url>]

The test starts the server over stdio, like an agent harness does, and talks to
it with the official MCP client. A fake ComfyUI (a small HTTP server in this
process) answers the backend calls. A temporary folder holds a copy of the
it-illustration workflow, tiny stand-in model files, a second workflow
whose files are absent, and the stand-in speech workflow. The speech checks use
the fake speech worker (internal/speechworker/fakeworker): the test builds it
with `go build` as the Python of a sandbox speech venv, and puts a fake ffmpeg
and ffprobe on the PATH of the server. The setup checks run install_workflow on
a machine without ComfyUI, and on a machine without the speech setup, with a
fake package manager on a PATH that holds only fakes and the folders of the OS.

It prints one PASS or FAIL line per check and "contract: N of M passed".
Exit code 0 only when all checks pass. The default server command is
`horus mcp`, with `horus` found on the PATH. The list of error codes comes from
`<server program> errors -json`. --report writes each check, and the answers of
the server to the speech checks, as JSON (the goal suite reads it).

--stack runs the checks of the stack over stdio into the stack (`docker exec -i
<name>-api horus mcp`). With --http <url> too, the same checks go over
Streamable HTTP to /mcp of horus api at <url>, with the bearer token of the
environment variable HORUS_API_TOKEN. The test never prints the token. The
test stack mounts two folders of the test into horus-api: --models-dir as
HORUS_MODELS_DIR (horus stack-mounts) and --workflows-dir as
HORUS_WORKFLOWS_DIR (tests/stack-sandbox.yaml). The test writes the same two
workflows and stand-in model files there as on the host. It refuses a models
folder with a file that it did not write, so it never changes real model files.
"""

from __future__ import annotations

import argparse
import asyncio
import hashlib
import json
import os
import shutil
import struct
import socket
import subprocess
import sys
import tempfile
import threading
import time
import zlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import httpx2
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client
from mcp.client.streamable_http import streamable_http_client

ROOT = Path(__file__).resolve().parent.parent
FIXTURE = json.loads((ROOT / "tests" / "fixtures" / "object_info.json").read_text(encoding="utf-8"))
SUBJECT = ("On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt. "
           "On the right, a sky-blue server box takes them off the belt.")
EXPECTED_TOOLS = {"list_workflows", "describe_workflow", "install_workflow", "uninstall_workflow", "generate_image", "generate_speech"}
ERROR_KEYS = {"code", "message", "retryable", "action_by", "next_step", "details"}
# The speech checks. The stand-in speech workflow has the voices alpha and beta, the tags [laugh] and [sigh], and a text
# of 2 to 500 characters. The fake speech worker answers with the text as the transcript.
SPEECH = "stand-in-speech"
SPEECH_TEXT = "Xin chào  [laugh] các bạn, hôm nay chúng ta nói về hàng đợi."
SPEECH_ARGUMENTS = {"workflow", "text", "voice", "out_path", "overwrite"}
# Words that an argument of generate_speech must not have: the caller gives only a text and a voice (goal row speech-hidden).
SETUP_WORDS = ("engine", "python", "ffmpeg", "ffprobe", "loudness", "lufs", "venv", "vieneu", "whisper")


def tiny_png() -> bytes:
    def chunk(kind: bytes, data: bytes) -> bytes:
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data) & 0xFFFFFFFF)
    raw = b"\x00\xff\xff\xff"
    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 1, 1, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(raw)) + chunk(b"IEND", b""))


class Fake:
    """State of the fake ComfyUI."""
    mode = "ok"            # ok | oom | slow
    prompts: dict[str, dict] = {}
    polls: dict[str, int] = {}
    cancelled: list[str] = []
    counter = 0
    lock = threading.Lock()


class Handler(BaseHTTPRequestHandler):
    # HTTP/1.1 keeps each connection open for the next request. With HTTP/1.0, each request that a container sends
    # through host.docker.internal opens a new connection, and some of them hang until their time limit
    # (tests/peak_load.py measured this). Each answer has a Content-Length, so a connection can stay open.
    protocol_version = "HTTP/1.1"

    def log_message(self, *a):  # quiet
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
            pid = p.split("/")[-1]
            with Fake.lock:
                if pid not in Fake.prompts:
                    return self._send({})
                Fake.polls[pid] = Fake.polls.get(pid, 0) + 1
                mode, n = Fake.prompts[pid]["mode"], Fake.polls[pid]
            if mode == "slow" or n < 2:
                return self._send({})
            if mode == "oom":
                return self._send({pid: {"status": {"status_str": "error", "completed": False, "messages": [
                    ["execution_start", {"timestamp": 1}],
                    ["execution_error", {"node_type": "KSampler", "exception_type": "torch.OutOfMemoryError",
                                         "exception_message": "CUDA out of memory. Tried to allocate 2.00 GiB"}]]},
                    "outputs": {}}})
            return self._send({pid: {"status": {"status_str": "success", "completed": True, "messages": [
                ["execution_start", {"timestamp": 1000}], ["execution_success", {"timestamp": 4500}]]},
                "outputs": {"9": {"images": [{"filename": f"{pid}.png", "subfolder": "", "type": "output"}]}}}})
        if p == "/queue":
            with Fake.lock:
                running = [[0, pid] for pid, v in Fake.prompts.items() if pid not in Fake.cancelled and v["mode"] == "slow"]
            return self._send({"queue_running": running, "queue_pending": []})
        if p.startswith("/view"):
            return self._send(None, raw=tiny_png(), ctype="image/png")
        return self._send({"error": "not found"}, 404)

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", 0))) or b"{}")
        if self.path == "/prompt":
            with Fake.lock:
                Fake.counter += 1
                pid = f"job-{Fake.counter}"
                Fake.prompts[pid] = {"mode": Fake.mode, "graph": body["prompt"]}
            return self._send({"prompt_id": pid, "number": Fake.counter, "node_errors": {}})
        if self.path == "/interrupt":
            Fake.cancelled.append(body.get("prompt_id", "?"))
            return self._send({})
        if self.path == "/queue":
            Fake.cancelled.extend(body.get("delete", []))
            return self._send({})
        if self.path == "/free":
            return self._send({})
        return self._send({"error": "not found"}, 404)


class Server(ThreadingHTTPServer):
    """The server of the fake ComfyUI. HTTP/1.1 keeps a connection open, and the handler thread of an open connection
    answers after shutdown(), so stop() also closes each open connection: a client then gets "connection refused"."""
    request_queue_size = 128  # the default of 5 refuses some connections that arrive together

    def __init__(self, *a, **kw) -> None:
        self.open_connections: set = set()
        self.connections_lock = threading.Lock()
        super().__init__(*a, **kw)

    def get_request(self):
        sock, addr = super().get_request()
        with self.connections_lock:
            self.open_connections.add(sock)
        return sock, addr

    def shutdown_request(self, request) -> None:
        with self.connections_lock:
            self.open_connections.discard(request)
        super().shutdown_request(request)

    def stop(self) -> None:
        self.shutdown()
        self.server_close()
        with self.connections_lock:
            socks = list(self.open_connections)
        for sock in socks:
            try:
                sock.shutdown(socket.SHUT_RDWR)
            except OSError:
                pass


def make_sandbox(tmp: Path, with_speech: bool = False) -> dict[str, str]:
    """Workflows: it-illustration (installed, tiny stand-in files), absent-style (no files) and
    stand-in-speech (a manifest of the kind speech, no files; only with with_speech)."""
    wf_root, models_root = tmp / "workflows", tmp / "comfy" / "ComfyUI" / "models"
    write_image_workflows(wf_root, models_root)
    (tmp / "comfy" / "ComfyUI" / "main.py").write_text("# fake\n", encoding="utf-8")
    out = {"workflows": str(wf_root), "comfy": str(tmp / "comfy"), "models": str(models_root)}
    if with_speech:
        out.update(make_speech_setup(tmp, wf_root))
    return out


# The first bytes of each stand-in model file. The run on the stack refuses a models folder with another file.
STAND_IN = b"stand-in for "
# The two image workflows of the sandbox, and the stand-in model file that a check removes and writes again.
IMAGE_WORKFLOWS = ("it-illustration", "absent-style")
REMOVED = ("vae", "flux2-vae.safetensors")


def write_image_workflows(wf_root: Path, models_root: Path) -> None:
    """it-illustration with a stand-in file of each of its 3 model files in models_root, and absent-style, whose
    diffusion model is absent."""
    src = ROOT / "workflows" / "it-illustration"
    dst = wf_root / "it-illustration"
    shutil.copytree(src, dst, ignore=shutil.ignore_patterns("examples"))
    models = json.loads((src / "models.json").read_text(encoding="utf-8"))
    for f in models["files"]:
        data = STAND_IN + f["repo_path"].encode()
        target = models_root / f["folder"] / f["repo_path"].split("/")[-1]
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
        f["bytes"], f["sha256"] = len(data), hashlib.sha256(data).hexdigest()
    (dst / "models.json").write_text(json.dumps(models, indent=1), encoding="utf-8")

    # A second style whose model file is absent. Its graph is a test copy, built in code.
    absent = wf_root / "absent-style"
    shutil.copytree(dst, absent)
    graph = json.loads((absent / "workflow.json").read_text(encoding="utf-8"))
    for n in graph["nodes"]:
        if n["type"] == "UNETLoader":
            n["widgets_values"][0] = "absent-model.safetensors"
    (absent / "workflow.json").write_text(json.dumps(graph), encoding="utf-8")
    m = json.loads((absent / "manifest.json").read_text(encoding="utf-8"))
    m["name"], m["title"] = "absent-style", "Absent style"
    (absent / "manifest.json").write_text(json.dumps(m), encoding="utf-8")
    am = json.loads((absent / "models.json").read_text(encoding="utf-8"))
    am["files"][0]["repo_path"] = "diffusion_models/absent-model.safetensors"
    am["files"][0]["bytes"], am["files"][0]["sha256"] = 5_000_000_000, "0" * 64
    (absent / "models.json").write_text(json.dumps(am), encoding="utf-8")


def stand_in(models_root: Path, folder: str, name: str) -> Path:
    """Write the stand-in model file folder/name of it-illustration again, as write_image_workflows writes it."""
    models = json.loads((ROOT / "workflows" / "it-illustration" / "models.json").read_text(encoding="utf-8"))
    f = next(f for f in models["files"] if f["folder"] == folder and f["repo_path"].endswith("/" + name))
    target = models_root / folder / name
    target.write_bytes(STAND_IN + f["repo_path"].encode())
    return target


def make_stack_sandbox(workflows: Path, models: Path) -> dict[str, str]:
    """The sandbox of the run on the stack, in the two folders that the test stack mounts into horus-api: the same
    two image workflows as on the host, and their stand-in model files. The stack has no speech workflow. It refuses a
    models folder with a file that is not a stand-in or the mount marker, and a workflows folder with another entry,
    so a wrong folder (the real models of a machine) is never changed."""
    for f in (p for p in models.rglob("*") if p.is_file()):
        if f.name != MARKER and f.read_bytes()[:len(STAND_IN)] != STAND_IN:
            raise SystemExit(f"--models-dir {models} holds {f}, which this test did not write: give an empty test folder")
    for d in workflows.iterdir() if workflows.is_dir() else []:
        if d.name not in IMAGE_WORKFLOWS:
            raise SystemExit(f"--workflows-dir {workflows} holds {d}, which this test did not write: give an empty test folder")
        shutil.rmtree(d)
    write_image_workflows(workflows, models)
    if os.name != "nt":  # the user of the containers (uid 10001) writes and deletes model files there
        for d in [models, *(p for p in models.rglob("*") if p.is_dir())]:
            d.chmod(0o777)
    return {"workflows": str(workflows), "models": str(models)}


# The file that the run on the stack writes into the models folder to see that horus-api mounts that folder.
MARKER = ".horus-contract-mount"


def models_mounted(stack: str, models: Path) -> tuple[bool, str]:
    """True when the folder HORUS_MODELS_DIR of <stack>-api is the models folder of the test: a new random marker
    file that the test writes there can be read in the container."""
    token = os.urandom(8).hex()
    marker = models / MARKER
    marker.write_text(token, encoding="utf-8")
    try:
        out = subprocess.run(["docker", "exec", f"{stack}-api", "sh", "-c", f'cat "$HORUS_MODELS_DIR/{MARKER}"'],
                             capture_output=True, text=True, timeout=60)
    except (OSError, subprocess.SubprocessError) as exc:
        return False, str(exc)
    finally:
        marker.unlink(missing_ok=True)
    return out.returncode == 0 and out.stdout.strip() == token, (out.stdout + out.stderr).strip()[:200]


def same_path(a: str, b: str) -> bool:
    return os.path.normcase(os.path.normpath(a)) == os.path.normcase(os.path.normpath(b))


def make_speech_setup(tmp: Path, wf_root: Path) -> dict[str, str]:
    """The stand-in speech workflow with one stand-in model file, and a speech folder (HORUS_SPEECH_DIR) with each
    part of the speech setup: the venv whose Python is the fake speech worker, and the model file. A folder of fake
    programs holds ffmpeg and ffprobe; the fake worker runs neither."""
    speech = wf_root / SPEECH
    speech.mkdir()
    shutil.copy(ROOT / "internal" / "catalog" / "testdata" / "stand-in-speech" / "manifest.json", speech / "manifest.json")
    data = b"stand-in speech model"
    models = {"repo": "stand-in/speech", "license": "Apache-2.0", "files": [
        {"role": "speech", "repo_path": "model.bin", "folder": "tts", "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}]}
    (speech / "models.json").write_text(json.dumps(models, indent=1), encoding="utf-8")
    speech_dir = tmp / "speech"
    model = speech_dir / "models" / "tts" / "model.bin"
    model.parent.mkdir(parents=True)
    model.write_bytes(data)
    python = speech_dir / "venv" / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
    python.parent.mkdir(parents=True)
    try:
        build = subprocess.run(["go", "build", "-o", str(python), "./internal/speechworker/fakeworker"], cwd=ROOT,
                               capture_output=True, text=True, timeout=600)
        if build.returncode != 0:
            print(f"the fake speech worker did not build: {build.stdout}{build.stderr}", flush=True)
    except (OSError, subprocess.SubprocessError) as exc:
        print(f"the fake speech worker did not build (Go is needed): {exc}", flush=True)
    bindir = tmp / "speech-bin"
    bindir.mkdir()
    for name in ("ffmpeg", "ffprobe"):
        fake_program(bindir, name, tmp / f"{name}.args")
    return {"speech_dir": str(speech_dir), "speech_model": str(model), "speech_bin": str(bindir)}


class Checks:
    def __init__(self) -> None:
        self.rows: list[tuple[bool, str, str]] = []
        self.keys: list[str] = []          # the key of each row; "" for a row without a key
        self.answers: dict[str, list] = {}  # key -> the errors that the server gave in that check
        self.speech_schema: dict = {}       # the input schema of generate_speech, as the server gave it

    def check(self, name: str, ok: bool, detail: str = "", key: str = "", answers: list | None = None) -> None:
        self.rows.append((bool(ok), name, detail))
        self.keys.append(key)
        if key and answers is not None:
            self.answers[key] = answers
        print(f"{'PASS' if ok else 'FAIL'}  {name}{'' if ok else '  -- ' + detail}", flush=True)

    def report(self, total: int) -> dict:
        return {"total": total, "passed": sum(1 for ok, _, _ in self.rows if ok),
                "checks": [{"key": k, "name": n, "ok": ok, "detail": d} for (ok, n, d), k in zip(self.rows, self.keys)],
                "speech": {"input_schema": self.speech_schema, "answers": self.answers}}


def env_of(res) -> dict:
    return res.structured_content or json.loads(res.content[0].text)


def without_seconds(env: dict) -> dict:
    """The envelope without result.seconds, which changes from call to call."""
    return {**env, "result": {k: v for k, v in (env.get("result") or {}).items() if k != "seconds"}}


def resolve_program(program: str) -> str:
    """A path that exists becomes an absolute path; a bare name is looked up on the PATH.
    A program that is not found stays as written, so the run fails."""
    if os.sep in program or "/" in program:
        return str(Path(program).resolve()) if Path(program).is_file() else program
    return shutil.which(program) or program


async def run(server_cmd: list[str], sandbox: dict[str, str], port: int, tmp: Path, c: Checks) -> None:
    server_cmd = [resolve_program(server_cmd[0]), *server_cmd[1:]]  # one program for the session and for `errors -json`
    env = {
        "HORUS_WORKFLOWS_DIR": sandbox["workflows"], "HORUS_COMFYUI_DIR": sandbox["comfy"],
        "HORUS_MODELS_DIR": sandbox["models"], "HORUS_TEXT_ENCODER_DIR": str(Path(sandbox["models"]) / "text_encoders"),
        "HORUS_COMFYUI_URL": f"http://127.0.0.1:{port}", "HORUS_AUTOSTART": "0",
        "HORUS_OUTPUT_DIR": str(tmp / "out"), "HORUS_GENERATE_TIMEOUT": "12",
        "PYTHONUTF8": "1",
        "HORUS_SPEECH_DIR": sandbox["speech_dir"], "HORUS_FAKE_SPEECH_RECORD": str(tmp / "speech-worker.record"),
        "PATH": os.pathsep.join([sandbox["speech_bin"], os.environ.get("PATH", "")]),
    }
    params = StdioServerParameters(command=server_cmd[0], args=server_cmd[1:], env=env)
    with open(tmp / "server.stderr.log", "w", encoding="utf-8") as errlog:
        await session_checks(lambda: stdio_client(params, errlog=errlog), sandbox, tmp, c)
    await speech_worker_checks(server_cmd, env, tmp, c)
    await setup_checks(server_cmd, env, tmp, c)
    await speech_setup_checks(server_cmd, env, sandbox, tmp, c)

    neutral_checks(c)
    code_table_checks([server_cmd[0], "errors", "-json"], c)


def code_table_checks(errors_cmd: list[str], c: Checks) -> None:
    """The checks of the table of error codes: errors_cmd is the command that prints the table as JSON (the server
    program on the host, `docker exec <stack>-api horus` in the stack)."""
    table: dict = {}
    try:  # the doc table must list every code; the server program prints the code table
        out = subprocess.run(errors_cmd, capture_output=True, timeout=60).stdout
        table = json.loads(out)
        ERRORS = list(table) or ["<no code in errors -json>"]
    except (OSError, ValueError, subprocess.SubprocessError) as exc:
        ERRORS = [f"<{' '.join(errors_cmd)} failed: {exc}>"]
    doc = (ROOT / "docs" / "mcp.md").read_text(encoding="utf-8") if (ROOT / "docs" / "mcp.md").is_file() else ""
    missing = [code for code in ERRORS if f"`{code}`" not in doc]
    c.check("docs/mcp.md lists every error code", not missing, f"missing: {missing}")
    # The code of a call while a dependency of the stack is down (spec 2026-11-19-job-queue.md, "MCP contract").
    nr = table.get("NOT_READY") or {}
    c.check("the code table has NOT_READY: retryable, action_by=human, next_step names details.down",
            nr.get("retryable") is True and nr.get("action_by") == "human" and "details.down" in nr.get("next_step", ""),
            json.dumps(nr))
    # The codes of a call through the job queue of the stack.
    qf = table.get("QUEUE_FULL") or {}
    c.check("the code table has QUEUE_FULL: retryable, action_by=retry, next_step names details.retry_after_seconds",
            qf.get("retryable") is True and qf.get("action_by") == "retry" and "details.retry_after_seconds" in qf.get("next_step", ""),
            json.dumps(qf))
    wl = table.get("WORKER_LOST") or {}
    c.check("the code table has WORKER_LOST: retryable, action_by=retry",
            wl.get("retryable") is True and wl.get("action_by") == "retry", json.dumps(wl))


def wait_ready(stack: str, within: float) -> None:
    """Wait until `docker exec <stack>-api horus ready` exits 0: the 4 dependencies are up, and the stack reaches the fake
    ComfyUI of this process."""
    until, last = time.monotonic() + within, ""
    while True:
        r = subprocess.run(["docker", "exec", f"{stack}-api", "horus", "ready"], capture_output=True, text=True)
        if r.returncode == 0:
            return
        last = (r.stdout + r.stderr).strip()
        if time.monotonic() > until:
            raise SystemExit(f"the stack {stack} is not ready after {within:g} s: {last}")
        time.sleep(1)


async def run_stack(stack: str, shared: Path, sandbox: dict[str, str], tmp: Path, c: Checks, http_url: str = "") -> None:
    """The contract checks over stdio into the stack: the server is `horus mcp` in the container <stack>-api. With
    http_url, the same checks go over Streamable HTTP to /mcp of horus api, with the bearer token of HORUS_API_TOKEN.
    The fake ComfyUI of this process is the ComfyUI of the stack. The checks that need the sandbox of the host
    (STACK_SKIPPED) are left out."""
    if http_url:
        # The read time limit is longer than the gap between two progress messages of a long call (5 s).
        headers = {"Authorization": "Bearer " + os.environ.get("HORUS_API_TOKEN", "")}
        async with httpx2.AsyncClient(headers=headers, timeout=httpx2.Timeout(30, read=300)) as client:
            await session_checks(lambda: streamable_http_client(http_url, http_client=client), sandbox, tmp, c, stack=stack, shared=shared)
    else:
        params = StdioServerParameters(command="docker", args=["exec", "-i", f"{stack}-api", "horus", "mcp"])
        with open(tmp / "server.stderr.log", "w", encoding="utf-8") as errlog:
            await session_checks(lambda: stdio_client(params, errlog=errlog), sandbox, tmp, c, stack=stack, shared=shared)
    code_table_checks(["docker", "exec", f"{stack}-api", "horus", "errors", "-json"], c)


STYLE_WORDS = ("palette", "pastel", "navy", "flat vector", "house style", "house-style", "dotted grid")


def neutral_checks(c: Checks) -> None:
    """Every shipped workflow is style-neutral: no theme, no style file. An image workflow also has no style words in
    its graph. A speech workflow has no graph, and the check says so."""
    for d in sorted(p for p in (ROOT / "workflows").iterdir() if (p / "manifest.json").is_file()):
        m = json.loads((d / "manifest.json").read_text(encoding="utf-8"))
        c.check(f"{d.name}: manifest has no theme and no caption style file",
                "theme" not in m and "caption" not in m and not list(d.glob("*style*.json")), json.dumps(m.get("theme") or m.get("caption")))
        if m.get("kind") == "speech":
            # A speech workflow has no ComfyUI graph, so there are no graph words to check. It must stay without one.
            c.check(f"{d.name}: a speech manifest has no graph, no subject and no output",
                    not ({"graph", "subject", "output"} & set(m)) and not (d / "workflow.json").exists(), str(sorted(m)))
            continue
        graph = json.loads((d / m["graph"]["file"]).read_text(encoding="utf-8"))
        texts = " ".join(str(v) for n in graph.get("nodes", []) for v in (n.get("widgets_values") or [])).lower()
        words = [w for w in STYLE_WORDS if w in texts]
        c.check(f"{d.name}: the graph holds no style words", not words, str(words))


async def session_checks(connect, sandbox: dict[str, str], tmp: Path, c: Checks, stack: str = "", shared: Path | None = None) -> None:
    """The checks of one MCP session. connect() gives the transport (stdio or Streamable HTTP). With stack (the compose
    project name), the server is horus in the stack and the checks that need the sandbox of the host are left out (see
    STACK_SKIPPED)."""
    async with connect() as (read, write):
        async with ClientSession(read, write) as s:
            init = await s.initialize()
            c.check("server identifies as horus and has instructions",
                    init.server_info.name == "horus" and bool(init.instructions), str(init.server_info))

            tools = {t.name: t for t in (await s.list_tools()).tools}
            c.check("tools/list has exactly the 6 tools", set(tools) == EXPECTED_TOOLS, str(sorted(tools)))
            schema = tools["generate_speech"].input_schema if "generate_speech" in tools else {}
            c.speech_schema = schema
            props = schema.get("properties", {})
            words = sorted({w for name, p in props.items() for w in SETUP_WORDS
                            if w in f"{name} {p.get('title', '')} {p.get('description', '')}".lower()})
            c.check("generate_speech takes only workflow, text, voice, out_path and overwrite; no argument names an engine, "
                    "Python, ffmpeg or the loudness", set(props) == SPEECH_ARGUMENTS and not words
                    and set(schema.get("required", [])) == {"workflow", "text"}, f"arguments {sorted(props)}, words {words}",
                    key="speech-hidden")
            c.check("every tool has an outputSchema with ok, result, error",
                    all(t.output_schema and {"ok", "result", "error"} <= set(t.output_schema.get("properties", {}))
                        for t in tools.values()), "")
            ann = tools.get("uninstall_workflow").annotations if "uninstall_workflow" in tools else None
            c.check("uninstall_workflow is marked destructive, list_workflows read-only",
                    bool(ann and ann.destructive_hint) and bool(tools["list_workflows"].annotations.read_only_hint), "")

            # The stack has the two image workflows of the sandbox, and no speech workflow (the speech setup is on the host).
            speech_row = {} if stack else {SPEECH: "installed"}
            res = env_of(await s.call_tool("list_workflows", {}))
            states = {w["name"]: w["install_state"] for w in (res.get("result") or {}).get("workflows", [])}
            # absent-style shares the text encoder and the VAE with it-illustration, so 2 of its 3 files are present.
            # The speech setup of the sandbox has each part of stand-in-speech, so it is installed.
            c.check("list_workflows shows installed and partially_installed styles",
                    res["ok"] and states == {"it-illustration": "installed", "absent-style": "partially_installed", **speech_row},
                    json.dumps(res)[:400])
            kinds = {w["name"]: w.get("kind") for w in (res.get("result") or {}).get("workflows", [])}
            c.check("list_workflows shows the kind of each workflow: a manifest without kind is image",
                    kinds == {"it-illustration": "image", "absent-style": "image", **({} if stack else {SPEECH: "speech"})}, str(kinds))
            if not stack:
                res = env_of(await s.call_tool("describe_workflow", {"workflow": "stand-in-speech"}))
                r = res.get("result") or {}
                c.check("describe_workflow of a speech workflow shows the voices, tags and formats, and no graph",
                        res["ok"] and r.get("kind") == "speech" and r.get("voices") == ["alpha", "beta"] and r.get("default_voice") == "alpha"
                        and r.get("tags") and r.get("formats") == ["wav", "mp3"] and r.get("text", {}).get("max_chars")
                        and r.get("output") is None and r.get("subject") is None and "graph" not in json.dumps(res), json.dumps(res)[:400])
                jobs = len(Fake.prompts)
                res = env_of(await s.call_tool("generate_image", {"workflow": "stand-in-speech", "subject": SUBJECT}))
                e = res.get("error") or {}
                c.check("generate_image with a speech workflow -> INVALID_ARGUMENT, action_by=caller, next_step names generate_speech",
                        e.get("code") == "INVALID_ARGUMENT" and e.get("action_by") == "caller" and "generate_speech" in e.get("next_step", "")
                        and e["details"].get("field") == "workflow" and len(Fake.prompts) == jobs, json.dumps(res)[:400])
                await speech_checks(s, sandbox, tmp, c)

            raw = await s.call_tool("describe_workflow", {"workflow": "it-illustration"})
            res = env_of(raw)
            text = raw.content[0].text
            r = res.get("result") or {}
            c.check("describe_workflow gives subject rules, examples and the 3 model files",
                    res["ok"] and len(r.get("subject", {}).get("rules", [])) >= 3 and r["subject"]["examples"]
                    and [f["state"] for f in r.get("files", [])] == ["present"] * 3,
                    json.dumps(res)[:400])
            listed = (env_of(await s.call_tool("list_workflows", {})).get("result") or {}).get("workflows", [])
            c.check("list_workflows and describe_workflow carry no theme or look",
                    not any(k in r for k in ("theme", "look")) and not any(k in w for w in listed for k in ("theme", "look")),
                    json.dumps(r)[:300])
            c.check("describe_workflow hides the ComfyUI graph (no node class names)",
                    not any(k in text for k in ("CLIPTextEncode", "KSampler", "class_type", "workflow.json")), "")

            res = env_of(err := await s.call_tool("describe_workflow", {"workflow": "nope"}))
            e = res.get("error") or {}
            c.check("unknown workflow -> WORKFLOW_NOT_FOUND with the known names, isError",
                    err.is_error and e.get("code") == "WORKFLOW_NOT_FOUND" and "it-illustration" in e.get("details", {}).get("known", [])
                    and set(e) == ERROR_KEYS, json.dumps(res)[:300])

            progress: list[str] = []

            async def on_progress(p, total, message):
                progress.append(message or "")

            # Each run on the stack writes its own file, so that the stdio run and the HTTP run can share one stack.
            out = (shared if stack else tmp / "images") / ("queue-http.png" if HTTP_RUN else "queue.png")
            res = env_of(await s.call_tool("generate_image", {"workflow": "it-illustration", "subject": SUBJECT,
                                                             "out_path": str(out), "seed": 7}, progress_callback=on_progress))
            r = res.get("result") or {}
            # backend_job_id is the id of the job in ComfyUI, on the host and in the stack.
            graph = Fake.prompts.get(r.get("backend_job_id", ""), {}).get("graph", {})
            caption = next((n["inputs"]["text"] for n in graph.values() if n["class_type"] == "CLIPTextEncode"), "")
            c.check("generate_image writes a PNG at out_path and returns its path, seed and backend_job_id",
                    res["ok"] and Path(r.get("path", "")) == out and out.read_bytes()[:8] == b"\x89PNG\r\n\x1a\n" and r.get("seed") == 7
                    and r.get("backend_job_id") in Fake.prompts and "queue_seconds" in r,
                    json.dumps(res)[:400])
            # horus is style-neutral: the caller owns the look, the palette and the design system.
            c.check(f"horus sends the subject to the model as written ({len(caption) - len(SUBJECT)} characters added)",
                    caption == SUBJECT, caption[:200])
            sampler = next((n["inputs"] for n in graph.values() if n["class_type"] == "KSampler"), {})
            c.check("seed 7 reaches the sampler of the graph", sampler.get("seed") == 7, str(sampler)[:200])
            c.check("generate_image sends progress notifications", len(progress) >= 1, str(progress))
            if stack:
                # The stack ran the call: the PNG is at the host path of the shared folder, the fake ComfyUI got the job
                # from the worker (the job row names the worker and the id of the job in the fake), and job_id is a horus
                # job id, not the id of the job in ComfyUI.
                row = job_row(stack, r.get("job_id", ""))
                c.check("the stack ran the call: the PNG is in the shared folder, the job row says that a worker ran it and "
                        "names the job that the fake ComfyUI got, and job_id is not the id of the job in ComfyUI",
                        res["ok"] and out.is_file() and row[:1] == ["succeeded"] and bool(row[1]) and row[2] == r.get("backend_job_id")
                        and r.get("job_id") != r.get("backend_job_id") and r.get("backend_job_id") in Fake.prompts,
                        f"row {row} {json.dumps(res)[:300]}")

            res = env_of(await s.call_tool("generate_image", {"workflow": "it-illustration", "subject": SUBJECT, "out_path": str(out)}))
            c.check("existing out_path -> OUTPUT_EXISTS", (res.get("error") or {}).get("code") == "OUTPUT_EXISTS", json.dumps(res)[:300])

            res = env_of(await s.call_tool("generate_image", {"workflow": "it-illustration", "subject": SUBJECT, "width": 1000}))
            e = res.get("error") or {}
            c.check("width 1000 -> INVALID_ARGUMENT field=width, action_by=caller",
                    e.get("code") == "INVALID_ARGUMENT" and e["details"].get("field") == "width" and e["action_by"] == "caller", json.dumps(res)[:300])

            res = env_of(await s.call_tool("generate_image", {"workflow": "it-illustration", "subject": "a box"}))
            e = res.get("error") or {}
            c.check("too short subject -> INVALID_ARGUMENT field=subject with the rules",
                    e.get("code") == "INVALID_ARGUMENT" and e["details"].get("field") == "subject" and e["details"].get("rules"), json.dumps(res)[:300])

            if not stack:  # absent-style is a copy of the sandbox
                res = env_of(await s.call_tool("generate_image", {"workflow": "absent-style", "subject": SUBJECT}))
                e = res.get("error") or {}
                c.check("style without model files -> WORKFLOW_NOT_INSTALLED naming the file",
                        e.get("code") == "WORKFLOW_NOT_INSTALLED" and "absent-model.safetensors" in e["details"].get("missing", []), json.dumps(res)[:300])

            Fake.mode = "oom"
            res = env_of(await s.call_tool("generate_image", {"workflow": "it-illustration", "subject": SUBJECT}))
            e = res.get("error") or {}
            c.check("GPU out of memory -> OUT_OF_MEMORY, retryable, action_by=caller",
                    e.get("code") == "OUT_OF_MEMORY" and e["retryable"] is True and e["action_by"] == "caller", json.dumps(res)[:300])

            Fake.mode = "slow"
            res = env_of(await s.call_tool("generate_image", {"workflow": "it-illustration", "subject": SUBJECT}))
            e = res.get("error") or {}
            job = e.get("details", {}).get("backend_job_id") or e.get("details", {}).get("job_id")  # the id in ComfyUI
            c.check("a job over the time limit -> GENERATION_TIMEOUT, and horus cancels it on the backend",
                    e.get("code") == "GENERATION_TIMEOUT" and job in Fake.cancelled, json.dumps(res)[:300] + f" cancelled={Fake.cancelled}")
            Fake.mode = "ok"

            # The model files are in the models folder of the sandbox: on the host the models folder of the server, in the
            # stack the test folder that horus stack-mounts mounts into horus-api as HORUS_MODELS_DIR. The stack never
            # installs ComfyUI: its plan holds only model files (spec 2026-11-19-job-queue.md, "MCP contract").
            models_root = Path(sandbox["models"])
            mounted = True
            if stack:
                mounted, why = models_mounted(stack, models_root)
                c.check("the stack mounts the models folder of the test into horus-api as HORUS_MODELS_DIR", mounted, why)
            res = env_of(await s.call_tool("install_workflow", {"workflow": "it-illustration"}))
            c.check("install of an installed style -> ok, already_installed",
                    res["ok"] and res["result"]["already_installed"] is True, json.dumps(res)[:300])

            installed = res
            res = env_of(await s.call_tool("install_workflow", {"workflow": "absent-style"}))
            e = res.get("error") or {}
            c.check("install without accept_license -> LICENSE_CONFIRMATION_REQUIRED, action_by=human, with size",
                    e.get("code") == "LICENSE_CONFIRMATION_REQUIRED" and e["action_by"] == "human" and e["details"].get("download_bytes", 0) > 0,
                    json.dumps(res)[:300])

            # The setup consent changes nothing on a machine where ComfyUI is installed.
            again = env_of(await s.call_tool("install_workflow", {"workflow": "it-illustration", "accept_setup": True}))
            lic = env_of(await s.call_tool("install_workflow", {"workflow": "absent-style", "accept_setup": True}))
            c.check("a machine with ComfyUI: accept_setup=true changes no answer of install_workflow",
                    without_seconds(again) == without_seconds(installed) and again["ok"] and lic == res,
                    json.dumps(again)[:200] + " " + json.dumps(lic)[:200])

            # One model file removed: the install state and the plan see it. The plan is the first call, before the
            # consent, so nothing is downloaded. Then the test writes the stand-in again.
            folder, name = REMOVED
            (models_root / folder / name).unlink(missing_ok=True)
            res = env_of(await s.call_tool("list_workflows", {}))
            states = {w["name"]: w["install_state"] for w in (res.get("result") or {}).get("workflows", [])}
            c.check("one model file removed -> list_workflows shows it-illustration partially_installed",
                    states.get("it-illustration") == "partially_installed", json.dumps(states))
            res = env_of(await s.call_tool("install_workflow", {"workflow": "it-illustration"}))
            e = res.get("error") or {}
            steps = (e.get("details") or {}).get("steps")
            one = steps[0] if isinstance(steps, list) and len(steps) == 1 else {}
            c.check("one model file removed -> the plan of install_workflow (before the consent) holds exactly that model "
                    "file step, in the models folder, and no setup step",
                    e.get("code") == "LICENSE_CONFIRMATION_REQUIRED" and one.get("kind") == "file" and one.get("name") == name
                    and same_path(one.get("dir") or "", str(models_root / folder)), json.dumps(res)[:500])
            stand_in(models_root, folder, name)

            model_files = list(models_root.rglob("*.safetensors"))
            if not mounted:  # never change a models folder that is not the folder of the test
                for what in ("uninstall without confirm", "uninstall with confirm", "uninstall with remove_shared", "after uninstall"):
                    c.check(f"{what}: not run, because horus-api does not mount the models folder of the test", False)
            else:
                states_of = lambda files: sorted(f["state"] for f in files)  # noqa: E731
                res = env_of(await s.call_tool("uninstall_workflow", {"workflow": "it-illustration"}))
                e = res.get("error") or {}
                c.check("uninstall without confirm -> CONFIRMATION_REQUIRED, plan: 1 to delete, 2 shared kept; nothing deleted",
                        e.get("code") == "CONFIRMATION_REQUIRED"
                        and states_of(e["details"].get("files", [])) == ["kept_shared", "kept_shared", "would_delete"]
                        and all(p.exists() for p in model_files), json.dumps(res)[:400])

                res = env_of(await s.call_tool("uninstall_workflow", {"workflow": "it-illustration", "confirm": True}))
                c.check("uninstall with confirm deletes the 1 unshared file and keeps the 2 shared files",
                        res["ok"] and states_of(res["result"]["files"]) == ["deleted", "kept_shared", "kept_shared"]
                        and sum(p.exists() for p in model_files) == 2, json.dumps(res)[:400])

                res = env_of(await s.call_tool("uninstall_workflow", {"workflow": "it-illustration", "confirm": True, "remove_shared": True}))
                c.check("uninstall with remove_shared deletes the 2 shared files",
                        res["ok"] and states_of(res["result"]["files"]) == ["absent", "deleted", "deleted"]
                        and not any(p.exists() for p in model_files), json.dumps(res)[:400])

                res = env_of(await s.call_tool("list_workflows", {}))
                states = {w["name"]: w["install_state"] for w in (res.get("result") or {}).get("workflows", [])}
                c.check("after uninstall, list_workflows shows both styles not_installed",
                        states == {"it-illustration": "not_installed", "absent-style": "not_installed", **speech_row}, json.dumps(states))

            res = env_of(await s.call_tool("generate_image", {"workflow": "nope-style", "subject": SUBJECT}))
            c.check("errors are the same envelope in structuredContent and in text",
                    res.get("ok") is False and res.get("result") is None and set(res.get("error", {})) == ERROR_KEYS, json.dumps(res)[:300])

            HTTPD.stop()
            res = env_of(await s.call_tool("generate_image", {"workflow": "it-illustration", "subject": SUBJECT}))
            if stack:
                e = res.get("error") or {}
                down = (e.get("details") or {}).get("down") or []
                c.check("ComfyUI down -> NOT_READY, action_by=human, details.down names comfyui (the stack never starts ComfyUI)",
                        e.get("code") == "NOT_READY" and e.get("action_by") == "human" and "comfyui" in down, json.dumps(res)[:300])
            else:
                e = res.get("error") or {}
                c.check("backend down and autostart off -> BACKEND_UNAVAILABLE, action_by=human",
                        e.get("code") == "BACKEND_UNAVAILABLE" and e["action_by"] == "human", json.dumps(res)[:300])


def job_row(stack: str, job_id: str) -> list[str]:
    """The state, the worker id and the backend job id of a row of horus.jobs, read in <stack>-db. An empty list when
    there is no row, or when docker or psql fails."""
    if not job_id or not all(ch.isalnum() or ch == "-" for ch in job_id):
        return []
    sql = f"select state, coalesce(worker_id, ''), coalesce(backend_job_id, '') from horus.jobs where id = '{job_id}'"
    try:
        out = subprocess.run(["docker", "exec", f"{stack}-db", "psql", "-U", "horus", "-d", "horus", "-At", "-F", "|", "-c", sql],
                             capture_output=True, text=True, timeout=60)
    except (OSError, subprocess.SubprocessError):
        return []
    line = out.stdout.strip()
    return line.split("|") if out.returncode == 0 and line else []


async def speech_states(s: ClientSession) -> tuple:
    """The install state of the stand-in speech workflow: from list_workflows, from describe_workflow, and the state of
    each file of describe_workflow."""
    listed = env_of(await s.call_tool("list_workflows", {}))
    described = env_of(await s.call_tool("describe_workflow", {"workflow": SPEECH}))
    from_list = {w["name"]: w["install_state"] for w in (listed.get("result") or {}).get("workflows", [])}.get(SPEECH)
    r = described.get("result") or {}
    return from_list, r.get("install_state"), [f.get("state") for f in r.get("files", [])]


def speech_record(tmp: Path) -> list[str]:
    """The lines of the record file of the fake speech worker: "start <args>" for each start, "<op> <id>" for each request."""
    p = tmp / "speech-worker.record"
    return p.read_text(encoding="utf-8").splitlines() if p.is_file() else []


def refusal(errs: list[dict], code: str, action_by: str) -> bool:
    """Each error has the code, the action_by, a next_step and the keys of an error."""
    return bool(errs) and all(e.get("code") == code and e.get("action_by") == action_by and e.get("next_step")
                              and set(e) == ERROR_KEYS for e in errs)


async def speech_checks(s: ClientSession, sandbox: dict[str, str], tmp: Path, c: Checks) -> None:
    """generate_speech with the fake speech worker: one file, a second call on the same worker, progress, and one check
    for each failure of the spec table "Failures" that the server finds before the worker runs. Those calls must not
    reach the worker: its record has no more speak requests after them."""
    progress: list[str] = []

    async def on_progress(p, total, message):
        progress.append(message or "")

    out = tmp / "speech-out" / "one.wav"
    res = env_of(await s.call_tool("generate_speech", {"workflow": SPEECH, "text": SPEECH_TEXT, "voice": "beta", "out_path": str(out)},
                                   progress_callback=on_progress))
    r = res.get("result") or {}
    data = out.read_bytes() if out.is_file() else b""
    c.check("generate_speech writes the audio file at out_path and returns the transcript of the worker (the text as written)",
            res["ok"] and Path(r.get("path", "")) == out and data[:4] == b"RIFF" and r.get("bytes") == len(data)
            and r.get("transcript") == SPEECH_TEXT and r.get("voice") == "beta" and r.get("format") == "wav"
            and r.get("audio_seconds", 0) > 0 and r.get("started_worker") is True, json.dumps(res)[:400], key="speech-file")
    c.check("generate_speech sends progress notifications", len(progress) >= 1, str(progress), key="speech-progress")

    states = await speech_states(s)
    c.check("list_workflows and describe_workflow show a speech workflow installed when each part of the speech setup is there",
            states == ("installed", "installed", ["present"]), str(states), key="speech-install-state:installed")

    res = env_of(await s.call_tool("generate_speech", {"workflow": SPEECH, "text": "Một câu nữa."}))
    r = res.get("result") or {}
    starts = [ln for ln in speech_record(tmp) if ln.startswith("start ")]
    default = Path(r.get("path", ""))
    c.check("a second call keeps the speech worker (started_worker false, 1 start), with the default voice and a new .wav in the output folder",
            res["ok"] and r.get("started_worker") is False and len(starts) == 1 and r.get("voice") == "alpha"
            and default.parent == tmp / "out" / SPEECH and default.suffix == ".wav" and default.is_file(),
            json.dumps(res)[:300] + f" starts={starts}", key="speech-worker-kept")

    def speaks() -> int:
        return sum(1 for ln in speech_record(tmp) if ln.startswith("speak "))

    async def errors(*calls: dict) -> list[dict]:
        got = []
        for args in calls:
            got.append(env_of(await s.call_tool("generate_speech", {"workflow": SPEECH, "text": SPEECH_TEXT, **args})).get("error") or {})
        return got

    def details(errs: list[dict], key: str) -> list:
        return [(e.get("details") or {}).get(key) for e in errs]

    before = speaks()
    errs = await errors({"text": ""}, {"text": "a"}, {"text": "x" * 501})
    c.check("an empty, too short or too long text -> INVALID_ARGUMENT field=text with the limits, action_by=caller",
            refusal(errs, "INVALID_ARGUMENT", "caller") and details(errs, "field") == ["text"] * 3
            and details(errs, "min_chars") == [2] * 3 and details(errs, "max_chars") == [500] * 3 and speaks() == before,
            json.dumps(errs)[:500], key="speech-failure:text-length", answers=errs)

    errs = await errors({"text": "Xin chào [cough] các bạn [laugh]."})
    c.check("a tag that the model does not perform -> INVALID_ARGUMENT with details.allowed_tags",
            refusal(errs, "INVALID_ARGUMENT", "caller") and details(errs, "allowed_tags") == [["[laugh]", "[sigh]"]]
            and details(errs, "found") == [["[cough]"]] and speaks() == before,
            json.dumps(errs)[:400], key="speech-failure:text-tag", answers=errs)

    errs = await errors({"voice": "gamma"})
    c.check("a voice that the workflow does not have -> INVALID_ARGUMENT field=voice with details.known",
            refusal(errs, "INVALID_ARGUMENT", "caller") and details(errs, "field") == ["voice"]
            and details(errs, "known") == [["alpha", "beta"]] and speaks() == before,
            json.dumps(errs)[:400], key="speech-failure:voice", answers=errs)

    errs = await errors({"out_path": "speech-out/relative.wav"}, {"out_path": str(tmp / "speech-out" / "a.ogg")})
    c.check("out_path not absolute, or not .wav or .mp3 -> INVALID_ARGUMENT field=out_path",
            refusal(errs, "INVALID_ARGUMENT", "caller") and details(errs, "field") == ["out_path"] * 2 and speaks() == before,
            json.dumps(errs)[:400], key="speech-failure:out-path", answers=errs)

    errs = await errors({"workflow": "it-illustration"})
    c.check("generate_speech with an image workflow -> INVALID_ARGUMENT, next_step names generate_image",
            refusal(errs, "INVALID_ARGUMENT", "caller") and details(errs, "field") == ["workflow"]
            and "generate_image" in errs[0].get("next_step", "") and speaks() == before,
            json.dumps(errs)[:400], key="speech-failure:kind", answers=errs)

    errs = await errors({"out_path": str(out)})
    c.check("an existing out_path without overwrite -> OUTPUT_EXISTS",
            refusal(errs, "OUTPUT_EXISTS", "caller") and details(errs, "path") == [str(out)] and speaks() == before,
            json.dumps(errs)[:400], key="speech-failure:output-exists", answers=errs)

    model = Path(sandbox["speech_model"])
    hidden = model.with_name(model.name + ".away")
    model.rename(hidden)
    try:
        errs = await errors({})
        away = await speech_states(s)
    finally:
        hidden.rename(model)
    c.check("a missing part of the speech setup (a model file) -> list_workflows and describe_workflow show the speech workflow "
            "as partially installed, and its file as missing",
            away == ("partially_installed", "partially_installed", ["missing"]), str(away), key="speech-install-state:missing")
    c.check("a missing part of the speech setup (a model file) -> WORKFLOW_NOT_INSTALLED naming it, next_step names install_workflow",
            refusal(errs, "WORKFLOW_NOT_INSTALLED", "caller") and "model.bin" in (details(errs, "missing")[0] or [])
            and "install_workflow" in errs[0].get("next_step", "") and speaks() == before,
            json.dumps(errs)[:400], key="speech-failure:not-installed", answers=errs)


async def speech_worker_checks(server_cmd: list[str], base_env: dict[str, str], tmp: Path, c: Checks) -> None:
    """The failure "the speech worker stops or reports an error while it makes the file": one server for each mode of
    the fake speech worker (die: the process ends in the request; error: it answers ok=false)."""
    errs, left = [], []
    for mode in ("die", "error"):
        params = StdioServerParameters(command=server_cmd[0], args=server_cmd[1:], env={**base_env, "HORUS_FAKE_SPEECH": mode})
        out = tmp / "speech-out" / f"{mode}.wav"
        with open(tmp / f"server-speech-{mode}.stderr.log", "w", encoding="utf-8") as errlog:
            async with stdio_client(params, errlog=errlog) as (read, write):
                async with ClientSession(read, write) as s:
                    await s.initialize()
                    res = env_of(await s.call_tool("generate_speech", {"workflow": SPEECH, "text": SPEECH_TEXT, "out_path": str(out)}))
        errs.append(res.get("error") or {})
        left.append(out.exists())
    c.check("a speech worker that stops or reports an error -> GENERATION_FAILED, retryable, action_by=retry, with details.worker_message",
            refusal(errs, "GENERATION_FAILED", "retry") and all(e.get("retryable") is True and "generate_speech" in e.get("next_step", "")
                                                               and (e.get("details") or {}).get("worker_message") for e in errs)
            and not any(left), json.dumps(errs)[:500] + f" files left={left}", key="speech-failure:worker-failed", answers=errs)


# The system package that the install of ComfyUI needs (spec docs/specs/2026-11-15-release-install.md,
# "The setup plan"): its name, its package manager, the arguments of the manager and its program.
PACKAGE = (("7-Zip", "winget", ["install", "--id", "7zip.7zip", "-e", "--accept-source-agreements", "--accept-package-agreements"], "7z")
           if os.name == "nt" else ("Python 3.12", "brew", ["install", "python@3.12"], "python3.12"))


def fake_program(bindir: Path, name: str, record: Path) -> Path:
    """A fake program: it appends its arguments to record and exits 0. It installs nothing."""
    bindir.mkdir(parents=True, exist_ok=True)
    if os.name == "nt":
        path = bindir / f"{name}.cmd"
        path.write_text(f'@echo off\r\necho %*>>"{record}"\r\nexit /b 0\r\n', encoding="utf-8", newline="")
    else:
        path = bindir / name
        path.write_text(f"#!/bin/sh\nprintf '%s\\n' \"$*\" >> '{record}'\nexit 0\n", encoding="utf-8")
        path.chmod(0o755)
    return path


def tree(root: Path) -> list[str]:
    """The path and the SHA-256 of each file under root."""
    return sorted(f"{p} {hashlib.sha256(p.read_bytes()).hexdigest()}" for p in root.rglob("*") if p.is_file())


async def setup_checks(server_cmd: list[str], base_env: dict[str, str], tmp: Path, c: Checks) -> None:
    """install_workflow on a machine without ComfyUI. The PATH holds only a folder of fake programs and the
    folders of the OS, so no real package manager runs. HTTP_PROXY and HTTPS_PROXY name a closed port, so no
    download can leave the machine. The model files of it-illustration are present; ComfyUI is not."""
    bare = tmp / "bare"
    sandbox = make_sandbox(bare)
    home, comfy, record = bare / "home", bare / "no-comfyui", bare / "package-manager.args"
    home.mkdir()
    system = [str(Path(os.environ.get("SYSTEMROOT", r"C:\Windows")) / "System32")] if os.name == "nt" else ["/usr/bin", "/bin"]
    name, manager, manager_args, program = PACKAGE

    def params_with(bindir: Path) -> StdioServerParameters:
        bindir.mkdir(parents=True, exist_ok=True)
        env = {**base_env, "HORUS_WORKFLOWS_DIR": sandbox["workflows"], "HORUS_MODELS_DIR": sandbox["models"],
               "HORUS_TEXT_ENCODER_DIR": str(Path(sandbox["models"]) / "text_encoders"), "HORUS_COMFYUI_DIR": str(comfy),
               "HORUS_ROOT": str(bare), "HORUS_HOME": str(home / "horus"), "HOME": str(home), "USERPROFILE": str(home),
               "ProgramFiles": str(home / "Program Files"), "ProgramFiles(x86)": str(home / "Program Files (x86)"),
               "PATH": os.pathsep.join([str(bindir), *system]),
               "HTTP_PROXY": "http://127.0.0.1:9", "HTTPS_PROXY": "http://127.0.0.1:9", "NO_PROXY": ""}
        return StdioServerParameters(command=server_cmd[0], args=server_cmd[1:], env=env)

    # No package manager: the package step is a step for a person.
    with open(tmp / "server-no-manager.stderr.log", "w", encoding="utf-8") as errlog:
        async with stdio_client(params_with(bare / "bin-empty"), errlog=errlog) as (read, write):
            async with ClientSession(read, write) as s:
                await s.initialize()
                before = tree(bare)
                res = env_of(await s.call_tool("install_workflow", {"workflow": "it-illustration", "accept_setup": True, "accept_license": True}))
                e = res.get("error") or {}
                c.check(f"no ComfyUI and no package manager -> BACKEND_NOT_INSTALLED naming {name}; nothing changed",
                        e.get("code") == "BACKEND_NOT_INSTALLED" and e.get("action_by") == "human" and name in e.get("next_step", "")
                        and tree(bare) == before, json.dumps(res)[:400])

    # A fake package manager on the PATH. It installs nothing, so the plan stops after its step.
    fake_program(bare / "bin-fake", manager, record)
    with open(tmp / "server-no-comfyui.stderr.log", "w", encoding="utf-8") as errlog:
        async with stdio_client(params_with(bare / "bin-fake"), errlog=errlog) as (read, write):
            async with ClientSession(read, write) as s:
                await s.initialize()
                before = tree(bare)
                res = env_of(await s.call_tool("install_workflow", {"workflow": "it-illustration"}))
                e = res.get("error") or {}
                steps = (e.get("details") or {}).get("steps") or []
                kinds = [st.get("kind") for st in steps]
                pkg = steps[0] if steps else {}
                command = pkg.get("command") or [""]
                c.check("no ComfyUI -> SETUP_CONFIRMATION_REQUIRED, action_by=human, with details.steps",
                        e.get("code") == "SETUP_CONFIRMATION_REQUIRED" and e.get("action_by") == "human" and e.get("retryable") is False
                        and kinds[:1] == ["package"] and kinds[-1:] == ["backend"] and pkg.get("name") == name
                        and Path(command[0]).stem == manager and command[1:] == manager_args
                        and next((st.get("dir") for st in steps if st.get("kind") == "backend"), None) == str(comfy),
                        json.dumps(res)[:500])

                res = env_of(await s.call_tool("install_workflow", {"workflow": "it-illustration", "accept_license": True}))
                c.check("no change on disk before the setup consent, also with accept_license=true",
                        (res.get("error") or {}).get("code") == "SETUP_CONFIRMATION_REQUIRED" and tree(bare) == before
                        and not record.exists() and not comfy.exists(), json.dumps(res)[:300])

                res = env_of(await s.call_tool("install_workflow", {"workflow": "absent-style", "accept_setup": True}))
                c.check("accept_setup=true without accept_license -> LICENSE_CONFIRMATION_REQUIRED; nothing changed",
                        (res.get("error") or {}).get("code") == "LICENSE_CONFIRMATION_REQUIRED" and tree(bare) == before
                        and not record.exists(), json.dumps(res)[:300])

                progress: list[str] = []

                async def on_progress(p, total, message):
                    progress.append(message or "")

                res = env_of(await s.call_tool("install_workflow", {"workflow": "it-illustration", "accept_setup": True},
                                               progress_callback=on_progress))
                e = res.get("error") or {}
                ran = record.read_text(encoding="utf-8").split() if record.exists() else []
                c.check(f"accept_setup=true runs {manager} with the arguments of the spec, then stops: {program} not found",
                        ran == manager_args and e.get("code") == "BACKEND_NOT_INSTALLED"
                        and ((e.get("details") or {}).get("step") or {}).get("kind") == "package"
                        and f"{program} was not found" in e.get("message", "") and not comfy.exists(),
                        json.dumps(res)[:400] + f" ran={ran}")
                c.check("install_workflow sends a progress message that names the setup step",
                        any(m.startswith("setup step 1 of ") and name in m for m in progress), str(progress))


# The speech packages (spec docs/specs/2026-11-20-text-to-speech.md, "Install and uninstall"): for each package, its
# name, the arguments of the package manager, and where the package manager puts its programs (platform.SpeechPackagesFor).
SPEECH_PACKAGES = ((("Python 3.12", ["install", "--id", "Python.Python.3.12", "-e", "--accept-source-agreements", "--accept-package-agreements"]),
                    ("FFmpeg", ["install", "--id", "Gyan.FFmpeg", "-e", "--accept-source-agreements", "--accept-package-agreements"]))
                   if os.name == "nt" else
                   (("Python 3.12", ["install", "python@3.12"]), ("FFmpeg", ["install", "ffmpeg"])))
SPEECH_PINS = ["vieneu==3.8.4", "faster-whisper==1.2.1"]


def fake_package_manager(bindir: Path, name: str, record: Path, src: Path, local: Path) -> None:
    """A fake winget or brew: it appends its arguments to record, then puts a copy of src (the fake speech worker,
    which is also a fake Python) where the real one puts each program: Python 3.12, or ffmpeg and ffprobe."""
    bindir.mkdir(parents=True, exist_ok=True)
    if os.name == "nt":
        py, ff = local / "Programs" / "Python" / "Python312", local / "Microsoft" / "WinGet" / "Links"
        (bindir / f"{name}.cmd").write_text(
            "@echo off\r\n"
            f'echo %*>>"{record}"\r\n'
            f'if "%3"=="Python.Python.3.12" (mkdir "{py}" 2>nul & copy /y "{src}" "{py}\\python.exe" >nul)\r\n'
            f'if "%3"=="Gyan.FFmpeg" (mkdir "{ff}" 2>nul & copy /y "{src}" "{ff}\\ffmpeg.exe" >nul & copy /y "{src}" "{ff}\\ffprobe.exe" >nul)\r\n'
            "exit /b 0\r\n", encoding="utf-8", newline="")
    else:
        path = bindir / name
        path.write_text("#!/bin/sh\n"
                        f"printf '%s\\n' \"$*\" >> '{record}'\n"
                        'case "$*" in\n'
                        f"  *python@3.12*) cp '{src}' '{bindir}/python3.12' ;;\n"
                        f"  *ffmpeg*) cp '{src}' '{bindir}/ffmpeg'; cp '{src}' '{bindir}/ffprobe' ;;\n"
                        "esac\nexit 0\n", encoding="utf-8")
        path.chmod(0o755)


async def speech_setup_checks(server_cmd: list[str], base_env: dict[str, str], sandbox: dict[str, str], tmp: Path, c: Checks) -> None:
    """install_workflow of a speech workflow on a machine without Python 3.12, FFmpeg and the speech setup. The PATH
    holds only a fake package manager and the folders of the OS. The fake package manager puts a copy of the fake
    speech worker where the real one puts Python 3.12, ffmpeg and ffprobe. As Python 3.12, the fake worker makes a
    stand-in venv ("-m venv") and installs nothing ("-m pip"). The model file has an http address, which the
    downloader refuses at once: the download step runs, and no byte leaves the machine. Then the test puts the
    stand-in model file where the download puts it, as a finished download does. At the end, uninstall_workflow
    deletes that model file."""
    bare = tmp / "speech-bare"
    flows, home, local, speech = bare / "workflows", bare / "home", bare / "local", bare / "speech"
    flow = flows / SPEECH
    flow.mkdir(parents=True)
    home.mkdir()
    shutil.copy(ROOT / "internal" / "catalog" / "testdata" / SPEECH / "manifest.json", flow / "manifest.json")
    data = b"stand-in voice model"
    (flow / "models.json").write_text(json.dumps({"repo": "stand-in/speech", "license": "Apache-2.0", "files": [
        {"role": "speech", "repo_path": "voice.bin", "folder": "tts", "url": "http://127.0.0.1:9/voice.bin",
         "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}]}, indent=1), encoding="utf-8")
    bindir, record, worker = bare / "bin", bare / "package-manager.args", bare / "speech-worker.record"
    manager = "winget" if os.name == "nt" else "brew"
    fake_package_manager(bindir, manager, record, Path(sandbox["speech_dir"]) / "venv" / (
        "Scripts/python.exe" if os.name == "nt" else "bin/python"), local)
    system = [str(Path(os.environ.get("SYSTEMROOT", r"C:\Windows")) / "System32")] if os.name == "nt" else ["/usr/bin", "/bin"]
    env = {**base_env, "HORUS_WORKFLOWS_DIR": str(flows), "HORUS_SPEECH_DIR": str(speech), "HORUS_ROOT": str(bare),
           "HORUS_HOME": str(home / "horus"), "HORUS_OUTPUT_DIR": str(bare / "out"), "HORUS_COMFYUI_DIR": str(bare / "no-comfyui"),
           "HOME": str(home), "USERPROFILE": str(home), "LOCALAPPDATA": str(local),
           "ProgramFiles": str(home / "Program Files"), "ProgramFiles(x86)": str(home / "Program Files (x86)"),
           "HORUS_FAKE_SPEECH_RECORD": str(worker), "PATH": os.pathsep.join([str(bindir), *system]),
           "HTTP_PROXY": "http://127.0.0.1:9", "HTTPS_PROXY": "http://127.0.0.1:9", "NO_PROXY": ""}
    venv = speech / "venv"
    model = speech / "models" / "tts" / "voice.bin"
    params = StdioServerParameters(command=server_cmd[0], args=server_cmd[1:], env=env)
    with open(tmp / "server-speech-setup.stderr.log", "w", encoding="utf-8") as errlog:
        async with stdio_client(params, errlog=errlog) as (read, write):
            async with ClientSession(read, write) as s:
                await s.initialize()
                before = tree(bare)
                res = env_of(await s.call_tool("install_workflow", {"workflow": SPEECH}))
                e = res.get("error") or {}
                steps = (e.get("details") or {}).get("steps") or []
                want = [("package", SPEECH_PACKAGES[0][0]), ("package", SPEECH_PACKAGES[1][0]), ("speech-venv", "speech venv"), ("file", "voice.bin")]
                commands = [st.get("command") or [""] for st in steps]
                c.check("a speech workflow on a machine without Python 3.12, FFmpeg and the speech setup -> SETUP_CONFIRMATION_REQUIRED, "
                        "details.steps in order: Python 3.12, FFmpeg, the speech venv, the model file; no ComfyUI step",
                        e.get("code") == "SETUP_CONFIRMATION_REQUIRED" and e.get("action_by") == "human"
                        and [(st.get("kind"), st.get("name")) for st in steps] == want
                        and all(Path(commands[i][0]).stem == manager and commands[i][1:] == SPEECH_PACKAGES[i][1] for i in (0, 1))
                        and len(commands) == 4 and commands[2][-2:] == SPEECH_PINS and steps[2].get("dir") == str(venv)
                        and steps[3].get("dir") == str(model.parent), json.dumps(res)[:600], key="speech-setup:plan")

                lic = env_of(await s.call_tool("install_workflow", {"workflow": SPEECH, "accept_license": True}))
                setup = env_of(await s.call_tool("install_workflow", {"workflow": SPEECH, "accept_setup": True}))
                c.check("a speech workflow: no change on disk before both consents (accept_license alone -> SETUP_CONFIRMATION_REQUIRED, "
                        "accept_setup alone -> LICENSE_CONFIRMATION_REQUIRED)",
                        (lic.get("error") or {}).get("code") == "SETUP_CONFIRMATION_REQUIRED"
                        and (setup.get("error") or {}).get("code") == "LICENSE_CONFIRMATION_REQUIRED"
                        and tree(bare) == before and not record.exists() and not speech.exists() and not worker.exists(),
                        json.dumps(lic)[:200] + " " + json.dumps(setup)[:200], key="speech-setup:no-change")

                progress: list[str] = []

                async def on_progress(p, total, message):
                    progress.append(message or "")

                res = env_of(await s.call_tool("install_workflow", {"workflow": SPEECH, "accept_setup": True, "accept_license": True},
                                               progress_callback=on_progress))
                e = res.get("error") or {}
                ran = record.read_text(encoding="utf-8").splitlines() if record.exists() else []
                python = [ln for ln in speech_record_of(worker) if ln.startswith("start -m ")]
                rec = json.loads((venv / "horus-install.json").read_text(encoding="utf-8")) if (venv / "horus-install.json").is_file() else {}
                c.check(f"both consents run the steps in order: {manager} for Python 3.12, then for FFmpeg, the speech venv with "
                        "the pinned packages and its install record, then the download of the model file (refused: no network)",
                        [ln.split() for ln in ran] == [SPEECH_PACKAGES[0][1], SPEECH_PACKAGES[1][1]]
                        and python == [f"start -m venv {venv}", "start -m pip install --disable-pip-version-check " + " ".join(SPEECH_PINS)]
                        and rec.get("kind") == "speech-venv" and rec.get("packages") == SPEECH_PINS
                        and e.get("code") == "DOWNLOAD_FAILED" and (e.get("details") or {}).get("file") == "voice.bin" and not model.exists()
                        and any(m.startswith("setup step 3 of 3: make the speech venv") for m in progress),
                        json.dumps(res)[:400] + f" ran={ran} python={python} record={rec} progress={progress}", key="speech-setup:steps")

                model.parent.mkdir(parents=True, exist_ok=True)
                model.write_bytes(data)  # what the download puts there
                out = bare / "out" / "after-install.wav"
                spoken = env_of(await s.call_tool("generate_speech", {"workflow": SPEECH, "text": SPEECH_TEXT, "out_path": str(out)}))
                listed = env_of(await s.call_tool("list_workflows", {}))
                states = {w["name"]: w["install_state"] for w in (listed.get("result") or {}).get("workflows", [])}
                c.check("after install_workflow and the model file, generate_speech writes the audio file, and list_workflows shows "
                        "the speech workflow installed",
                        spoken.get("ok") is True and out.is_file() and (spoken.get("result") or {}).get("transcript") == SPEECH_TEXT
                        and states == {SPEECH: "installed"}, json.dumps(spoken)[:400] + " " + json.dumps(states), key="speech-setup:works")

                runs = len(ran)
                again = env_of(await s.call_tool("install_workflow", {"workflow": SPEECH, "accept_setup": True, "accept_license": True}))
                ran = record.read_text(encoding="utf-8").splitlines() if record.exists() else []
                c.check("a second install_workflow of the speech workflow -> already_installed; no setup step runs again",
                        again.get("ok") is True and (again.get("result") or {}).get("already_installed") is True and len(ran) == runs
                        and [ln for ln in speech_record_of(worker) if ln.startswith("start -m ")] == python,
                        json.dumps(again)[:300] + f" ran={ran}", key="speech-setup:again")

                # The uninstall of the speech workflow: uninstall_workflow deletes the model file in the speech folder. The
                # speech venv stays (horus uninstall removes it). The server ends its speech worker before it deletes.
                def starts() -> list[str]:
                    return [ln for ln in speech_record_of(worker) if ln.startswith("start -I ")]

                plan = env_of(await s.call_tool("uninstall_workflow", {"workflow": SPEECH}))
                e = plan.get("error") or {}
                rows = [(f.get("file"), f.get("path"), f.get("state")) for f in (e.get("details") or {}).get("files") or []]
                c.check("uninstall_workflow of a speech workflow without confirm -> CONFIRMATION_REQUIRED naming the model file in the "
                        "speech folder; nothing deleted",
                        e.get("code") == "CONFIRMATION_REQUIRED" and rows == [("voice.bin", str(model), "would_delete")] and model.is_file(),
                        json.dumps(plan)[:400], key="speech-uninstall:plan")

                first = starts()
                gone = env_of(await s.call_tool("uninstall_workflow", {"workflow": SPEECH, "confirm": True}))
                r = gone.get("result") or {}
                missing = env_of(await s.call_tool("generate_speech", {"workflow": SPEECH, "text": SPEECH_TEXT})).get("error") or {}
                model.write_bytes(data)  # the file again, as a new install puts it
                back = env_of(await s.call_tool("generate_speech", {"workflow": SPEECH, "text": SPEECH_TEXT}))
                c.check("uninstall_workflow of a speech workflow with confirm deletes the model file and ends the speech worker: "
                        "generate_speech then answers WORKFLOW_NOT_INSTALLED naming the file, the speech venv stays, and with the "
                        "file back a new speech worker starts",
                        gone.get("ok") is True and [(f.get("file"), f.get("state")) for f in r.get("files", [])] == [("voice.bin", "deleted")]
                        and r.get("freed_bytes") == len(data) and missing.get("code") == "WORKFLOW_NOT_INSTALLED"
                        and "voice.bin" in ((missing.get("details") or {}).get("missing") or []) and (venv / "horus-install.json").is_file()
                        and back.get("ok") is True and (back.get("result") or {}).get("started_worker") is True
                        and len(first) == 1 and len(starts()) == 2,
                        json.dumps(gone)[:300] + " " + json.dumps(missing)[:200] + " " + json.dumps(back)[:200] + f" starts={starts()}",
                        key="speech-uninstall:deleted")


def speech_record_of(path: Path) -> list[str]:
    """The lines of a record file of the fake speech worker."""
    return path.read_text(encoding="utf-8").splitlines() if path.is_file() else []


HTTPD: Server
# True in the run over Streamable HTTP (--http).
HTTP_RUN = False
# The count of checks of the run on the stack (the checks of the host run, less STACK_SKIPPED, and the 2 stack checks:
# the stack ran the call, and the stack mounts the models folder of the test).
STACK_TOTAL = 37


# The checks that the run on the stack leaves out, and why (the pull request of task #203 lists them too).
STACK_SKIPPED = {
    "the checks of the speech workflow (describe, generate_image refusal, generate_speech, the speech worker)":
        "the stand-in speech workflow and the fake speech worker are files of the host sandbox. The container has no speech setup (the speech worker is outside the job queue).",
    "style without model files -> WORKFLOW_NOT_INSTALLED":
        "absent-style is a copy in the sandbox of the host. The stack has no such workflow.",
    "the 2 uninstall checks of the speech workflow":
        "they are speech setup checks: the stack has no speech workflow. The 9 install and uninstall checks of the image workflows run on the stack.",
    "the setup checks and the speech setup checks":
        "they run install_workflow with a fake package manager on the PATH of a host server process.",
    "the 2 style-neutral checks of each shipped workflow":
        "they read the workflow files of the repo, not an answer of the server. The host run has them.",
    "backend down and autostart off -> BACKEND_UNAVAILABLE":
        "the stack has no autostart. The same step on the stack checks NOT_READY with details.down naming comfyui.",
}


def main() -> int:
    global HTTPD, HTTP_RUN
    ap = argparse.ArgumentParser()
    ap.add_argument("--server-cmd", nargs="+", default=["horus", "mcp"])
    ap.add_argument("--report", type=Path, help="write each check and the answers of the speech checks to this JSON file")
    ap.add_argument("--stack", default="", help="run over stdio into the stack with this compose project name (docker exec -i <name>-api horus mcp)")
    ap.add_argument("--shared-dir", type=Path, help="stack: the host folder of a shared folder of the stack (HORUS_SHARED_DIRS)")
    ap.add_argument("--models-dir", type=Path, help="stack: the host folder that horus stack-mounts mounts into horus-api as "
                                                    "HORUS_MODELS_DIR. An empty test folder: the test writes stand-in model files there")
    ap.add_argument("--workflows-dir", type=Path, help="stack: the host folder that tests/stack-sandbox.yaml mounts into the stack "
                                                       "as HORUS_WORKFLOWS_DIR. An empty test folder: the test writes two workflows there")
    ap.add_argument("--port", type=int, default=0, help="the port of the fake ComfyUI (stack: the port in HORUS_COMFYUI_URL of the stack)")
    ap.add_argument("--listen", default="127.0.0.1",
                    help="the address of the fake ComfyUI. Docker Desktop reaches 127.0.0.1 of the host at host.docker.internal. "
                         "Docker on Linux needs 0.0.0.0")
    ap.add_argument("--ready-timeout", type=float, default=180, help="stack: the most seconds to wait until horus ready exits 0")
    ap.add_argument("--http", default="", metavar="URL",
                    help="stack: run over Streamable HTTP to /mcp at URL (http://127.0.0.1:<HORUS_API_PORT>/mcp), with the bearer token "
                         "of the environment variable HORUS_API_TOKEN. Needs --stack")
    a = ap.parse_args()
    if a.stack and not (a.shared_dir and a.models_dir and a.workflows_dir):
        ap.error("--stack needs --shared-dir, --models-dir and --workflows-dir")
    if a.http and not a.stack:
        ap.error("--http needs --stack: the checks read the job rows in <stack>-db")
    if a.http and not os.environ.get("HORUS_API_TOKEN"):
        ap.error("--http needs the environment variable HORUS_API_TOKEN: the token of the .env of the stack")
    HTTP_RUN = bool(a.http)
    sys.path.insert(0, str(ROOT))
    c = Checks()
    HTTPD = Server((a.listen, a.port), Handler)
    threading.Thread(target=HTTPD.serve_forever, daemon=True).start()
    with tempfile.TemporaryDirectory(prefix="horus-contract-") as td:
        tmp = Path(td)
        try:
            if a.stack:
                sandbox = make_stack_sandbox(a.workflows_dir.resolve(), a.models_dir.resolve())
                wait_ready(a.stack, a.ready_timeout)
                asyncio.run(run_stack(a.stack, a.shared_dir.resolve(), sandbox, tmp, c, a.http))
            else:
                sandbox = make_sandbox(tmp, with_speech=True)
                asyncio.run(run(a.server_cmd, sandbox, HTTPD.server_address[1], tmp, c))
        except Exception as exc:  # the server did not start or the session broke
            way = "over HTTP" if a.http else "over stdio"
            c.check(f"the server starts and answers {way}", False, f"{type(exc).__name__}: {exc}")
            log = tmp / "server.stderr.log"
            if log.is_file():
                print(log.read_text(encoding="utf-8", errors="replace")[-2000:])
    passed = sum(1 for ok, _, _ in c.rows if ok)
    shipped = sum(1 for d in (ROOT / "workflows").iterdir() if (d / "manifest.json").is_file())
    # 43 server checks of the image tools, the kind and the code table, 14 speech checks, 5 checks of the speech setup,
    # 2 checks of the speech uninstall, plus 2 style-neutral checks for each shipped workflow
    total = 43 + 14 + 5 + 2 + 2 * shipped
    if a.stack:
        total = STACK_TOTAL
    if a.report:
        a.report.write_text(json.dumps(c.report(total), indent=1, ensure_ascii=False), encoding="utf-8")
    print(f"contract: {passed} of {total} passed")
    return 0 if passed == total and len(c.rows) == total else 1


if __name__ == "__main__":
    sys.exit(main())

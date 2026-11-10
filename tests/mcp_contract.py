#!/usr/bin/env python3
"""Contract test of the horus MCP server. No GPU, no network, no real model files.

    python tests/mcp_contract.py [--server-cmd <exe> ...]      (Python 3.12 with mcp==2.3.0)

The test starts the server over stdio, like an agent harness does, and talks to
it with the official MCP client. A fake ComfyUI (a small HTTP server in this
process) answers the backend calls. A temporary folder holds a copy of the
it-illustration workflow, tiny stand-in model files, and a second workflow
whose files are absent.

It prints one PASS or FAIL line per check and "contract: N of M passed".
Exit code 0 only when all checks pass. The default server command is
`horus mcp`, with `horus` found on the PATH. The list of error codes comes from
`<server program> errors -json`.
"""

from __future__ import annotations

import argparse
import asyncio
import hashlib
import json
import os
import shutil
import struct
import subprocess
import sys
import tempfile
import threading
import zlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

ROOT = Path(__file__).resolve().parent.parent
FIXTURE = json.loads((ROOT / "tests" / "fixtures" / "object_info.json").read_text(encoding="utf-8"))
SUBJECT = ("On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt. "
           "On the right, a sky-blue server box takes them off the belt.")
EXPECTED_TOOLS = {"list_workflows", "describe_workflow", "install_workflow", "uninstall_workflow", "generate_image"}
ERROR_KEYS = {"code", "message", "retryable", "action_by", "next_step", "details"}


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


def make_sandbox(tmp: Path) -> dict[str, str]:
    """Workflows: it-illustration (installed, tiny stand-in files) and absent-style (no files)."""
    wf_root, models_root = tmp / "workflows", tmp / "comfy" / "ComfyUI" / "models"
    src = ROOT / "workflows" / "it-illustration"
    dst = wf_root / "it-illustration"
    shutil.copytree(src, dst, ignore=shutil.ignore_patterns("examples"))
    models = json.loads((src / "models.json").read_text(encoding="utf-8"))
    for f in models["files"]:
        data = f"stand-in for {f['repo_path']}".encode()
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
    (tmp / "comfy" / "ComfyUI" / "main.py").write_text("# fake\n", encoding="utf-8")
    return {"workflows": str(wf_root), "comfy": str(tmp / "comfy"), "models": str(models_root)}


class Checks:
    def __init__(self) -> None:
        self.rows: list[tuple[bool, str, str]] = []

    def check(self, name: str, ok: bool, detail: str = "") -> None:
        self.rows.append((bool(ok), name, detail))
        print(f"{'PASS' if ok else 'FAIL'}  {name}{'' if ok else '  -- ' + detail}", flush=True)


def env_of(res) -> dict:
    return res.structured_content or json.loads(res.content[0].text)


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
    }
    params = StdioServerParameters(command=server_cmd[0], args=server_cmd[1:], env=env)
    with open(tmp / "server.stderr.log", "w", encoding="utf-8") as errlog:
        await session_checks(params, errlog, sandbox, tmp, c)

    neutral_checks(c)

    try:  # the doc table must list every code; the server program prints the code table
        out = subprocess.run([server_cmd[0], "errors", "-json"], capture_output=True, timeout=60).stdout
        ERRORS = list(json.loads(out)) or ["<no code in errors -json>"]
    except (OSError, ValueError, subprocess.SubprocessError) as exc:
        ERRORS = [f"<{server_cmd[0]} errors -json failed: {exc}>"]
    doc = (ROOT / "docs" / "mcp.md").read_text(encoding="utf-8") if (ROOT / "docs" / "mcp.md").is_file() else ""
    missing = [code for code in ERRORS if f"`{code}`" not in doc]
    c.check("docs/mcp.md lists every error code", not missing, f"missing: {missing}")


STYLE_WORDS = ("palette", "pastel", "navy", "flat vector", "house style", "house-style", "dotted grid")


def neutral_checks(c: Checks) -> None:
    """Every shipped workflow is style-neutral: no theme, no style file, no style words in the graph."""
    for d in sorted(p for p in (ROOT / "workflows").iterdir() if (p / "manifest.json").is_file()):
        m = json.loads((d / "manifest.json").read_text(encoding="utf-8"))
        c.check(f"{d.name}: manifest has no theme and no caption style file",
                "theme" not in m and "caption" not in m and not list(d.glob("*style*.json")), json.dumps(m.get("theme") or m.get("caption")))
        graph = json.loads((d / m["graph"]["file"]).read_text(encoding="utf-8"))
        texts = " ".join(str(v) for n in graph.get("nodes", []) for v in (n.get("widgets_values") or [])).lower()
        words = [w for w in STYLE_WORDS if w in texts]
        c.check(f"{d.name}: the graph holds no style words", not words, str(words))


async def session_checks(params, errlog, sandbox: dict[str, str], tmp: Path, c: Checks) -> None:
    async with stdio_client(params, errlog=errlog) as (read, write):
        async with ClientSession(read, write) as s:
            init = await s.initialize()
            c.check("server identifies as horus and has instructions",
                    init.server_info.name == "horus" and bool(init.instructions), str(init.server_info))

            tools = {t.name: t for t in (await s.list_tools()).tools}
            c.check("tools/list has exactly the 5 tools", set(tools) == EXPECTED_TOOLS, str(sorted(tools)))
            c.check("every tool has an outputSchema with ok, result, error",
                    all(t.output_schema and {"ok", "result", "error"} <= set(t.output_schema.get("properties", {}))
                        for t in tools.values()), "")
            ann = tools.get("uninstall_workflow").annotations if "uninstall_workflow" in tools else None
            c.check("uninstall_workflow is marked destructive, list_workflows read-only",
                    bool(ann and ann.destructive_hint) and bool(tools["list_workflows"].annotations.read_only_hint), "")

            res = env_of(await s.call_tool("list_workflows", {}))
            states = {w["name"]: w["install_state"] for w in (res.get("result") or {}).get("workflows", [])}
            # absent-style shares the text encoder and the VAE with it-illustration, so 2 of its 3 files are present.
            c.check("list_workflows shows installed and partially_installed styles",
                    res["ok"] and states == {"it-illustration": "installed", "absent-style": "partially_installed"}, json.dumps(res)[:400])

            raw = await s.call_tool("describe_workflow", {"workflow": "it-illustration"})
            res = env_of(raw)
            text = raw.content[0].text
            r = res.get("result") or {}
            c.check("describe_workflow gives subject rules, examples and 3 present files",
                    res["ok"] and len(r.get("subject", {}).get("rules", [])) >= 3 and r["subject"]["examples"]
                    and [f["state"] for f in r.get("files", [])] == ["present"] * 3, json.dumps(res)[:400])
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

            out = tmp / "images" / "queue.png"
            res = env_of(await s.call_tool("generate_image", {"workflow": "it-illustration", "subject": SUBJECT,
                                                             "out_path": str(out), "seed": 7}, progress_callback=on_progress))
            r = res.get("result") or {}
            graph = Fake.prompts.get(r.get("job_id", ""), {}).get("graph", {})
            caption = next((n["inputs"]["text"] for n in graph.values() if n["class_type"] == "CLIPTextEncode"), "")
            c.check("generate_image writes a PNG at out_path and returns its path and seed",
                    res["ok"] and Path(r.get("path", "")) == out and out.read_bytes()[:8] == b"\x89PNG\r\n\x1a\n" and r.get("seed") == 7,
                    json.dumps(res)[:400])
            # horus is style-neutral: the caller owns the look, the palette and the design system.
            c.check(f"horus sends the subject to the model as written ({len(caption) - len(SUBJECT)} characters added)",
                    caption == SUBJECT, caption[:200])
            sampler = next((n["inputs"] for n in graph.values() if n["class_type"] == "KSampler"), {})
            c.check("seed 7 reaches the sampler of the graph", sampler.get("seed") == 7, str(sampler)[:200])
            c.check("generate_image sends progress notifications", len(progress) >= 1, str(progress))

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
            job = e.get("details", {}).get("job_id")
            c.check("a job over the time limit -> GENERATION_TIMEOUT, and horus cancels it on the backend",
                    e.get("code") == "GENERATION_TIMEOUT" and job in Fake.cancelled, json.dumps(res)[:300] + f" cancelled={Fake.cancelled}")
            Fake.mode = "ok"

            res = env_of(await s.call_tool("install_workflow", {"workflow": "it-illustration"}))
            c.check("install of an installed style -> ok, already_installed",
                    res["ok"] and res["result"]["already_installed"] is True, json.dumps(res)[:300])

            res = env_of(await s.call_tool("install_workflow", {"workflow": "absent-style"}))
            e = res.get("error") or {}
            c.check("install without accept_license -> LICENSE_CONFIRMATION_REQUIRED, action_by=human, with size",
                    e.get("code") == "LICENSE_CONFIRMATION_REQUIRED" and e["action_by"] == "human" and e["details"].get("download_bytes", 0) > 0,
                    json.dumps(res)[:300])

            model_files = list(Path(sandbox["models"]).rglob("*.safetensors"))
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
                    states == {"it-illustration": "not_installed", "absent-style": "not_installed"}, json.dumps(states))

            res = env_of(await s.call_tool("generate_image", {"workflow": "nope-style", "subject": SUBJECT}))
            c.check("errors are the same envelope in structuredContent and in text",
                    res.get("ok") is False and res.get("result") is None and set(res.get("error", {})) == ERROR_KEYS, json.dumps(res)[:300])

            HTTPD.shutdown()
            HTTPD.server_close()
            res = env_of(await s.call_tool("generate_image", {"workflow": "it-illustration", "subject": SUBJECT}))
            e = res.get("error") or {}
            c.check("backend down and autostart off -> BACKEND_UNAVAILABLE, action_by=human",
                    e.get("code") == "BACKEND_UNAVAILABLE" and e["action_by"] == "human", json.dumps(res)[:300])


HTTPD: ThreadingHTTPServer


def main() -> int:
    global HTTPD
    ap = argparse.ArgumentParser()
    ap.add_argument("--server-cmd", nargs="+", default=["horus", "mcp"])
    a = ap.parse_args()
    sys.path.insert(0, str(ROOT))
    c = Checks()
    HTTPD = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=HTTPD.serve_forever, daemon=True).start()
    with tempfile.TemporaryDirectory(prefix="horus-contract-") as td:
        tmp = Path(td)
        sandbox = make_sandbox(tmp)
        try:
            asyncio.run(run(a.server_cmd, sandbox, HTTPD.server_address[1], tmp, c))
        except Exception as exc:  # the server did not start or the session broke
            c.check("the server starts and answers over stdio", False, f"{type(exc).__name__}: {exc}")
            log = tmp / "server.stderr.log"
            if log.is_file():
                print(log.read_text(encoding="utf-8", errors="replace")[-2000:])
    passed = sum(1 for ok, _, _ in c.rows if ok)
    shipped = sum(1 for d in (ROOT / "workflows").iterdir() if (d / "manifest.json").is_file())
    total = 28 + 2 * shipped  # 28 server checks, plus 2 style-neutral checks for each shipped workflow
    print(f"contract: {passed} of {total} passed")
    return 0 if passed == total and len(c.rows) == total else 1


if __name__ == "__main__":
    sys.exit(main())

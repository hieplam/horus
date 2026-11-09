#!/usr/bin/env python3
"""Make one flat-vector IT illustration with a local ComfyUI.

Command line:

    python scripts/run-illustration.py --subject "a message queue between two services" \
        --seed 7 --out out/queue.png

Python:

    from run_illustration import generate   # import with sys.path set to scripts/
    result = generate("a message queue between two services", seed=7, out="out/queue.png")

The prompt that FLUX 3 Image Klein receives is: house-style prefix + subject +
house-style suffix (workflows/it-illustration/house-style.json).

The script needs only the Python standard library and the horus/ package of this
repo (no third-party package). It reads the workflow in
ComfyUI "UI" format, converts it to the "API" format with the node definitions
of the running server (GET /object_info), submits it (POST /prompt), polls
GET /history, and downloads the PNG (GET /view).

The server URL comes from --url, or the environment variable HORUS_COMFYUI_URL,
or http://127.0.0.1:8188.

On success the CLI prints one JSON line and exits 0:
    {"ok": true, "prompt_id": ..., "out": ..., "seed": ..., "execution_seconds": ...}
On failure it prints {"ok": false, "error": ...} to stderr and exits 1.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))
from horus.comfy import ui_to_api  # noqa: E402  (the same converter as the MCP server)
from horus.errors import HorusError  # noqa: E402
WORKFLOW_DIR = ROOT / "workflows" / "it-illustration"
DEFAULT_WORKFLOW = WORKFLOW_DIR / "workflow.json"
DEFAULT_STYLE = WORKFLOW_DIR / "house-style.json"
DEFAULT_URL = "http://127.0.0.1:8188"

# Node classes that carry the settings this script changes.
ROLE_CLASSES = {
    "CLIPTextEncode": "prompt",
    "KSampler": "sampler",
    "EmptyFlux2LatentImage": "canvas",
    "SaveImage": "save",
}


class IllustrationError(RuntimeError):
    """A step failed. The message says which step and why."""


def _get_json(url: str, timeout: int = 30) -> dict:
    with urllib.request.urlopen(url, timeout=timeout) as resp:
        return json.loads(resp.read().decode("utf-8"))


def _post_json(url: str, body: dict, timeout: int = 60) -> dict:
    req = urllib.request.Request(
        url, data=json.dumps(body).encode("utf-8"), headers={"Content-Type": "application/json"}
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as err:
        raise IllustrationError(f"POST {url} failed with HTTP {err.code}: {err.read().decode('utf-8', 'replace')[:1500]}")


def build_prompt_text(subject: str, style: dict | None) -> str:
    """Join house-style prefix, subject and house-style suffix."""
    subject = " ".join(subject.split())
    if not subject:
        raise IllustrationError("the subject is empty")
    if style is None:
        return subject
    return f"{style['prefix']} {subject} {style['suffix']}"


def find_roles(workflow: dict) -> dict[str, str]:
    """Map each role (prompt, sampler, canvas, save) to its node id."""
    found: dict[str, str] = {}
    for node in workflow["nodes"]:
        role = ROLE_CLASSES.get(node["type"])
        if role:
            if role in found:
                raise IllustrationError(f"the workflow has two {node['type']} nodes")
            found[role] = str(node["id"])
    missing = sorted(set(ROLE_CLASSES.values()) - set(found))
    if missing:
        raise IllustrationError("the workflow has no node for: " + ", ".join(missing))
    return found


def generate(
    subject: str,
    seed: int = 424242,
    out: str | os.PathLike = "illustration.png",
    *,
    url: str | None = None,
    steps: int | None = None,
    width: int = 1536,
    height: int = 1024,
    use_style: bool = True,
    workflow_path: str | os.PathLike = DEFAULT_WORKFLOW,
    style_path: str | os.PathLike = DEFAULT_STYLE,
    timeout: int = 1800,
    poll_seconds: float = 5.0,
) -> dict:
    """Make one image and save it to `out`.

    Returns {"prompt_id", "out", "seed", "steps", "width", "height",
    "execution_seconds", "comfy_output"}. Raises IllustrationError on failure.
    """
    url = (url or os.environ.get("HORUS_COMFYUI_URL") or DEFAULT_URL).rstrip("/")
    if width % 16 or height % 16:
        raise IllustrationError("width and height must be multiples of 16")
    style = json.loads(Path(style_path).read_text(encoding="utf-8")) if use_style else None
    prompt_text = build_prompt_text(subject, style)

    workflow = json.loads(Path(workflow_path).read_text(encoding="utf-8"))
    roles = find_roles(workflow)
    try:
        object_info = _get_json(f"{url}/object_info", timeout=60)
    except (urllib.error.URLError, OSError) as err:
        raise IllustrationError(f"cannot reach ComfyUI at {url}: {err}")
    api = ui_to_api(workflow, object_info)
    expected = sum(1 for n in workflow["nodes"] if n["type"] not in ("Note", "MarkdownNote"))
    if len(api) != expected:
        raise IllustrationError(f"converted {len(api)} of {expected} nodes")

    prefix = f"horus-it-illustration-{int(time.time())}"
    api[roles["prompt"]]["inputs"]["text"] = prompt_text
    api[roles["sampler"]]["inputs"]["seed"] = seed
    if steps is not None:
        api[roles["sampler"]]["inputs"]["steps"] = steps
    api[roles["canvas"]]["inputs"]["width"] = width
    api[roles["canvas"]]["inputs"]["height"] = height
    api[roles["save"]]["inputs"]["filename_prefix"] = prefix

    reply = _post_json(f"{url}/prompt", {"prompt": api, "client_id": str(uuid.uuid4())})
    if reply.get("node_errors"):
        raise IllustrationError("ComfyUI rejected the graph: " + json.dumps(reply["node_errors"])[:1500])
    prompt_id = reply.get("prompt_id")
    if not prompt_id:
        raise IllustrationError("ComfyUI returned no prompt_id: " + json.dumps(reply)[:500])

    deadline = time.time() + timeout
    entry = None
    while time.time() < deadline:
        entry = _get_json(f"{url}/history/{prompt_id}").get(prompt_id)
        status = (entry or {}).get("status", {})
        if status.get("status_str") == "error":
            raise IllustrationError(f"job {prompt_id} failed: " + json.dumps(status.get("messages"))[-1500:])
        if status.get("completed"):
            break
        time.sleep(poll_seconds)
    else:
        raise IllustrationError(f"job {prompt_id} did not finish in {timeout} s; it can still run in ComfyUI")

    images = [img for node in entry.get("outputs", {}).values() for img in node.get("images", [])]
    if not images:
        raise IllustrationError(f"job {prompt_id} finished without an image")
    img = images[0]
    query = urllib.parse.urlencode(
        {"filename": img["filename"], "subfolder": img.get("subfolder", ""), "type": img.get("type", "output")}
    )
    out_path = Path(out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    with urllib.request.urlopen(f"{url}/view?{query}", timeout=120) as resp:
        out_path.write_bytes(resp.read())

    stamps = {
        m[0]: m[1].get("timestamp")
        for m in entry["status"].get("messages", [])
        if isinstance(m, list) and len(m) == 2 and isinstance(m[1], dict)
    }
    seconds = None
    if stamps.get("execution_start") and stamps.get("execution_success"):
        seconds = round((stamps["execution_success"] - stamps["execution_start"]) / 1000, 1)
    return {
        "prompt_id": prompt_id,
        "out": str(out_path),
        "seed": seed,
        "steps": api[roles["sampler"]]["inputs"]["steps"],
        "width": width,
        "height": height,
        "execution_seconds": seconds,
        "comfy_output": img["filename"],
    }


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description="Make one flat-vector IT illustration with a local ComfyUI.")
    src = p.add_mutually_exclusive_group(required=True)
    src.add_argument("--subject", help="what the picture shows: objects, positions, relations")
    src.add_argument("--subject-file", help="a text file that holds the subject")
    p.add_argument("--out", default="illustration.png", help="where to save the PNG")
    p.add_argument("--seed", type=int, default=424242)
    p.add_argument("--steps", type=int, default=None, help="default: the value in the workflow (15)")
    p.add_argument("--width", type=int, default=1536)
    p.add_argument("--height", type=int, default=1024)
    p.add_argument("--no-style", action="store_true", help="send the subject without the house style")
    p.add_argument("--url", default=None, help="ComfyUI URL (default: $HORUS_COMFYUI_URL or %s)" % DEFAULT_URL)
    p.add_argument("--workflow", default=str(DEFAULT_WORKFLOW))
    p.add_argument("--style-file", default=str(DEFAULT_STYLE))
    p.add_argument("--timeout", type=int, default=1800, help="seconds to wait for the job")
    a = p.parse_args(argv)
    try:
        subject = a.subject if a.subject is not None else Path(a.subject_file).read_text(encoding="utf-8")
        result = generate(
            subject, a.seed, a.out, url=a.url, steps=a.steps, width=a.width, height=a.height,
            use_style=not a.no_style, workflow_path=a.workflow, style_path=a.style_file, timeout=a.timeout,
        )
    except (IllustrationError, HorusError, OSError, ValueError) as err:
        print(json.dumps({"ok": False, "error": str(err)}), file=sys.stderr)
        return 1
    print(json.dumps({"ok": True, **result}))
    return 0


if __name__ == "__main__":
    sys.exit(main())

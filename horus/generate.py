"""generate_image: one image from one workflow.

The steps:
    1. Check the arguments against the workflow manifest (subject length, size, out_path).
    2. Make sure that ComfyUI answers. Start it when it does not (HORUS_AUTOSTART=0 turns this off).
    3. Convert workflow.json to the API format with the live node definitions.
    4. Refuse early when the server does not list a model file of the graph (WORKFLOW_NOT_INSTALLED).
    5. Set caption (house-style prefix + subject + suffix), seed, size and file prefix.
    6. Submit, then wait. Report progress every few seconds.
    7. Save the PNG to out_path.

A timeout or a cancelled call removes the job from the ComfyUI queue, so a
later call does not wait behind an orphan job.
"""

from __future__ import annotations

import asyncio
import json
import os
import random
import time
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Awaitable, Callable

from . import catalog, settings
from .catalog import Workflow
from .comfy import Comfy, execution_seconds, job_error, missing_model_files, ui_to_api
from .errors import HorusError

Report = Callable[[float, float | None, str], Awaitable[None]]
POLL_SECONDS = 2.0
REPORT_SECONDS = 10.0


async def _noop(progress: float, total: float | None, message: str) -> None:
    return None


def build_caption(subject: str, style: dict[str, str] | None) -> str:
    if style is None:
        return subject
    return f"{style['prefix']} {subject} {style['suffix']}"


def validate(w: Workflow, subject: Any, seed: Any, width: Any, height: Any,
             out_path: Any, overwrite: bool) -> dict[str, Any]:
    out = w.manifest["output"]
    rules = w.manifest["subject"]
    if not isinstance(subject, str):
        raise HorusError("INVALID_ARGUMENT", "subject must be a string", {"field": "subject"})
    subject = " ".join(subject.split())
    lo, hi = rules.get("min_chars", 1), rules.get("max_chars", 4000)
    if not lo <= len(subject) <= hi:
        raise HorusError("INVALID_ARGUMENT", f"subject has {len(subject)} characters; allowed: {lo} to {hi}",
                         {"field": "subject", "min_chars": lo, "max_chars": hi, "got": len(subject),
                          "rules": rules.get("rules", [])})
    size = {}
    for field, value, default in (("width", width, out["default_width"]), ("height", height, out["default_height"])):
        value = default if value is None else value
        if isinstance(value, bool) or not isinstance(value, int):
            raise HorusError("INVALID_ARGUMENT", f"{field} must be an integer", {"field": field})
        if not out["min_side"] <= value <= out["max_side"] or value % out["multiple_of"]:
            raise HorusError("INVALID_ARGUMENT", f"{field} {value} is not allowed",
                             {"field": field, "got": value, "min": out["min_side"], "max": out["max_side"],
                              "multiple_of": out["multiple_of"]})
        size[field] = value
    if seed is None:
        seed = random.randrange(0, 2**32)
    if isinstance(seed, bool) or not isinstance(seed, int) or not 0 <= seed < 2**64:
        raise HorusError("INVALID_ARGUMENT", "seed must be an integer from 0 to 2^64-1", {"field": "seed"})
    if out_path in (None, ""):
        stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
        path = settings.output_dir() / w.name / f"{stamp}-{seed}.png"
    else:
        if not isinstance(out_path, str):
            raise HorusError("INVALID_ARGUMENT", "out_path must be a string", {"field": "out_path"})
        path = Path(out_path)
        if not path.is_absolute():
            raise HorusError("INVALID_ARGUMENT", "out_path must be an absolute path", {"field": "out_path", "got": out_path})
        if path.suffix.lower() != ".png":
            raise HorusError("INVALID_ARGUMENT", "out_path must end with .png", {"field": "out_path", "got": out_path})
    if path.exists() and not overwrite:
        raise HorusError("OUTPUT_EXISTS", f"{path} exists", {"path": str(path)})
    return {"subject": subject, "seed": seed, "path": path, **size}


def _node_id(api: dict[str, Any], class_type: str) -> str:
    ids = [k for k, v in api.items() if v["class_type"] == class_type]
    if len(ids) != 1:
        raise HorusError("INTERNAL", f"the graph must have exactly one {class_type} node, it has {len(ids)}")
    return ids[0]


async def generate_image(workflow: str, subject: str, *, seed: int | None = None, width: int | None = None,
                         height: int | None = None, out_path: str | None = None, overwrite: bool = False,
                         report: Report = _noop, comfy: Comfy | None = None,
                         timeout: int | None = None) -> dict[str, Any]:
    t0 = time.monotonic()
    w = catalog.get(workflow)
    req = validate(w, subject, seed, width, height, out_path, overwrite)
    comfy = comfy or Comfy()
    timeout = timeout or settings.generate_timeout()

    if not await asyncio.to_thread(comfy.is_up):
        await report(0, None, "starting the image backend (ComfyUI); the first start takes 1 to 3 minutes")
    started = await asyncio.to_thread(comfy.ensure_up)
    object_info = await asyncio.to_thread(comfy.get_json, "/object_info", 60)

    graph = json.loads(w.graph_path.read_text(encoding="utf-8"))
    api = ui_to_api(graph, object_info)
    expected = w.manifest["graph"].get("node_count")
    if expected and len(api) != expected:
        raise HorusError("INTERNAL", f"converted {len(api)} of {expected} nodes", {"workflow": w.name})
    missing = missing_model_files(api, object_info)
    if missing:
        raise HorusError("WORKFLOW_NOT_INSTALLED", "the backend does not list model files of this workflow",
                         {"workflow": w.name, "missing": missing},
                         next_step=f"Call install_workflow with workflow={w.name!r}, then call generate_image again.")

    values = {
        "caption": build_caption(req["subject"], w.house_style()),
        "seed": req["seed"],
        "width": req["width"],
        "height": req["height"],
        "filename_prefix": f"horus-{w.name}-{int(time.time())}",
    }
    for key, where in w.manifest["graph"]["inputs"].items():
        api[_node_id(api, where["class_type"])]["inputs"][where["input"]] = values[key]

    prompt_id = await asyncio.to_thread(comfy.submit, api, str(uuid.uuid4()))
    await report(0, None, f"job {prompt_id} submitted")
    try:
        entry = await _wait(comfy, prompt_id, timeout, report, t0)
    except asyncio.CancelledError:
        await asyncio.to_thread(comfy.cancel, prompt_id)
        raise

    images = [img for node in entry.get("outputs", {}).values() for img in node.get("images", [])]
    if not images:
        raise HorusError("GENERATION_FAILED", "the job finished without an image", {"job_id": prompt_id})
    data = await asyncio.to_thread(comfy.view, images[0])
    path: Path = req["path"]
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(path.name + ".part")
    tmp.write_bytes(data)
    os.replace(tmp, path)
    return {
        "workflow": w.name,
        "path": str(path),
        "bytes": len(data),
        "width": req["width"],
        "height": req["height"],
        "seed": req["seed"],
        "seconds": round(time.monotonic() - t0, 1),
        "backend_seconds": execution_seconds(entry),
        "started_backend": started,
        "job_id": prompt_id,
    }


async def _wait(comfy: Comfy, prompt_id: str, timeout: int, report: Report, t0: float) -> dict[str, Any]:
    deadline = time.monotonic() + timeout
    last_report = 0.0
    absent_since: float | None = None
    while True:
        entry = await asyncio.to_thread(comfy.history, prompt_id)
        status = (entry or {}).get("status", {})
        if status.get("status_str") == "error":
            raise job_error(prompt_id, status)
        if entry and status.get("completed"):
            return entry
        now = time.monotonic()
        if now > deadline:
            await asyncio.to_thread(comfy.cancel, prompt_id)
            raise HorusError("GENERATION_TIMEOUT", f"the job did not finish in {timeout} s",
                             {"job_id": prompt_id, "timeout_seconds": timeout})
        if now - last_report >= REPORT_SECONDS or entry is None:
            state = await asyncio.to_thread(comfy.queue_state, prompt_id)
            if entry is None and state == "absent":
                absent_since = absent_since or now
                if now - absent_since > 30:
                    raise HorusError("GENERATION_FAILED", "the job left the backend queue without a result",
                                     {"job_id": prompt_id}, next_step="The backend restarted or someone cleared its queue. Call generate_image again.")
            else:
                absent_since = None
            if now - last_report >= REPORT_SECONDS:
                elapsed = round(now - t0)
                text = "rendering" if state == "running" else (
                    f"waiting in the backend queue, position {state.split(':')[1]}" if state.startswith("pending") else "finishing")
                await report(elapsed, None, f"{text}, {elapsed} s")
                last_report = now
        await asyncio.sleep(POLL_SECONDS)

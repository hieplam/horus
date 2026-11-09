"""HTTP client for the local ComfyUI, and the UI-to-API graph conversion.

Only the Python standard library. ComfyUI endpoints used:
    GET  /system_stats        is the server up
    GET  /object_info         node classes, their inputs, and the model files the server lists
    POST /prompt              submit a graph in API format, returns prompt_id
    GET  /history/<id>        status and outputs of a job
    GET  /queue               running and pending jobs
    POST /queue {"delete"}    remove a pending job
    POST /interrupt           stop the running job
    POST /free                unload models (before horus deletes model files)
    GET  /view                download an output image
"""

from __future__ import annotations

import json
import subprocess
import urllib.error
import urllib.parse
import urllib.request
from typing import Any

from . import settings
from .errors import HorusError

SIMPLE_TYPES = {"INT", "FLOAT", "STRING", "BOOLEAN", "COMBO"}
NOTE_TYPES = ("Note", "MarkdownNote")
MODEL_EXT = (".safetensors", ".sft", ".gguf", ".ckpt", ".pt", ".pth", ".bin", ".onnx")


class Comfy:
    def __init__(self, url: str | None = None) -> None:
        self.url = (url or settings.comfy_url()).rstrip("/")

    # -- HTTP -------------------------------------------------------------
    def _open(self, req: urllib.request.Request | str, timeout: float) -> bytes:
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                return resp.read()
        except urllib.error.HTTPError:
            raise
        except (urllib.error.URLError, OSError) as err:
            raise HorusError("BACKEND_UNAVAILABLE", f"ComfyUI does not answer at {self.url}", {"reason": str(err)})

    def get_json(self, path: str, timeout: float = 30) -> Any:
        return json.loads(self._open(self.url + path, timeout).decode("utf-8"))

    def post_json(self, path: str, body: dict[str, Any], timeout: float = 60) -> Any:
        req = urllib.request.Request(self.url + path, data=json.dumps(body).encode("utf-8"),
                                     headers={"Content-Type": "application/json"})
        raw = self._open(req, timeout)
        return json.loads(raw.decode("utf-8")) if raw.strip() else {}

    # -- server -----------------------------------------------------------
    def is_up(self) -> bool:
        try:
            self.get_json("/system_stats", timeout=3)
            return True
        except HorusError:
            return False

    def ensure_up(self) -> bool:
        """Return True when this call started the server. Raise BACKEND_UNAVAILABLE when it cannot."""
        if self.is_up():
            return False
        if not settings.autostart():
            raise HorusError("BACKEND_UNAVAILABLE", f"ComfyUI does not answer at {self.url}, and HORUS_AUTOSTART is off")
        script = settings.ROOT / "scripts" / "start-comfyui.ps1"
        proc = subprocess.run(
            [settings.powershell(), "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", str(script),
             "-ServerUrl", self.url],
            capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=420,
            creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0),
        )
        if proc.returncode != 0 or not self.is_up():
            tail = (proc.stdout + proc.stderr).strip().splitlines()[-8:]
            code = "BACKEND_NOT_INSTALLED" if any("was not found" in t for t in tail) else "BACKEND_UNAVAILABLE"
            raise HorusError(code, f"horus could not start ComfyUI at {self.url}", {"start_output_tail": tail})
        return True

    def free(self) -> None:
        try:
            self.post_json("/free", {"unload_models": True, "free_memory": True}, timeout=30)
        except (HorusError, urllib.error.HTTPError):
            pass

    # -- jobs -------------------------------------------------------------
    def submit(self, api: dict[str, Any], client_id: str) -> str:
        try:
            reply = self.post_json("/prompt", {"prompt": api, "client_id": client_id})
        except urllib.error.HTTPError as err:
            body = err.read().decode("utf-8", "replace")
            try:
                reply = json.loads(body)
            except ValueError:
                raise HorusError("GENERATION_FAILED", f"ComfyUI rejected the job with HTTP {err.code}",
                                 {"backend_message": body[:1500]})
        if reply.get("node_errors"):
            raise rejected_graph_error(reply)
        if reply.get("error"):
            raise rejected_graph_error(reply)
        prompt_id = reply.get("prompt_id")
        if not prompt_id:
            raise HorusError("INTERNAL", "ComfyUI returned no prompt_id", {"reply": json.dumps(reply)[:500]})
        return prompt_id

    def history(self, prompt_id: str) -> dict[str, Any] | None:
        return self.get_json(f"/history/{prompt_id}").get(prompt_id)

    def queue_state(self, prompt_id: str) -> str:
        """running, pending (with position) or absent."""
        q = self.get_json("/queue", timeout=10)
        if any(item[1] == prompt_id for item in q.get("queue_running", [])):
            return "running"
        pending = [item[1] for item in sorted(q.get("queue_pending", []), key=lambda i: i[0])]
        if prompt_id in pending:
            return f"pending:{pending.index(prompt_id) + 1}"
        return "absent"

    def cancel(self, prompt_id: str) -> None:
        try:
            state = self.queue_state(prompt_id)
            if state == "running":
                self.post_json("/interrupt", {"prompt_id": prompt_id}, timeout=10)
            elif state.startswith("pending"):
                self.post_json("/queue", {"delete": [prompt_id]}, timeout=10)
        except (HorusError, urllib.error.HTTPError, ValueError):
            pass

    def view(self, image: dict[str, Any]) -> bytes:
        query = urllib.parse.urlencode(
            {"filename": image["filename"], "subfolder": image.get("subfolder", ""), "type": image.get("type", "output")})
        return self._open(f"{self.url}/view?{query}", 120)


def rejected_graph_error(reply: dict[str, Any]) -> HorusError:
    """Map a /prompt rejection to a horus error. A model file that the server does not list
    means the workflow is not installed (or the backend has not seen the new file)."""
    text = json.dumps(reply)
    missing = []
    for node in (reply.get("node_errors") or {}).values():
        for e in node.get("errors", []):
            info = e.get("extra_info") or {}
            value = str(info.get("received_value", ""))
            if e.get("type") == "value_not_in_list" and value.lower().endswith(MODEL_EXT):
                missing.append(value)
    if missing:
        return HorusError("WORKFLOW_NOT_INSTALLED", "the backend does not list model files of this workflow",
                          {"missing": missing})
    return HorusError("GENERATION_FAILED", "ComfyUI rejected the graph", {"backend_message": text[:1500]}, retryable=False,
                      next_step="This is a defect of the workflow, not of your call. Report details.backend_message to the user.")


def ui_to_api(workflow: dict[str, Any], object_info: dict[str, Any]) -> dict[str, Any]:
    """Convert a ComfyUI UI-format graph to the API format.

    For each node, the widget values (widgets_values) are matched to the input
    names in the order that the server lists them in /object_info. Linked inputs
    become [source_node_id, source_slot].
    """
    links = {link[0]: link for link in workflow.get("links", [])}
    api: dict[str, dict[str, Any]] = {}
    missing = sorted({n["type"] for n in workflow["nodes"] if n["type"] not in NOTE_TYPES and n["type"] not in object_info})
    if missing:
        raise HorusError("NODE_PACK_MISSING", "the backend lacks node classes of this workflow", {"missing": missing})
    for node in workflow["nodes"]:
        class_type = node["type"]
        if class_type in NOTE_TYPES:
            continue
        info = object_info[class_type]
        inputs: dict[str, Any] = {}
        linked = set()
        for slot in node.get("inputs", []):
            if slot.get("link") is not None:
                link = links[slot["link"]]
                inputs[slot["name"]] = [str(link[1]), link[2]]
                linked.add(slot["name"])
        values = list(node.get("widgets_values", []))
        specs = {**info["input"].get("required", {}), **info["input"].get("optional", {})}
        for name, spec in specs.items():
            kind = spec[0]
            opts = spec[1] if len(spec) > 1 and isinstance(spec[1], dict) else {}
            if not (isinstance(kind, list) or kind in SIMPLE_TYPES):
                continue  # a connection type such as MODEL or CLIP, never a widget
            if not values:
                break
            value = values.pop(0)
            if opts.get("control_after_generate") and values:
                values.pop(0)  # the UI stores "fixed" or "randomize" after the seed
            if name not in linked:
                inputs[name] = value
        api[str(node["id"])] = {"class_type": class_type, "inputs": inputs}
    return api


def missing_model_files(api: dict[str, Any], object_info: dict[str, Any]) -> list[str]:
    """Model file names in the graph that the server does not list in its options."""
    missing = []
    for node in api.values():
        specs = {**object_info[node["class_type"]]["input"].get("required", {}),
                 **object_info[node["class_type"]]["input"].get("optional", {})}
        for name, value in node["inputs"].items():
            if not isinstance(value, str) or not value.lower().endswith(MODEL_EXT):
                continue
            options = _combo_options(specs.get(name))
            if options is not None and value not in options:
                missing.append(value)
    return missing


def _combo_options(spec: Any) -> list[Any] | None:
    """The allowed values of a COMBO input: [[a, b], {...}] (old form) or ["COMBO", {"options": [a, b]}]."""
    if not spec:
        return None
    if isinstance(spec[0], list):
        return spec[0]
    if len(spec) > 1 and isinstance(spec[1], dict) and isinstance(spec[1].get("options"), list):
        return spec[1]["options"]
    return None


def job_error(prompt_id: str, status: dict[str, Any]) -> HorusError:
    """Map a failed job in /history to a horus error."""
    msgs = {m[0]: m[1] for m in status.get("messages", []) if isinstance(m, list) and len(m) == 2}
    if "execution_interrupted" in msgs:
        return HorusError("CANCELLED", "the job was interrupted on the backend", {"job_id": prompt_id})
    err = msgs.get("execution_error", {}) or {}
    etype = str(err.get("exception_type", ""))
    emsg = str(err.get("exception_message", ""))
    facts = {"job_id": prompt_id, "node_type": err.get("node_type"), "exception_type": etype,
             "backend_message": emsg[-1500:]}
    low = (etype + " " + emsg).lower()
    if "outofmemory" in low.replace(" ", "") or "out of memory" in low or "allocation on device" in low:
        return HorusError("OUT_OF_MEMORY", "the GPU ran out of memory", facts)
    if "cuda driver version is insufficient" in low:
        return HorusError("GENERATION_FAILED", "the NVIDIA driver is too old for a kernel of this workflow", facts,
                          retryable=False, next_step="Ask the user to update the NVIDIA driver (docs/why-flux3-klein.md in the horus repo).")
    return HorusError("GENERATION_FAILED", "the backend failed while it ran the job", facts)


def execution_seconds(entry: dict[str, Any]) -> float | None:
    stamps = {m[0]: m[1].get("timestamp") for m in entry.get("status", {}).get("messages", [])
              if isinstance(m, list) and len(m) == 2 and isinstance(m[1], dict)}
    if stamps.get("execution_start") and stamps.get("execution_success"):
        return round((stamps["execution_success"] - stamps["execution_start"]) / 1000, 1)
    return None

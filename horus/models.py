"""Install, check and remove the model files of a workflow.

This module runs scripts/workflow.ps1 with -Json and reads its last line
"HORUS_JSON {...}". The PowerShell script stays the one implementation of the
file rules: where a model file lives, the SHA-256 check, the token handling
(the token never passes through Python), the refusal to overwrite or delete a
file that differs from models.json.
"""

from __future__ import annotations

import json
import os
import subprocess
import threading
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Callable

from . import settings
from .catalog import Workflow
from .errors import HorusError

SCRIPT = settings.ROOT / "scripts" / "workflow.ps1"
MARK = "HORUS_JSON "


@dataclass
class Run:
    """One run of workflow.ps1. The reader threads fill it while the script runs."""

    lines: list[str] = field(default_factory=list)
    stderr: list[str] = field(default_factory=list)
    result: dict[str, Any] | None = None
    exit_code: int | None = None
    current_dest: str | None = None  # the file that install downloads now
    proc: subprocess.Popen | None = None

    def cancel(self) -> None:
        if self.proc and self.proc.poll() is None:
            # /T also stops curl.exe. The .part file stays, so the next install resumes it.
            subprocess.run(["taskkill", "/PID", str(self.proc.pid), "/T", "/F"], capture_output=True)


def _command(action: str, name: str | None, extra: list[str]) -> list[str]:
    cmd = [settings.powershell(), "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
           "-File", str(SCRIPT), action]
    if name:
        cmd.append(name)
    cmd += ["-Json", "-WorkflowsDir", str(settings.workflows_dir())]
    return cmd + extra


def run_script(action: str, name: str | None = None, extra: list[str] | None = None,
               run: Run | None = None, on_line: Callable[[str], None] | None = None) -> Run:
    """Run workflow.ps1 and wait. Fills and returns `run`."""
    run = run or Run()
    proc = subprocess.Popen(
        _command(action, name, extra or []), stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        stdin=subprocess.DEVNULL, cwd=str(settings.ROOT), env=os.environ.copy(),
        creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0),
    )
    run.proc = proc

    def read_err() -> None:
        for raw in proc.stderr:  # type: ignore[union-attr]
            run.stderr.append(raw.decode("utf-8", "replace").rstrip())

    t = threading.Thread(target=read_err, daemon=True)
    t.start()
    for raw in proc.stdout:  # type: ignore[union-attr]
        line = raw.decode("utf-8", "replace").rstrip("\r\n")
        if line.startswith(MARK):
            run.result = json.loads(line[len(MARK):])
            continue
        run.lines.append(line)
        if line.startswith("-- ") and " -> " in line:
            run.current_dest = line.split(" -> ", 1)[1].strip()
        if on_line:
            on_line(line)
    run.exit_code = proc.wait()
    t.join(timeout=5)
    return run


def _result(run: Run, action: str) -> dict[str, Any]:
    if run.result is None:
        tail = (run.lines + run.stderr)[-15:]
        raise HorusError("INTERNAL", f"workflow.ps1 {action} printed no result", {"exit_code": run.exit_code, "output_tail": tail})
    return run.result


def list_status() -> dict[str, dict[str, Any]]:
    """name -> {files, present, bytes}. A size check only, so it is fast."""
    run = run_script("list")
    text = "\n".join(run.lines).strip()
    if run.exit_code != 0 or not text.startswith("["):
        raise HorusError("INTERNAL", "workflow.ps1 list failed", {"output_tail": (run.lines + run.stderr)[-15:]})
    rows = json.loads(text)
    rows = rows if isinstance(rows, list) else [rows]
    return {r["name"]: r for r in rows}


def install_state(row: dict[str, Any] | None) -> str:
    if not row or not row.get("files"):
        return "not_installed"
    if row["present"] == row["files"]:
        return "installed"
    return "partially_installed" if row["present"] else "not_installed"


def check(w: Workflow, quick: bool = True) -> dict[str, Any]:
    return _result(run_script("check", w.name, ["-Quick"] if quick else []), "check")


def _download_error(item: dict[str, Any], stderr: str, w: Workflow) -> HorusError:
    text = f"{item.get('detail', '')}\n{stderr}"
    facts = {"file": item.get("file"), "path": item.get("path"), "detail": item.get("detail")}
    # curl prints "The requested URL returned error: 401" to stderr. The hint line of
    # download-model.ps1 names both codes, so it is not evidence of either.
    if "error: 401" in text:
        return HorusError("HF_TOKEN_INVALID", "Hugging Face refused the token (HTTP 401)", facts)
    if "error: 403" in text:
        return HorusError("LICENSE_NOT_ACCEPTED", "Hugging Face refused the download (HTTP 403)",
                          {**facts, "license": w.license()["name"], "license_url": w.license()["url"]})
    if "Not enough free space" in text:
        return HorusError("DISK_FULL", "not enough free disk space for the download", facts)
    if "Refusing to overwrite" in text:
        return HorusError("FILE_CONFLICT", "a different file with the same name is on disk", facts)
    return HorusError("DOWNLOAD_FAILED", "the download did not finish", facts)


def _check_error(items: list[dict[str, Any]], message: str) -> HorusError | None:
    by = lambda s: [i for i in items if i["status"] == s]  # noqa: E731
    if by("NO_NODE"):
        return HorusError("NODE_PACK_MISSING", "the backend lacks node classes of this workflow",
                          {"missing": [i["file"] for i in by("NO_NODE")]})
    if by("BAD_SIZE") or by("BAD_HASH"):
        bad = by("BAD_SIZE") + by("BAD_HASH")
        return HorusError("FILE_CONFLICT", "a model file on disk differs from models.json",
                          {"path": bad[0]["path"], "files": bad})
    if by("NOT_SEEN"):
        return HorusError("BACKEND_RESTART_REQUIRED", "the backend does not list the new model files yet",
                          {"files": [i["file"] for i in by("NOT_SEEN")]})
    if by("MISSING"):
        return HorusError("WORKFLOW_NOT_INSTALLED", "model files are missing",
                          {"missing": [i["file"] for i in by("MISSING")]})
    if by("UNLISTED"):
        return HorusError("INTERNAL", "the workflow loads a model file that models.json does not list",
                          {"files": [i["file"] for i in by("UNLISTED")]})
    if "ComfyUI was not found" in message:
        return HorusError("BACKEND_NOT_INSTALLED", message)
    return None


def install(w: Workflow, run: Run) -> dict[str, Any]:
    """Download and verify every model file, then check. Blocks; `run` shows the progress."""
    run_script("install", w.name, run=run)
    res = _result(run, "install")
    if res["ok"]:
        return res
    items = res.get("items", [])
    failed = [i for i in items if i["status"] == "FAILED"]
    if failed:
        raise _download_error(failed[0], "\n".join(run.stderr), w)
    err = _check_error(items, res.get("message", ""))
    raise err or HorusError("INTERNAL", res.get("message", "install failed"), {"items": items})


def remove(w: Workflow, confirm: bool, force_shared: bool) -> dict[str, Any]:
    extra = (["-Yes"] if confirm else []) + (["-Force"] if force_shared else [])
    res = _result(run_script("remove", w.name, extra), "remove")
    items = res.get("items", [])
    if res["ok"]:
        return res
    if [i for i in items if i["status"] == "DELETE_FAILED"]:
        raise HorusError("FILE_IN_USE", "some model files could not be deleted",
                         {"files": [i for i in items if i["status"] == "DELETE_FAILED"]})
    refused = [i for i in items if i["status"] == "REFUSED"]
    if refused:
        raise HorusError("FILE_CONFLICT", "a file on disk differs from models.json, so horus does not delete it",
                         {"path": refused[0]["path"], "files": refused})
    raise HorusError("INTERNAL", res.get("message", "remove failed"), {"items": items})


def part_size(dest: str | None) -> int:
    if not dest:
        return 0
    for p in (Path(dest + ".part"), Path(dest)):
        try:
            return p.stat().st_size
        except OSError:
            continue
    return 0

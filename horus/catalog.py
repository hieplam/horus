"""The workflows that horus offers.

A workflow is a folder workflows/<name>/ with:
    manifest.json     the public contract: summary, what it is good for,
                      output sizes, how to write the subject, and which graph
                      inputs horus sets (graph.inputs).
    workflow.json     the ComfyUI graph in UI format.
    models.json       the model files, with size and SHA-256.

A workflow is style-neutral. horus sends the caller's subject to the model as
written. The look, the palette and the design system belong to the caller, so a
manifest with "theme" or "caption" (a house style) is refused.

A folder without manifest.json (or with a name that starts with "_") is not
offered by the MCP server.
"""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from . import settings
from .errors import HorusError

NAME_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")
REQUIRED_INPUTS = ("caption", "seed", "width", "height", "filename_prefix")


@dataclass
class Workflow:
    name: str
    dir: Path
    manifest: dict[str, Any]

    @property
    def graph_path(self) -> Path:
        return self.dir / self.manifest["graph"]["file"]

    def models(self) -> dict[str, Any]:
        path = self.dir / "models.json"
        return json.loads(path.read_text(encoding="utf-8")) if path.is_file() else {"files": []}

    def license(self) -> dict[str, Any]:
        m = self.models()
        repo = m.get("repo")
        return {
            "name": m.get("license", "not stated"),
            "url": f"https://huggingface.co/{repo}" if repo else None,
            "download_bytes": sum(int(f.get("bytes", 0)) for f in m.get("files", [])),
        }


def _check_manifest(name: str, m: dict[str, Any]) -> None:
    missing = [k for k in ("name", "title", "summary", "output", "subject", "graph") if k not in m]
    if missing:
        raise ValueError(f"manifest.json of {name} lacks: {', '.join(missing)}")
    styled = [k for k in ("theme", "caption") if k in m]
    if styled:
        raise ValueError(f"manifest.json of {name} has {', '.join(styled)}: a workflow must not add a style; the caller owns it")
    if m["name"] != name:
        raise ValueError(f"manifest.json of {name} has name {m['name']!r}")
    absent = [k for k in REQUIRED_INPUTS if k not in m["graph"].get("inputs", {})]
    if absent:
        raise ValueError(f"manifest.json of {name}: graph.inputs lacks {', '.join(absent)}")


def all_workflows() -> list[Workflow]:
    root = settings.workflows_dir()
    out: list[Workflow] = []
    if not root.is_dir():
        return out
    for d in sorted(p for p in root.iterdir() if p.is_dir()):
        mf = d / "manifest.json"
        if d.name.startswith("_") or not mf.is_file():
            continue
        manifest = json.loads(mf.read_text(encoding="utf-8"))
        _check_manifest(d.name, manifest)
        out.append(Workflow(d.name, d, manifest))
    return out


def get(name: str) -> Workflow:
    if not isinstance(name, str) or not NAME_RE.match(name):
        raise HorusError("INVALID_ARGUMENT", f"invalid workflow name {name!r}", {"field": "workflow"})
    found = {w.name: w for w in all_workflows()}
    if name not in found:
        raise HorusError("WORKFLOW_NOT_FOUND", f"no workflow named {name!r}", {"known": sorted(found)})
    return found[name]


def summary(w: Workflow) -> dict[str, Any]:
    """The short view for list_workflows."""
    m = w.manifest
    out = m["output"]
    return {
        "name": w.name,
        "title": m["title"],
        "summary": m["summary"],
        "good_for": m.get("good_for", []),
        "not_good_for": m.get("not_good_for", []),
        "output": {"format": out["format"], "default_width": out["default_width"], "default_height": out["default_height"]},
        "typical_seconds": (m.get("timing") or {}).get("measured_seconds"),
    }


def detail(w: Workflow) -> dict[str, Any]:
    """The full view for describe_workflow. It has no ComfyUI details (graph, node names)."""
    m = w.manifest
    examples = []
    for ex in m["subject"].get("examples", []):
        img = w.dir / ex["image"] if ex.get("image") else None
        examples.append({**ex, "image": str(img) if img and img.is_file() else None})
    return {
        **summary(w),
        "output": m["output"],
        "subject": {**m["subject"], "examples": examples},
        "timing": m.get("timing"),
        "license": w.license(),
    }

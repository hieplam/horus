"""Settings of horus.

Each setting comes from, in this order: the environment variable, the .env file
in the repo root (the same file that the PowerShell scripts read), the default.
The .env file can hold HF_TOKEN. This module never returns or logs that token.
"""

from __future__ import annotations

import os
from pathlib import Path

ROOT = Path(os.environ.get("HORUS_ROOT") or Path(__file__).resolve().parent.parent)

_env_cache: dict[str, str] | None = None


def _env_file() -> dict[str, str]:
    global _env_cache
    if _env_cache is None:
        _env_cache = {}
        path = ROOT / ".env"
        if path.is_file():
            for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                key, value = line.split("=", 1)
                value = value.strip().strip('"').strip("'")
                if value:
                    _env_cache[key.strip()] = value
    return _env_cache


def get(name: str, default: str = "") -> str:
    if name == "HF_TOKEN":
        raise ValueError("horus code must not read the token; download-model.ps1 reads it")
    return os.environ.get(name) or _env_file().get(name) or default


def comfy_url() -> str:
    return get("HORUS_COMFYUI_URL", "http://127.0.0.1:8188").rstrip("/")


def comfy_dir() -> Path:
    return Path(get("HORUS_COMFYUI_DIR", str(Path.home() / "ComfyUI_windows_portable")))


def workflows_dir() -> Path:
    return Path(get("HORUS_WORKFLOWS_DIR", str(ROOT / "workflows")))


def output_dir() -> Path:
    return Path(get("HORUS_OUTPUT_DIR", str(ROOT / ".scratch" / "outputs")))


def autostart() -> bool:
    """Start ComfyUI when it does not answer. HORUS_AUTOSTART=0 turns this off."""
    return get("HORUS_AUTOSTART", "1") not in ("0", "false", "no")


def generate_timeout() -> int:
    return int(get("HORUS_GENERATE_TIMEOUT", "1800"))


def powershell() -> str:
    return get("HORUS_POWERSHELL", "powershell.exe")

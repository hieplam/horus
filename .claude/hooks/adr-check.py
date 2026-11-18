#!/usr/bin/env python3
"""Claude Code hook: check the decision records after a change in docs/adr.

.claude/settings.json runs this file after each Write or Edit. Claude Code gives
the tool call as JSON on standard input. When the changed file is in docs/adr,
this file runs `scripts/lint-public.py --adr`. On a finding it prints the
findings on standard error and exits 2, and Claude Code shows them to the agent.
For each other file it exits 0 and prints nothing.
"""
from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent


def main() -> int:
    try:
        call = json.load(sys.stdin)
    except ValueError:
        return 0
    name = (call.get("tool_input") or {}).get("file_path") or ""
    if not name:
        return 0
    path = Path(name)
    if not path.is_absolute():
        path = Path(call.get("cwd") or ROOT) / path
    try:
        path.resolve().relative_to(ROOT / "docs" / "adr")
    except ValueError:
        return 0
    run = subprocess.run(
        [sys.executable, str(ROOT / "scripts" / "lint-public.py"), "--adr"],
        capture_output=True, text=True,
    )
    if run.returncode == 0:
        return 0
    sys.stderr.write("The decision records in docs/adr have a problem (docs/adr/README.md gives the rules):\n")
    sys.stderr.write(run.stdout + run.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())

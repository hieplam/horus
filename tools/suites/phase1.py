"""Phase 1: the Go port (docs/specs/2026-11-10-go-port.md).

The 8 goal rows: contract, graph, workflow-cli, downloader, setup-commands, builds, real-image,
no-python. Unless --no-ci, the row contract needs the job "contract <os>" on windows-latest and
macos-latest of the newest ci.yml run on HEAD. The row real-image passes only with --real: a MCP
client starts the server command, calls generate_image for it-illustration with the subject of the
contract test and seed 7 at the default size, and reads the width and the height from the PNG header.
The server reads its settings from the environment and the .env file of its repo root (HORUS_ROOT),
and it can start ComfyUI. One image takes 3 to 8 minutes, longer after a cold start.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys

import goal_runner as r
from suites import common

TITLE = "the Go port"
SPEC = "docs/specs/2026-11-10-go-port.md"
REAL_WHERE = "on the Mac and on the PC"
SETUP_COMMANDS = ["installcomfyui", "startcomfyui", "installcomfymcp", "checksetup",
                  "syncworkflows", "newworkflow", "register", "runillustration"]
# The package of a setup command that moved out of internal/scripts in Phase 2.
SETUP_PACKAGES = {"runillustration": "internal/cli", "installcomfyui": "internal/backend", "startcomfyui": "internal/backend",
                  "checksetup": "internal/doctor", "installcomfymcp": "internal/provision", "register": "internal/provision",
                  "syncworkflows": "internal/devtools", "newworkflow": "internal/devtools"}
BUILD_TARGETS = [("windows", "amd64"), ("darwin", "arm64"), ("linux", "amd64")]


def row_graph(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail, out = r.go_test(["./internal/graph"], ["-v", "-run", "TestUIToAPIGolden"])
    if ok and "--- PASS: TestUIToAPIGolden" not in out:
        return False, "TestUIToAPIGolden did not run"
    return ok, "TestUIToAPIGolden passes" if ok else detail


def row_workflow_cli(a: argparse.Namespace) -> tuple[bool, str]:
    if not (r.ROOT / "tests" / "workflow_parity.py").is_file():
        return False, "missing: tests/workflow_parity.py"
    code, out = r.run([sys.executable, "tests/workflow_parity.py"])
    return code == 0, r.last_line(out)


def row_downloader(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail, out = r.go_test(["./internal/download"], ["-v"])
    if not ok:
        return False, detail
    n = len(re.findall(r"^--- PASS", out, re.M))
    return n >= 9, f"{n} tests passed" + ("" if n >= 9 else ", needs at least 9")


def row_setup(a: argparse.Namespace) -> tuple[bool, str]:
    moved = sorted({"./" + p for p in SETUP_PACKAGES.values()})
    ok, detail, _ = r.go_test(moved, [])
    if not ok:
        return False, detail
    go = r.go_exe()
    lacking = []
    for name in SETUP_COMMANDS:
        pkg = SETUP_PACKAGES.get(name, f"internal/scripts/{name}")
        code, out = r.run([go, "test", "-list", ".", f"./{pkg}"])
        if code != 0 or not re.search(r"^Test\w*", out, re.M):
            lacking.append(name)
    have = len(SETUP_COMMANDS) - len(lacking)
    if lacking:
        return False, f"{have} of 8 commands have a test; none for " + ", ".join(lacking)
    return True, "8 of 8 commands have a passing test"


def row_builds(a: argparse.Namespace) -> tuple[bool, str]:
    go = r.go_exe()
    if not go:
        return False, "go not found"
    if not (r.ROOT / "go.mod").is_file() or not (r.ROOT / "cmd" / "horus").is_dir():
        return False, "missing: go.mod or cmd/horus"
    built, failed = [], []
    for goos, goarch in BUILD_TARGETS:
        env = dict(os.environ, GOOS=goos, GOARCH=goarch, CGO_ENABLED="0")
        code, _ = r.run([go, "build", "-o", os.devnull, "./cmd/horus"], env=env)
        (built if code == 0 else failed).append(f"{goos}/{goarch}")
    if failed:
        return False, f"{len(built)} of 3 build; failed: " + ", ".join(failed)
    return True, "3 of 3 build"


def row_real(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail, _ = common.real_image(a)
    return ok, detail


def row_no_python(a: argparse.Namespace) -> tuple[bool, str]:
    code, out = r.run(["git", "ls-files"], timeout=60)
    if code != 0:
        return False, f"git ls-files failed: {r.last_line(out)}"
    files = out.splitlines()
    old = [f for f in files if re.fullmatch(r"horus/[^/]*\.py", f) or re.fullmatch(r"scripts/[^/]*\.ps1", f)]
    lines = 0
    for f in old:
        try:
            lines += (r.ROOT / f).read_bytes().count(b"\n")
        except OSError:
            pass
    problems = []
    if old:
        problems.append(f"{len(old)} files of Python and PowerShell remain")
    for f in files:
        if f.endswith(".go"):
            try:
                if re.search(r"powershell", (r.ROOT / f).read_text(encoding="utf-8", errors="replace"), re.I):
                    problems.append(f"{f} contains powershell")
            except OSError:
                pass
    try:
        entry = json.loads((r.ROOT / ".mcp.json").read_text(encoding="utf-8"))["mcpServers"]["horus"]
        if entry.get("command") != "horus" or entry.get("args") != ["mcp"]:
            problems.append(".mcp.json horus entry is not `horus mcp`")
    except (OSError, ValueError, KeyError, TypeError):
        problems.append(".mcp.json has no horus entry")
    if problems:
        return False, f"{lines} lines left; " + "; ".join(problems)
    return True, "0 lines left"


ROWS = ["contract", "graph", "workflow-cli", "downloader", "setup-commands", "builds", "real-image", "no-python"]
CHECKS = {"contract": lambda a: common.row_contract(a),"graph": row_graph, "workflow-cli": row_workflow_cli,
          "downloader": row_downloader, "setup-commands": row_setup, "builds": row_builds,
          "real-image": row_real, "no-python": row_no_python}

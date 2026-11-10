#!/usr/bin/env python3
"""Goal check of the Go port (epic: port horus to Go, Phase 1).

    python tools/goal_check.py [--no-ci] [--real] [--real-out <folder>] [--server-cmd <words...>]

Checks the 8 goal rows of docs/specs/2026-11-10-go-port.md. It prints one line
for each row, "PASS  <row> - <detail>" or "FAIL  <row> - <detail>", and then
"goal: N of 8 rows passed". The exit code is 0 only when N is 8.
A row whose tool or folder does not exist yet fails with a detail that says
what is missing. A row never raises. Standard library only, except --real:
the real-image row imports the MCP client (mcp==2.3.0) only when it runs.

--real makes one real image on the GPU machine. A MCP client starts the server
command, calls generate_image for it-illustration with the subject of the
contract test and seed 7 at the default size, and reads the width and the
height from the PNG header. The server reads its settings from the
environment and the .env file of its repo root (HORUS_ROOT), and it can start
ComfyUI. One image takes 3 to 8 minutes, longer after a cold start.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import struct
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
GO_DIRS = [r"C:\Program Files\Go\bin", "/usr/local/go/bin"]
SETUP_COMMANDS = ["installcomfyui", "startcomfyui", "installcomfymcp", "checksetup",
                  "syncworkflows", "newworkflow", "register", "runillustration"]
BUILD_TARGETS = [("windows", "amd64"), ("darwin", "arm64"), ("linux", "amd64")]
CI_OSES = ["windows-latest", "macos-latest"]
REAL_SUBJECT = ("On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt. "
                "On the right, a sky-blue server box takes them off the belt.")  # SUBJECT of tests/mcp_contract.py
REAL_SEED = 7
REAL_SIZE = (1536, 1024)  # the default size of workflows/it-illustration/manifest.json
REAL_TIMEOUT = 3600  # seconds; the server's own limit is HORUS_GENERATE_TIMEOUT (1800 s) plus the start of ComfyUI
PNG_SIGNATURE = bytes([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])  # the 8 bytes that start every PNG file
ROWS = ["contract", "graph", "workflow-cli", "downloader", "setup-commands", "builds", "real-image", "no-python"]


def run(cmd: list[str], env: dict | None = None, timeout: int = 900) -> tuple[int, str]:
    """Run a command in the repo root. Never raises: a missing program gives code 127."""
    try:
        p = subprocess.run(cmd, cwd=ROOT, env=env, capture_output=True, text=True,
                           encoding="utf-8", errors="replace", timeout=timeout)
        return p.returncode, (p.stdout or "") + (p.stderr or "")
    except FileNotFoundError:
        return 127, f"program not found: {cmd[0]}"
    except subprocess.TimeoutExpired:
        return 124, f"timeout after {timeout} s: {' '.join(cmd)}"
    except OSError as exc:
        return 126, f"{type(exc).__name__}: {exc}"


def go_exe() -> str | None:
    found = shutil.which("go")
    if found:
        return found
    for d in GO_DIRS:
        for name in ("go.exe", "go"):
            if (Path(d) / name).is_file():
                return str(Path(d) / name)
    return None


def last_line(text: str) -> str:
    lines = [ln.strip() for ln in text.splitlines() if ln.strip()]
    return lines[-1][:200] if lines else "no output"


def ci_detail(commit: str) -> tuple[bool, str]:
    """The newest run of ci.yml on this commit must have a successful contract job on each system."""
    if not shutil.which("gh"):
        return False, "CI: gh not found"
    code, out = run(["gh", "run", "list", "--workflow", "ci.yml", "--commit", commit,
                     "--json", "databaseId", "--limit", "1"], timeout=120)
    try:
        runs = json.loads(out) if code == 0 else []
    except ValueError:
        runs = []
    if not runs:
        return False, f"CI: no ci.yml run on commit {commit[:7]}"
    code, out = run(["gh", "run", "view", str(runs[0]["databaseId"]), "--json", "jobs"], timeout=120)
    try:
        jobs = json.loads(out).get("jobs", []) if code == 0 else []
    except ValueError:
        jobs = []
    missing = []
    for osname in CI_OSES:
        ok = any(j.get("name", "").startswith("contract") and osname in j.get("name", "")
                 and j.get("conclusion") == "success" for j in jobs)
        if not ok:
            missing.append(osname)
    if missing:
        return False, "CI: no successful contract job on " + ", ".join(missing)
    return True, "CI contract job green on " + " and ".join(CI_OSES)


def row_contract(a: argparse.Namespace) -> tuple[bool, str]:
    cmd = a.server_cmd or ["horus", "mcp"]
    if not shutil.which(cmd[0]) and not Path(cmd[0]).is_file():
        return False, f"server command not found: {cmd[0]}"
    code, out = run([sys.executable, "tests/mcp_contract.py", "--server-cmd", *cmd], timeout=600)
    m = re.search(r"contract: (\d+) of (\d+) passed", out)
    if not m:
        return False, f"contract test gave no result line ({last_line(out)})"
    local_ok = code == 0 and m.group(1) == m.group(2) == "30"
    detail = f"local {m.group(1)} of {m.group(2)}"
    if not local_ok:
        return False, detail + ", needs 30 of 30"
    if a.no_ci:
        return True, detail + "; CI part skipped (--no-ci)"
    _, sha = run(["git", "rev-parse", "HEAD"], timeout=30)
    ci_ok, ci = ci_detail(sha.strip())
    return ci_ok, f"{detail}; {ci}"


def go_test(pkgs: list[str], extra: list[str]) -> tuple[bool, str, str]:
    go = go_exe()
    if not go:
        return False, "go not found", ""
    missing = [p for p in pkgs if not p.endswith("...") and not (ROOT / p).is_dir()]
    if missing:
        return False, "missing: " + ", ".join(missing), ""
    if not (ROOT / "go.mod").is_file():
        return False, "missing: go.mod", ""
    code, out = run([go, "test", *extra, *pkgs])
    return code == 0, last_line(out), out


def row_graph(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail, out = go_test(["./internal/comfy"], ["-v", "-run", "TestUIToAPIGolden"])
    if ok and "--- PASS: TestUIToAPIGolden" not in out:
        return False, "TestUIToAPIGolden did not run"
    return ok, "TestUIToAPIGolden passes" if ok else detail


def row_workflow_cli(a: argparse.Namespace) -> tuple[bool, str]:
    if not (ROOT / "tests" / "workflow_parity.py").is_file():
        return False, "missing: tests/workflow_parity.py"
    code, out = run([sys.executable, "tests/workflow_parity.py"])
    return code == 0, last_line(out)


def row_downloader(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail, out = go_test(["./internal/scripts/downloadmodel"], ["-v"])
    if not ok:
        return False, detail
    n = len(re.findall(r"^--- PASS", out, re.M))
    return n >= 9, f"{n} tests passed" + ("" if n >= 9 else ", needs at least 9")


def row_setup(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail, _ = go_test(["./internal/scripts/..."], [])
    if not ok:
        return False, detail
    go = go_exe()
    lacking = []
    for name in SETUP_COMMANDS:
        code, out = run([go, "test", "-list", ".", f"./internal/scripts/{name}"])
        if code != 0 or not re.search(r"^Test\w*", out, re.M):
            lacking.append(name)
    have = len(SETUP_COMMANDS) - len(lacking)
    if lacking:
        return False, f"{have} of 8 commands have a test; none for " + ", ".join(lacking)
    return True, "8 of 8 commands have a passing test"


def row_builds(a: argparse.Namespace) -> tuple[bool, str]:
    go = go_exe()
    if not go:
        return False, "go not found"
    if not (ROOT / "go.mod").is_file() or not (ROOT / "cmd" / "horus").is_dir():
        return False, "missing: go.mod or cmd/horus"
    built, failed = [], []
    for goos, goarch in BUILD_TARGETS:
        env = dict(os.environ, GOOS=goos, GOARCH=goarch, CGO_ENABLED="0")
        code, _ = run([go, "build", "-o", os.devnull, "./cmd/horus"], env=env)
        (built if code == 0 else failed).append(f"{goos}/{goarch}")
    if failed:
        return False, f"{len(built)} of 3 build; failed: " + ", ".join(failed)
    return True, "3 of 3 build"


def png_size(path: Path) -> tuple[int, int] | None:
    """Width and height from the IHDR chunk of a PNG file, or None when the file is not a PNG."""
    try:
        with open(path, "rb") as f:
            head = f.read(24)
    except OSError:
        return None
    if len(head) < 24 or head[:8] != PNG_SIGNATURE or head[12:16] != b"IHDR":
        return None
    return struct.unpack(">II", head[16:24])


async def real_call(cmd: list[str], out: Path) -> dict:
    """Start the server with the MCP client and call generate_image one time. Returns the envelope."""
    from mcp import ClientSession, StdioServerParameters  # only --real needs the MCP client
    from mcp.client.stdio import stdio_client

    params = StdioServerParameters(command=cmd[0], args=cmd[1:], env=dict(os.environ))
    args = {"workflow": "it-illustration", "subject": REAL_SUBJECT, "seed": REAL_SEED, "out_path": str(out)}
    with open(out.with_suffix(".stderr.log"), "w", encoding="utf-8") as errlog:
        async with stdio_client(params, errlog=errlog) as (read, write):
            async with ClientSession(read, write) as s:
                await s.initialize()
                res = await s.call_tool("generate_image", args, read_timeout_seconds=float(REAL_TIMEOUT))
    if res.structured_content:
        return res.structured_content
    text = res.content[0].text if res.content else ""
    try:
        return json.loads(text)
    except ValueError:
        return {"ok": False, "error": {"code": "NOT_JSON", "message": text[:200]}}


def row_real(a: argparse.Namespace) -> tuple[bool, str]:
    if not a.real:
        return False, "needs --real on the GPU machine"
    try:
        import asyncio

        import mcp  # noqa: F401
    except ImportError:
        return False, "the MCP client is not installed (pip install mcp==2.3.0)"
    cmd = a.server_cmd or ["horus", "mcp"]
    if not shutil.which(cmd[0]) and not Path(cmd[0]).is_file():
        return False, f"server command not found: {cmd[0]}"
    folder = Path(a.real_out).resolve() if a.real_out else ROOT / ".scratch" / "real-image"
    folder.mkdir(parents=True, exist_ok=True)
    out = folder / f"real-image-seed{REAL_SEED}-{time.strftime('%Y%m%d-%H%M%S')}.png"
    t0 = time.monotonic()
    try:
        env = asyncio.run(real_call(cmd, out))
    except BaseExceptionGroup as group:  # the MCP client wraps each error in a task group
        while isinstance(group, BaseExceptionGroup) and group.exceptions:
            group = group.exceptions[0]
        return False, f"the MCP call failed: {type(group).__name__}: {str(group)[:160]}"
    took = time.monotonic() - t0
    if not env.get("ok"):
        err = env.get("error") or {}
        return False, f"generate_image failed: {err.get('code')}: {str(err.get('message'))[:160]}"
    path = Path((env.get("result") or {}).get("path") or "")
    size = png_size(path) if path.name else None
    if size is None:
        return False, f"generate_image returned ok, but {path} is not a PNG file"
    if size != REAL_SIZE:
        return False, f"the PNG is {size[0]}x{size[1]}, needs {REAL_SIZE[0]}x{REAL_SIZE[1]} ({path})"
    return True, f"{size[0]}x{size[1]} {path} (seed {REAL_SEED}, {took:.0f} s)"


def row_no_python(a: argparse.Namespace) -> tuple[bool, str]:
    code, out = run(["git", "ls-files"], timeout=60)
    if code != 0:
        return False, f"git ls-files failed: {last_line(out)}"
    files = out.splitlines()
    old = [f for f in files if re.fullmatch(r"horus/[^/]*\.py", f) or re.fullmatch(r"scripts/[^/]*\.ps1", f)]
    lines = 0
    for f in old:
        try:
            lines += (ROOT / f).read_bytes().count(b"\n")
        except OSError:
            pass
    problems = []
    if old:
        problems.append(f"{len(old)} files of Python and PowerShell remain")
    for f in files:
        if f.endswith(".go"):
            try:
                if re.search(r"powershell", (ROOT / f).read_text(encoding="utf-8", errors="replace"), re.I):
                    problems.append(f"{f} contains powershell")
            except OSError:
                pass
    try:
        entry = json.loads((ROOT / ".mcp.json").read_text(encoding="utf-8"))["mcpServers"]["horus"]
        if entry.get("command") != "horus" or entry.get("args") != ["mcp"]:
            problems.append(".mcp.json horus entry is not `horus mcp`")
    except (OSError, ValueError, KeyError, TypeError):
        problems.append(".mcp.json has no horus entry")
    if problems:
        return False, f"{lines} lines left; " + "; ".join(problems)
    return True, "0 lines left"


CHECKS = {"contract": row_contract, "graph": row_graph, "workflow-cli": row_workflow_cli,
          "downloader": row_downloader, "setup-commands": row_setup, "builds": row_builds,
          "real-image": row_real, "no-python": row_no_python}


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--server-cmd", nargs="+", help="server command of the contract row (default: horus mcp)")
    ap.add_argument("--no-ci", action="store_true", help="leave out the CI part of the contract row")
    ap.add_argument("--real", action="store_true", help="run the real-image row (GPU machine only)")
    ap.add_argument("--real-out", help="folder for the PNG of the real-image row (default: .scratch/real-image)")
    a = ap.parse_args()
    passed = 0
    for row in ROWS:
        try:
            ok, detail = CHECKS[row](a)
        except Exception as exc:  # a row never raises
            ok, detail = False, f"check failed to run: {type(exc).__name__}: {exc}"
        passed += ok
        print(f"{'PASS' if ok else 'FAIL'}  {row} - {detail}", flush=True)
    print(f"goal: {passed} of {len(ROWS)} rows passed")
    return 0 if passed == len(ROWS) else 1


if __name__ == "__main__":
    sys.exit(main())

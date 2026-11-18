"""The checks of this repo that more than one suite uses. This module is no suite.

The contract test (tests/mcp_contract.py), the parity test (tests/workflow_parity.py), the tests of
internal/archtest, a real image through the horus MCP server, and the ComfyUI and horus programs of
the real parts.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import struct
import sys
import time
from pathlib import Path

import goal_runner as r

CI_OSES = ["windows-latest", "macos-latest"]
# The CI jobs that the oracle of a row names. The names are the `name:` of the jobs of
# .github/workflows/ci.yml with the matrix filled in.
GO_JOBS = ["go windows/amd64", "go darwin/arm64"]  # they run go test ./..., so also the tree test of internal/platform
CONTRACT_JOBS = [f"contract {o}" for o in CI_OSES]
SAME_BEHAVIOUR_JOBS = (CONTRACT_JOBS + [f"workflow-parity {o}" for o in CI_OSES] + GO_JOBS)
CONTRACT_CHECKS = 37  # checks of tests/mcp_contract.py: 30 until Phase 4, 37 after task T5 of Phase 4 (setup through MCP)
PARITY_CASES = 17  # cases of tests/workflow_parity.py
ARCHTEST_DIR = "internal/archtest"
ARCHTEST_TESTS = ["TestRules", "TestPlanted", "TestProblems"]  # the tests that enforce the rules and prove them with planted files
STEPS_LINE = re.compile(r"^Plan: (\d+) step", re.M)  # the number of steps of the plan of horus setup
REAL_SUBJECT = ("On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt. "
                "On the right, a sky-blue server box takes them off the belt.")  # SUBJECT of tests/mcp_contract.py
REAL_SEED = 7
REAL_SIZE = (1536, 1024)  # the default size of workflows/it-illustration/manifest.json
REAL_TIMEOUT = 3600  # seconds; the server's own limit is HORUS_GENERATE_TIMEOUT (1800 s) plus the start of ComfyUI
PNG_SIGNATURE = bytes([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])  # the 8 bytes that start every PNG file


# ---------------------------------------------------------------------------------------------
# The contract test, the parity test, go test ./... and archtest.
# ---------------------------------------------------------------------------------------------

def server_missing(cmd: list[str]) -> bool:
    return not shutil.which(cmd[0]) and not Path(cmd[0]).is_file()


def row_contract(a: argparse.Namespace, ci_required: list[str] | None = None) -> tuple[bool, str]:
    """All CONTRACT_CHECKS checks of the contract test pass. Unless --no-ci, the CI jobs ci_required
    (default: a contract job on each system) are green on HEAD."""
    cmd = a.server_cmd or ["horus", "mcp"]
    if server_missing(cmd):
        return False, f"server command not found: {cmd[0]}"
    code, out = r.run([sys.executable, "tests/mcp_contract.py", "--server-cmd", *cmd], timeout=600)
    m = re.search(r"contract: (\d+) of (\d+) passed", out)
    if not m:
        return False, f"contract test gave no result line ({r.last_line(out)})"
    local_ok = code == 0 and m.group(1) == m.group(2) == str(CONTRACT_CHECKS)
    detail = f"local {m.group(1)} of {m.group(2)}"
    if not local_ok:
        return False, detail + f", needs {CONTRACT_CHECKS} of {CONTRACT_CHECKS}"
    if a.no_ci:
        return True, detail + "; CI part skipped (--no-ci)"
    ci_ok, ci = r.ci_part_lenient(ci_required or CONTRACT_JOBS)
    return ci_ok, f"{detail}; {ci}"


def parity_part(timeout: int = 900) -> tuple[bool, str]:
    """The parity test passes with all PARITY_CASES cases equal."""
    code, out = r.run([sys.executable, "tests/workflow_parity.py"], timeout=timeout)
    ok = code == 0 and re.search(rf"parity: {PARITY_CASES} of {PARITY_CASES} cases equal", out) is not None
    return ok, "parity: " + (f"{PARITY_CASES} of {PARITY_CASES}" if ok else r.last_line(out))


def go_test_all_part() -> tuple[bool, str]:
    """go test ./... passes."""
    go = r.go_exe()
    if not go:
        return False, "go not found"
    code, out = r.run([go, "test", "./..."])
    return code == 0, "go test ./...: " + ("ok" if code == 0 else "failed, " + r.last_line(out))


def allow_files() -> list[str]:
    """The allow list files of archtest that exist. Task T8 of Phase 2 removed the mechanism, so there must be none."""
    return sorted(p.name for p in (r.ROOT / ARCHTEST_DIR).glob("allow_*.txt"))


def archtest_part() -> tuple[bool, str]:
    """The three tests of internal/archtest pass, and no allow list file exists."""
    lists = allow_files()
    ok, detail, _ = r.go_test_ran([f"./{ARCHTEST_DIR}"], ARCHTEST_TESTS)
    return ok and not lists, f"archtest: {'ok' if ok else detail}, {len(lists)} allow list files"


def test_count_part(baseline: int) -> tuple[bool, str]:
    """The number of Go test functions is not below the baseline of the suite."""
    n, err = r.go_test_count()
    return n >= baseline, err if n < 0 else f"{n} Go test functions (baseline {baseline})"


def changed_files(base: str, paths: list[str]) -> list[str] | str:
    """Files of paths that differ from the commit base, or an error text.
    A rename with the same content (R100) is no change: a golden file moves with its package."""
    code, out = r.run(["git", "diff", "--name-status", "-M", base, "--", *paths], timeout=60)
    if code != 0:
        return f"git diff against {base} failed: {r.last_line(out)}"
    return [ln for ln in out.splitlines() if ln.strip() and not ln.startswith("R100")]


def changed_contract_files(base: str) -> list[str] | str:
    """Files of the contract (the two Python tests, fixtures and golden files) that differ from the baseline."""
    return changed_files(base, ["tests/mcp_contract.py", "tests/workflow_parity.py", "tests/fixtures", "*golden*"])


def changed_part(changed: list[str] | str, what: str) -> tuple[bool, str]:
    """The part of a row that needs no changed file. what names the files, for example "contract files"."""
    if isinstance(changed, str):
        return False, changed
    return not changed, f"{len(changed)} changed {what}" + (": " + ", ".join(changed[:3]) if changed else "")


# ---------------------------------------------------------------------------------------------
# A real image through the horus MCP server.
# ---------------------------------------------------------------------------------------------

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


def real_image(a: argparse.Namespace) -> tuple[bool, str, dict]:
    """Make the real image through the MCP server. Returns ok, the detail and the envelope."""
    if not a.real:
        return False, "needs --real on the GPU machine", {}
    try:
        import asyncio

        import mcp  # noqa: F401
    except ImportError:
        return False, "the MCP client is not installed (pip install mcp==2.3.0)", {}
    cmd = a.server_cmd or ["horus", "mcp"]
    if server_missing(cmd):
        return False, f"server command not found: {cmd[0]}", {}
    folder = Path(a.real_out).resolve() if a.real_out else r.ROOT / ".scratch" / "real-image"
    folder.mkdir(parents=True, exist_ok=True)
    out = folder / f"real-image-seed{REAL_SEED}-{time.strftime('%Y%m%d-%H%M%S')}.png"
    t0 = time.monotonic()
    try:
        env = asyncio.run(real_call(cmd, out))
    except BaseExceptionGroup as group:  # the MCP client wraps each error in a task group
        while isinstance(group, BaseExceptionGroup) and group.exceptions:
            group = group.exceptions[0]
        return False, f"the MCP call failed: {type(group).__name__}: {str(group)[:160]}", {}
    took = time.monotonic() - t0
    if not env.get("ok"):
        err = env.get("error") or {}
        return False, f"generate_image failed: {err.get('code')}: {str(err.get('message'))[:160]}", env
    path = Path((env.get("result") or {}).get("path") or "")
    size = png_size(path) if path.name else None
    if size is None:
        return False, f"generate_image returned ok, but {path} is not a PNG file", env
    if size != REAL_SIZE:
        return False, f"the PNG is {size[0]}x{size[1]}, needs {REAL_SIZE[0]}x{REAL_SIZE[1]} ({path})", env
    return True, f"{size[0]}x{size[1]} {path} (seed {REAL_SEED}, {took:.0f} s)", env


# ---------------------------------------------------------------------------------------------
# ComfyUI and the horus program, for the real parts.
# ---------------------------------------------------------------------------------------------

def comfy_url() -> str:
    return r.setting("HORUS_COMFYUI_URL", "http://127.0.0.1:8188").rstrip("/")


def answers(url: str) -> bool:
    """True when a ComfyUI answers /system_stats at url. The tests replace it."""
    import urllib.request
    try:
        with urllib.request.urlopen(url + "/system_stats", timeout=3) as resp:
            return resp.status == 200
    except Exception:
        return False


def wait_for(want: bool, url: str, seconds: int) -> bool:
    """Wait until answers(url) is want. True when it is, False after the time limit."""
    end = time.monotonic() + seconds
    while True:
        if answers(url) == want:
            return True
        if time.monotonic() >= end:
            return False
        time.sleep(1)


def horus_exe(a: argparse.Namespace) -> str:
    return (a.server_cmd or ["horus"])[0]


def doctor_run(a: argparse.Namespace, *args: str) -> tuple[int, str, int]:
    """horus doctor with args: (exit code, output, FAIL lines). On macOS a ComfyUI that does not
    answer is started first with horus start-comfyui, because the doctor checks the server."""
    exe = horus_exe(a)
    if r.this_os() == "darwin" and not answers(comfy_url()):
        r.run([exe, "start-comfyui"], timeout=900)
    code, out = r.run([exe, "doctor", *args], timeout=300)
    return code, out, len(re.findall(r"^FAIL\b", out, re.M))

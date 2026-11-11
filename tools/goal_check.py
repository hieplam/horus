#!/usr/bin/env python3
"""Goal check of the Go port (epic: port horus to Go, Phase 1).

    python tools/goal_check.py [--phase 2] [--no-ci] [--real] [--real-out <folder>] [--server-cmd <words...>] [--only <row...>]

Without --phase, it checks the 8 goal rows of Phase 1 (docs/specs/2026-11-10-go-port.md).
With --phase 2, it checks the 8 goal rows of Phase 2 (docs/specs/2026-11-11-restructure.md):
layout, same-behaviour, os-boundary, direction, process-tree, backend, token, commands.
The rows backend and commands pass only with --real, because their oracle needs the
owner's PC (the real image, horus doctor, horus setup).
The rows os-boundary, direction and token run go test ./internal/archtest. They pass
only when it passes and no allow list file internal/archtest/allow_<rule>.txt exists
(task T8 removed the allow list mechanism, so a new entry is not possible).
With --real, the row commands runs horus doctor and horus setup it-illustration (no -yes)
of the --server-cmd program. It fails on a FAIL line of doctor, on a step in the plan,
or when a file that the plan names changed.

It prints one line
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
# The package of a setup command that moved out of internal/scripts in Phase 2.
SETUP_PACKAGES = {"runillustration": "internal/cli", "installcomfyui": "internal/backend", "startcomfyui": "internal/backend",
                  "checksetup": "internal/doctor", "installcomfymcp": "internal/provision", "register": "internal/provision",
                  "syncworkflows": "internal/devtools", "newworkflow": "internal/devtools"}
BUILD_TARGETS = [("windows", "amd64"), ("darwin", "arm64"), ("linux", "amd64")]
CI_OSES = ["windows-latest", "macos-latest"]
REAL_SUBJECT = ("On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt. "
                "On the right, a sky-blue server box takes them off the belt.")  # SUBJECT of tests/mcp_contract.py
REAL_SEED = 7
REAL_SIZE = (1536, 1024)  # the default size of workflows/it-illustration/manifest.json
REAL_TIMEOUT = 3600  # seconds; the server's own limit is HORUS_GENERATE_TIMEOUT (1800 s) plus the start of ComfyUI
PNG_SIGNATURE = bytes([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])  # the 8 bytes that start every PNG file
ROWS = ["contract", "graph", "workflow-cli", "downloader", "setup-commands", "builds", "real-image", "no-python"]
ROWS2 = ["layout", "same-behaviour", "os-boundary", "direction", "process-tree", "backend", "token", "commands"]
PACKAGE_LIST = ROOT / "tools" / "phase2_packages.txt"
ARCHTEST_DIR = "internal/archtest"
BASELINE_COMMIT = "3e27da4"  # main before the first move task; the contract files are compared with it
BASELINE_TESTS = 157 # Go test functions at BASELINE_COMMIT (go test -list . ./...); same-behaviour never goes below it


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
    ok, detail, out = go_test(["./internal/graph"], ["-v", "-run", "TestUIToAPIGolden"])
    if ok and "--- PASS: TestUIToAPIGolden" not in out:
        return False, "TestUIToAPIGolden did not run"
    return ok, "TestUIToAPIGolden passes" if ok else detail


def row_workflow_cli(a: argparse.Namespace) -> tuple[bool, str]:
    if not (ROOT / "tests" / "workflow_parity.py").is_file():
        return False, "missing: tests/workflow_parity.py"
    code, out = run([sys.executable, "tests/workflow_parity.py"])
    return code == 0, last_line(out)


def row_downloader(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail, out = go_test(["./internal/download"], ["-v"])
    if not ok:
        return False, detail
    n = len(re.findall(r"^--- PASS", out, re.M))
    return n >= 9, f"{n} tests passed" + ("" if n >= 9 else ", needs at least 9")


def row_setup(a: argparse.Namespace) -> tuple[bool, str]:
    moved = sorted({"./" + p for p in SETUP_PACKAGES.values()})
    ok, detail, _ = go_test(moved, [])
    if not ok:
        return False, detail
    go = go_exe()
    lacking = []
    for name in SETUP_COMMANDS:
        pkg = SETUP_PACKAGES.get(name, f"internal/scripts/{name}")
        code, out = run([go, "test", "-list", ".", f"./{pkg}"])
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
    ok, detail, _ = real_image(a)
    return ok, detail


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
    if not shutil.which(cmd[0]) and not Path(cmd[0]).is_file():
        return False, f"server command not found: {cmd[0]}", {}
    folder = Path(a.real_out).resolve() if a.real_out else ROOT / ".scratch" / "real-image"
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


def go_files() -> list[str]:
    _, out = run(["git", "ls-files", "*.go"], timeout=60)
    return [f for f in out.splitlines() if f.endswith(".go")]


def count_taskkill() -> int:
    """Lines of Go files that name taskkill (the Windows program that ends a process tree)."""
    n = 0
    for f in go_files():
        try:
            n += len(re.findall(r"taskkill", (ROOT / f).read_text(encoding="utf-8", errors="replace"), re.I))
        except OSError:
            pass
    return n


def packages_off_target() -> tuple[list[str], list[str]] | str:
    """(packages that exist but are not on the list, packages on the list that do not exist), or an error text."""
    go = go_exe()
    if not go:
        return "go not found"
    if not PACKAGE_LIST.is_file():
        return f"missing: {PACKAGE_LIST.relative_to(ROOT).as_posix()}"
    code, out = run([go, "list", "./..."])
    if code != 0:
        return f"go list failed: {last_line(out)}"
    have = {ln.strip() for ln in out.splitlines() if ln.strip()}
    want = {ln.strip() for ln in PACKAGE_LIST.read_text(encoding="utf-8").splitlines() if ln.strip()}
    return sorted(have - want), sorted(want - have)


def row2_layout(a: argparse.Namespace) -> tuple[bool, str]:
    res = packages_off_target()
    if isinstance(res, str):
        return False, res
    extra, missing = res
    detail = f"{len(extra)} packages off target, {len(missing)} listed packages missing"
    return not extra and not missing, detail


def go_test_count() -> tuple[int, str]:
    go = go_exe()
    if not go:
        return -1, "go not found"
    code, out = run([go, "test", "-list", ".", "./..."])
    if code != 0:
        return -1, f"go test -list failed: {last_line(out)}"
    return len(re.findall(r"^Test\w+$", out, re.M)), ""


def changed_contract_files() -> list[str] | str:
    """Files of the contract (the two Python tests, fixtures and golden files) that differ from the baseline.
    A rename with the same content (R100) is no change: a golden file moves with its package."""
    code, out = run(["git", "diff", "--name-status", "-M", BASELINE_COMMIT, "--",
                     "tests/mcp_contract.py", "tests/workflow_parity.py", "tests/fixtures", "*golden*"], timeout=60)
    if code != 0:
        return f"git diff against {BASELINE_COMMIT} failed: {last_line(out)}"
    return [ln for ln in out.splitlines() if ln.strip() and not ln.startswith("R100")]


def row2_same(a: argparse.Namespace) -> tuple[bool, str]:
    parts, ok = [], True
    ok_c, detail_c = row_contract(a)
    ok &= ok_c
    parts.append("contract: " + detail_c)
    code, out = run([sys.executable, "tests/workflow_parity.py"], timeout=900)
    parity = re.search(r"parity: 17 of 17 cases equal", out) is not None and code == 0
    ok &= parity
    parts.append("parity: " + ("17 of 17" if parity else last_line(out)))
    go = go_exe()
    if go:
        code, out = run([go, "test", "./..."])
        ok &= code == 0
        parts.append("go test ./...: " + ("ok" if code == 0 else "failed, " + last_line(out)))
    else:
        ok = False
        parts.append("go not found")
    n, err = go_test_count()
    if n < 0:
        ok = False
        parts.append(err)
    else:
        ok &= n >= BASELINE_TESTS
        parts.append(f"{n} Go test functions (baseline {BASELINE_TESTS})")
    changed = changed_contract_files()
    if isinstance(changed, str):
        ok = False
        parts.append(changed)
    else:
        ok &= not changed
        parts.append(f"{len(changed)} changed contract files" + (": " + ", ".join(changed[:3]) if changed else ""))
    return ok, "; ".join(parts)


def allow_files() -> list[str]:
    """The allow list files of archtest that exist. Task T8 removed the mechanism, so there must be none."""
    return sorted(p.name for p in (ROOT / ARCHTEST_DIR).glob("allow_*.txt"))


def row2_archtest(rule: str):
    def check(a: argparse.Namespace) -> tuple[bool, str]:
        if not (ROOT / ARCHTEST_DIR).is_dir():
            return False, f"missing: {ARCHTEST_DIR} (task T1)"
        lists = allow_files()
        ok, detail, _ = go_test([f"./{ARCHTEST_DIR}"], [])
        text = f"{len(lists)} allow list files" + (" (" + ", ".join(lists) + ")" if lists else "")
        return ok and not lists, f"{text}; archtest: {'ok' if ok else detail}"
    return check


def row2_process_tree(a: argparse.Namespace) -> tuple[bool, str]:
    kills = count_taskkill()
    if not (ROOT / "internal" / "platform").is_dir():
        return False, f"missing: internal/platform (task T2); {kills} taskkill references"
    ok, detail, out = go_test(["./internal/platform"], ["-v", "-run", "Tree"])
    if ok and not re.search(r"^--- PASS: Test\w*Tree", out, re.M):
        return False, f"no passing test with Tree in its name in internal/platform; {kills} taskkill references"
    return ok and kills == 0, f"tree test: {'ok' if ok else detail}; {kills} taskkill references"


def row2_backend(a: argparse.Namespace) -> tuple[bool, str]:
    if not (ROOT / "internal" / "backend").is_dir():
        return False, "missing: internal/backend (task T7)"
    ok, detail, _ = go_test(["./internal/backend"], [])
    if not ok:
        return False, detail
    if not a.real:
        return False, "Go tests ok; needs --real on the owner's PC for the image through the backend"
    ok_r, detail_r, env = real_image(a)
    if ok_r and (env.get("result") or {}).get("started_backend") is not True:
        # The row proves a start through the backend: a ComfyUI that already answered proves nothing.
        return False, f"started_backend is not true: ComfyUI already answered; stop it first ({detail_r})"
    return ok_r, detail_r + ("; started through the backend" if ok_r else "")


SETUP_LINE = re.compile(r"^(PRESENT|MISSING)\s+(.*)$")  # a fact of the plan of horus setup
STEPS_LINE = re.compile(r"^Plan: (\d+) step", re.M)  # the number of steps of the plan


def setup_paths(out: str) -> list[Path]:
    """The paths that the plan of horus setup names: each model file, and main.py and
    extra_model_paths.yaml of the ComfyUI folder."""
    paths = []
    for ln in out.splitlines():
        m = SETUP_LINE.match(ln.strip())
        if not m:
            continue
        rest = m.group(2)
        if rest.startswith("ComfyUI in "):
            comfy = Path(rest[len("ComfyUI in "):]) / "ComfyUI"
            paths += [comfy / "main.py", comfy / "extra_model_paths.yaml"]
        elif ") " in rest:
            paths.append(Path(rest.split(") ", 1)[1]))
    return paths


def file_states(paths: list[Path]) -> list[tuple]:
    """(path, size, mtime) of each path, or (path, None, None) when it does not exist."""
    states = []
    for p in paths:
        try:
            st = p.stat()
            states.append((str(p), st.st_size, st.st_mtime_ns))
        except OSError:
            states.append((str(p), None, None))
    return states


def row2_commands(a: argparse.Namespace) -> tuple[bool, str]:
    missing = [p for p in ("internal/cli", "internal/doctor") if not (ROOT / p).is_dir()]
    if missing:
        return False, "missing: " + ", ".join(missing) + " (tasks T8 and T9)"
    ok, detail, _ = go_test(["./internal/cli", "./internal/doctor", "./internal/provision"], [])
    if not ok:
        return False, detail
    if not a.real:
        return False, "Go tests ok; needs --real on the owner's PC for horus doctor and horus setup"
    exe = (a.server_cmd or ["horus"])[0]
    # The plan first: it names the files that doctor and setup must not change.
    code, first = run([exe, "setup", "it-illustration"], timeout=300)
    paths = setup_paths(first)
    if code != 0 or not paths:
        return False, f"horus setup it-illustration: exit {code}, {len(paths)} paths in the plan: {last_line(first)}"
    before = file_states(paths)
    code, out = run([exe, "doctor"], timeout=300)
    fails = len(re.findall(r"^FAIL\b", out, re.M))
    if code != 0 or fails:
        return False, f"horus doctor: exit {code}, {fails} FAIL lines"
    code, second = run([exe, "setup", "it-illustration"], timeout=300)
    m = STEPS_LINE.search(second)
    steps = int(m.group(1)) if m else -1
    if code != 0 or steps != 0 or not re.search(r"nothing (is )?missing", second, re.I):
        return False, f"horus setup it-illustration: exit {code}, {steps} steps in the plan"
    changed = [b[0] for b, c in zip(before, file_states(paths)) if b != c]
    if changed or second != first:
        return False, "a file changed: " + (", ".join(changed) or "the plan text differs between two runs")
    present = sum(1 for _, size, _ in before if size is not None)
    return True, (f"horus doctor: 0 FAIL lines; horus setup it-illustration: 0 steps, nothing is missing; "
                  f"{present} of {len(paths)} named files present and unchanged")


CHECKS2 = {"layout": row2_layout, "same-behaviour": row2_same, "os-boundary": row2_archtest("os-boundary"),
           "direction": row2_archtest("direction"), "process-tree": row2_process_tree,
           "backend": row2_backend, "token": row2_archtest("token"), "commands": row2_commands}


CHECKS = {"contract": row_contract, "graph": row_graph, "workflow-cli": row_workflow_cli,
          "downloader": row_downloader, "setup-commands": row_setup, "builds": row_builds,
          "real-image": row_real, "no-python": row_no_python}


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--server-cmd", nargs="+", help="server command of the contract row (default: horus mcp)")
    ap.add_argument("--no-ci", action="store_true", help="leave out the CI part of the contract row")
    ap.add_argument("--real", action="store_true", help="run the real-image row (GPU machine only)")
    ap.add_argument("--real-out", help="folder for the PNG of the real-image row (default: .scratch/real-image)")
    ap.add_argument("--phase", type=int, choices=[1, 2], default=1,
                    help="goal rows to check: 1 = the Go port (default), 2 = the three layers")
    ap.add_argument("--only", nargs="+", metavar="ROW",
                    help="check only these rows, for example --only backend (the summary counts these rows)")
    a = ap.parse_args()
    rows, checks = (ROWS2, CHECKS2) if a.phase == 2 else (ROWS, CHECKS)
    if a.only:
        unknown = [r for r in a.only if r not in rows]
        if unknown:
            ap.error("unknown row(s) for this phase: " + ", ".join(unknown))
        rows = [r for r in rows if r in a.only]
    passed = 0
    for row in rows:
        try:
            ok, detail = checks[row](a)
        except Exception as exc:  # a row never raises
            ok, detail = False, f"check failed to run: {type(exc).__name__}: {exc}"
        passed += ok
        print(f"{'PASS' if ok else 'FAIL'}  {row} - {detail}", flush=True)
    print(f"goal: {passed} of {len(rows)} rows passed")
    return 0 if passed == len(rows) else 1


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""Goal check of the Go port (epic: port horus to Go, Phase 1).

    python tools/goal_check.py [--phase 2|3] [--no-ci] [--real] [--real-out <folder>] [--server-cmd <words...>] [--only <row...>]

With --phase 3, it checks the 8 goal rows of Phase 3 (docs/specs/2026-11-12-macos-backend.md):
mac-backend, mac-setup, mac-image, tcc-folders, out-of-memory, one-config, one-setup-doc,
same-behaviour. The rows that need Go code run the Go tests that PINNED3 pins by name. Each
pinned test must report "--- PASS" in go test -v -run of its package, and its body must
assert something. A missing package, a package with no test files, a missing name, a
skipped test and an empty body fail the row. Unless --no-ci, mac-backend needs the job
"go darwin/arm64" and same-behaviour the 6 jobs of Phase 2, and no other job of the run
may be red. Without --real, a row stands on its part without --real (mac-image fails:
it has no such part), its detail says "real part not run", and a note line follows the
summary. With --real on macOS, mac-backend, mac-setup, mac-image, tcc-folders and
one-config run horus and comfy on the Mac; on Windows, a row of the Mac fails and says
so, one-config runs horus doctor, and same-behaviour runs the Phase 2 rows backend and
commands with --real.

Without --phase, it checks the 8 goal rows of Phase 1 (docs/specs/2026-11-10-go-port.md).
With --phase 2, it checks the 8 goal rows of Phase 2 (docs/specs/2026-11-11-restructure.md):
layout, same-behaviour, os-boundary, direction, process-tree, backend, token, commands.
The rows backend and commands pass only with --real, because their oracle needs the
owner's PC (the real image, horus doctor, horus setup).
The rows os-boundary, direction and token run the three tests of internal/archtest
(TestRules, TestPlanted, TestProblems) with go test -v. They pass only when each of the
three reports "--- PASS", so an empty package, a missing test or a skipped test fails.
They also need that no allow list file internal/archtest/allow_<rule>.txt exists
(task T8 removed the allow list mechanism, so a new entry is not possible).
Unless --no-ci, the CI part reads the jobs of the newest ci.yml run on HEAD. The row contract
(Phase 1) needs "contract <os>" on windows-latest and macos-latest. The row same-behaviour needs
those, "workflow-parity <os>" on both, and the jobs "go windows/amd64" and "go darwin/arm64". The row
process-tree needs the two go jobs (they run the tree test). A job that is missing, still running,
cancelled or failed fails the row, and the detail names it. The process-tree row also needs
TestTreeOutlivesItsStarterAndEndsFromAnotherProcess to pass.
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
# The CI jobs that the oracle of a row names (docs/specs/2026-11-11-restructure.md). The names are
# the `name:` of the jobs of .github/workflows/ci.yml with the matrix filled in.
GO_JOBS = ["go windows/amd64", "go darwin/arm64"]  # they run go test ./..., so also the tree test of internal/platform
SAME_BEHAVIOUR_JOBS = ([f"contract {o}" for o in CI_OSES] + [f"workflow-parity {o}" for o in CI_OSES] + GO_JOBS)
PROCESS_TREE_JOBS = GO_JOBS
TREE_TEST = "TestTreeOutlivesItsStarterAndEndsFromAnotherProcess"  # check 1 and check 2 of the spec
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
ARCHTEST_TESTS = ["TestRules", "TestPlanted", "TestProblems"]  # the tests that enforce the rules and prove them with planted files
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


def ci_jobs(commit: str) -> tuple[list[dict] | None, str]:
    """The jobs of the newest run of ci.yml on this commit, or (None, why not)."""
    if not shutil.which("gh"):
        return None, "CI: gh not found"
    code, out = run(["gh", "run", "list", "--workflow", "ci.yml", "--commit", commit,
                     "--json", "databaseId", "--limit", "1"], timeout=120)
    try:
        runs = json.loads(out) if code == 0 else []
    except ValueError:
        runs = []
    if not runs:
        return None, f"CI: no ci.yml run on commit {commit[:7]}"
    code, out = run(["gh", "run", "view", str(runs[0]["databaseId"]), "--json", "jobs"], timeout=120)
    try:
        return json.loads(out).get("jobs", []) if code == 0 else [], ""
    except ValueError:
        return [], ""


def judge_jobs(jobs: list[dict], required: list[str]) -> tuple[bool, str]:
    """Pure judgement of a job list. Each required job name must be in the list with the conclusion
    "success". A job that is missing, still running, cancelled or failed fails, and the detail names it."""
    by_name = {j.get("name", ""): j for j in jobs}
    bad = []
    for name in required:
        job = by_name.get(name)
        if job is None:
            bad.append(f"{name} (missing)")
        elif job.get("conclusion") != "success":
            bad.append(f"{name} ({job.get('conclusion') or job.get('status') or 'unknown'})")
    if bad:
        return False, "CI: not green: " + ", ".join(bad)
    return True, f"CI: {len(required)} jobs green (" + ", ".join(required) + ")"


_CI_CACHE: dict[str, tuple[list[dict] | None, str]] = {}


def ci_detail(commit: str, required: list[str] | None = None) -> tuple[bool, str]:
    """The newest run of ci.yml on this commit must have every required job green.
    Default: a contract job on each system (the Phase 1 row contract)."""
    if commit not in _CI_CACHE:
        _CI_CACHE[commit] = ci_jobs(commit)
    jobs, why = _CI_CACHE[commit]
    if jobs is None:
        return False, why
    return judge_jobs(jobs, required or [f"contract {o}" for o in CI_OSES])


def current_ci_detail(required: list[str]) -> tuple[bool, str]:
    _, sha = run(["git", "rev-parse", "HEAD"], timeout=30)
    return ci_detail(sha.strip(), required)


def row_contract(a: argparse.Namespace, ci_required: list[str] | None = None) -> tuple[bool, str]:
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
    ci_ok, ci = current_ci_detail(ci_required or [f"contract {o}" for o in CI_OSES])
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


def go_test_ran(pkgs: list[str], names: list[str] | None = None) -> tuple[bool, str, str]:
    """Like go_test, but it also proves that tests ran. With names, each of them must report
    "--- PASS" (an empty package, a missing test and a skipped test fail). Without names,
    each package must have at least one test (a package with [no test files] fails)."""
    extra = ["-count=1", "-v"]
    if names:
        extra += ["-run", "^(" + "|".join(names) + ")$"]
    ok, detail, out = go_test(pkgs, extra)
    if not ok:
        return False, detail, out
    if names:
        lacking = [n for n in names if not re.search(rf"^--- PASS: {n} ", out, re.M)]
        if lacking:
            return False, "no passing test: " + ", ".join(lacking), out
        return True, f"{len(names)} named tests pass", out
    empty = [p for p in pkgs if not re.search(rf"^ok\s+\S*{re.escape(p.lstrip('./'))}\s", out, re.M)
             or not re.search(r"^--- PASS", out, re.M)]
    nopass = [ln for ln in out.splitlines() if "[no test files]" in ln]
    if nopass:
        return False, "no test files: " + "; ".join(ln.split()[1] for ln in nopass if len(ln.split()) > 1), out
    if not re.search(r"^--- PASS", out, re.M):
        return False, "no test passed", out
    return True, "tests pass", out


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


def changed_contract_files(base: str = BASELINE_COMMIT) -> list[str] | str:
    """Files of the contract (the two Python tests, fixtures and golden files) that differ from the baseline.
    A rename with the same content (R100) is no change: a golden file moves with its package."""
    code, out = run(["git", "diff", "--name-status", "-M", base, "--",
                     "tests/mcp_contract.py", "tests/workflow_parity.py", "tests/fixtures", "*golden*"], timeout=60)
    if code != 0:
        return f"git diff against {base} failed: {last_line(out)}"
    return [ln for ln in out.splitlines() if ln.strip() and not ln.startswith("R100")]


def row2_same(a: argparse.Namespace) -> tuple[bool, str]:
    parts, ok = [], True
    ok_c, detail_c = row_contract(a, SAME_BEHAVIOUR_JOBS)
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
        ok, detail, _ = go_test_ran([f"./{ARCHTEST_DIR}"], ARCHTEST_TESTS)
        text = f"{len(lists)} allow list files" + (" (" + ", ".join(lists) + ")" if lists else "")
        return ok and not lists, f"{text}; archtest: {'ok' if ok else detail}"
    return check


def row2_process_tree(a: argparse.Namespace) -> tuple[bool, str]:
    kills = count_taskkill()
    if not (ROOT / "internal" / "platform").is_dir():
        return False, f"missing: internal/platform (task T2); {kills} taskkill references"
    ok, detail, out = go_test(["./internal/platform"], ["-count=1", "-v", "-run", "Tree"])
    # The test with check 1 and check 2 must pass. Another test with Tree in its name is not enough.
    if ok and not re.search(rf"^--- PASS: {TREE_TEST} ", out, re.M):
        return False, f"{TREE_TEST} did not pass in internal/platform; {kills} taskkill references"
    detail = f"tree test: {'ok' if ok else detail}; {kills} taskkill references"
    if ok and kills == 0 and not a.no_ci:
        ci_ok, ci = current_ci_detail(PROCESS_TREE_JOBS)
        return ci_ok, f"{detail}; {ci}"
    if ok and kills == 0:
        detail += "; CI part skipped (--no-ci)"
    return ok and kills == 0, detail


def row2_backend(a: argparse.Namespace) -> tuple[bool, str]:
    if not (ROOT / "internal" / "backend").is_dir():
        return False, "missing: internal/backend (task T7)"
    ok, detail, _ = go_test_ran(["./internal/backend"])
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
    ok, detail, _ = go_test_ran(["./internal/cli", "./internal/doctor", "./internal/provision"])
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


# ---------------------------------------------------------------------------------------------
# Phase 3: horus on macOS through comfy-cli (docs/specs/2026-11-12-macos-backend.md).
# ---------------------------------------------------------------------------------------------

ROWS3 = ["mac-backend", "mac-setup", "mac-image", "tcc-folders", "out-of-memory", "one-config",
         "one-setup-doc", "same-behaviour"]
BASELINE_COMMIT3 = "943d430"  # main before the first task of Phase 3; the contract files are compared with it
BASELINE_TESTS3 = 215  # Go test functions at BASELINE_COMMIT3 (go test -list . ./...)
MAC_BACKEND_JOBS = ["go darwin/arm64"]  # the job that runs the Go tests of the comfy-cli backend on macOS
MPS_TEXT = "MPS backend out of memory"  # the start of the error text of PyTorch on MPS
TCC_FOLDERS = ["Desktop", "Documents", "Downloads"]  # the folders of <home> that TCC blocks
# The Go tests that the rows of Phase 3 pin: (package folder, test name, the task that writes it).
# Tasks T1 to T5 write tests with exactly these names. Each one must report "--- PASS" in
# go test -v -run, and its body must assert something (an empty body fails the row).
PIN_DEFAULT_DIR = [("internal/platform", "TestDefaultComfyDirOfEachOS", "T1"),
                   ("internal/settings", "TestComfyDirDefaultIsTheFolderOfPlatform", "T1")]
PINNED3 = {
    "mac-backend": [
        ("internal/backend", "TestComfyCLIIsTheBackendOfDarwin", "T1"),
        ("internal/backend", "TestComfyCLIInstallRunsTheCommandOfTheSpec", "T1"),
        ("internal/backend", "TestComfyCLIInstallSkipsWhenComfyUIIsInstalled", "T1"),
        ("internal/backend", "TestComfyCLIInstallNamesBothFixesWhenComfyIsMissing", "T1"),
        ("internal/backend", "TestComfyCLIStartRunsLaunchAndTheServerAnswers", "T2"),
        ("internal/backend", "TestComfyCLIStartFailsWhenComfyUIIsNotInstalled", "T2"),
        ("internal/backend", "TestComfyCLIStopRunsStopAndTheServerEnds", "T2"),
        ("internal/backend", "TestComfyCLIStopRefusesAComfyUIThatHorusDidNotStart", "T2"),
        ("internal/backend", "TestComfyCLIStopWithAStaleStateRunsNoStop", "T2"),
        ("cmd/horus", "TestGenerateStartsComfyUIThroughTheComfyCLIBackend", "T2"),
    ],
    "mac-setup": PIN_DEFAULT_DIR + [
        ("cmd/horus", "TestSetupYesWithTheFakeComfyThenASecondPlanHasZeroSteps", "T3"),
    ],
    "tcc-folders": PIN_DEFAULT_DIR[:1] + [
        ("internal/platform", "TestInTCCFolder", "T3"),
        ("internal/doctor", "TestTCCCheckWarnsForEachPathInATCCFolder", "T3"),
        ("internal/cli", "TestDoctorWarnLinesKeepExitCodeZero", "T3"),
    ],
    "out-of-memory": [
        ("internal/comfy", "TestJobErrorOfTheMPSOutOfMemoryText", "T4"),
        ("cmd/horus", "TestImageMPSOutOfMemoryThroughTheAdapter", "T4"),
    ],
    "one-config": [
        ("internal/cli", "TestDoctorUsesHorusComfyUIURLWithoutTheFlag", "T3"),
        ("cmd/horus", "TestComfyMCPPassesTheStreamsComfyBinAndTheExitCode", "T5"),
    ],
}
# A pinned test whose body must hold this text (the MPS text of the row out-of-memory).
PIN_TEXT = {"TestJobErrorOfTheMPSOutOfMemoryText": MPS_TEXT, "TestImageMPSOutOfMemoryThroughTheAdapter": MPS_TEXT}
ASSERTS = re.compile(r"\bt\.(Errorf?|Fatalf?|Fail|FailNow)\(|[(,]\s*t\s*[,)]")  # an assertion, or t given to a helper
SETUP_DOC_RED = re.compile(r"Two commands are|does steps 1 and 4")  # the first Red search of #91 (docs/setup.md)
HELP_RED = "then ~/ComfyUI_windows_portable"  # the second Red search of #91 (internal/cli)
MACOS_HEADING = re.compile(r"^#{1,6} .*\bmacOS\b", re.M)


def this_os() -> str:
    """sys.platform: "darwin" on macOS, "win32" on Windows. The tests replace it."""
    return sys.platform


def test_body(pkg: str, name: str) -> str | None:
    """The source of the body of the test function name in the _test.go files of pkg, or None."""
    for f in sorted((ROOT / pkg).glob("*_test.go")):
        try:
            text = f.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        m = re.search(rf"^func {re.escape(name)}\(\w+ \*testing\.T\)\s*\{{", text, re.M)
        if not m:
            continue
        depth, i = 1, m.end()
        while i < len(text) and depth:
            depth += {"{": 1, "}": -1}.get(text[i], 0)
            i += 1
        return text[m.end():i - 1]
    return None


def pinned_tests(pins: list[tuple[str, str, str]]) -> tuple[bool, str]:
    """Each pinned test must exist, assert something, hold its PIN_TEXT, and report "--- PASS" in
    go test -v -run of its package. A missing package, a package with no test files, a missing
    name, a skipped test and an empty body fail, and the detail names the test and its task."""
    go = go_exe()
    if not go:
        return False, "go not found"
    if not (ROOT / "go.mod").is_file():
        return False, "missing: go.mod"
    bad = []
    by_pkg: dict[str, list[tuple[str, str]]] = {}
    for pkg, name, task in pins:
        by_pkg.setdefault(pkg, []).append((name, task))
    for pkg, tests in by_pkg.items():
        if not (ROOT / pkg).is_dir():
            bad += [f"{n} ({pkg} is missing, {t})" for n, t in tests]
            continue
        names = [n for n, _ in tests]
        code, out = run([go, "test", "-count=1", "-v", "-run", "^(" + "|".join(names) + ")$", f"./{pkg}"])
        if "[no test files]" in out:
            bad += [f"{n} ({pkg} has no test files, {t})" for n, t in tests]
            continue
        for name, task in tests:
            body = test_body(pkg, name)
            if body is None:
                bad.append(f"{name} (not in {pkg}, {task})")
            elif not ASSERTS.search(body):
                bad.append(f"{name} (asserts nothing, {task})")
            elif PIN_TEXT.get(name) and PIN_TEXT[name] not in body:
                bad.append(f"{name} (has no text '{PIN_TEXT[name]}', {task})")
            elif not re.search(rf"^--- PASS: {name} ", out, re.M):
                bad.append(f"{name} (did not pass in {pkg}: {last_line(out)[:80]}, {task})")
    if bad:
        return False, "no passing test: " + "; ".join(bad)
    return True, f"{len(pins)} pinned tests pass"


def judge_jobs_strict(jobs: list[dict], required: list[str]) -> tuple[bool, str]:
    """judge_jobs, and also no other job of the run is red, cancelled or still running.
    A skipped or neutral job that the row does not name is no failure."""
    ok, detail = judge_jobs(jobs, required)
    other = [j for j in jobs if j.get("name") not in required
             and j.get("conclusion") not in ("success", "skipped", "neutral")]
    if other:
        names = ", ".join(f"{j.get('name')} ({j.get('conclusion') or j.get('status') or 'unknown'})" for j in other)
        return False, (detail + "; " if not ok else "CI: ") + "another job is not green: " + names
    return ok, detail


def ci_part3(a: argparse.Namespace, required: list[str]) -> tuple[bool, str]:
    if a.no_ci:
        return True, "CI part skipped (--no-ci)"
    _, sha = run(["git", "rev-parse", "HEAD"], timeout=30)
    sha = sha.strip()
    if sha not in _CI_CACHE:
        _CI_CACHE[sha] = ci_jobs(sha)
    jobs, why = _CI_CACHE[sha]
    if jobs is None:
        return False, why
    return judge_jobs_strict(jobs, required)


def setting(name: str, default: str) -> str:
    """A setting as horus reads it: the environment, then the .env file of the repo root, then the default."""
    if os.environ.get(name):
        return os.environ[name]
    try:
        for ln in (ROOT / ".env").read_text(encoding="utf-8").splitlines():
            key, sep, value = ln.strip().partition("=")
            if sep and key.strip() == name and value.strip():
                return value.strip().strip('"').strip("'")
    except OSError:
        pass
    return default


def comfy_url() -> str:
    return setting("HORUS_COMFYUI_URL", "http://127.0.0.1:8188").rstrip("/")


def answers(url: str) -> bool:
    """True when a ComfyUI answers /system_stats at url. The tests replace it."""
    import urllib.request
    try:
        with urllib.request.urlopen(url + "/system_stats", timeout=3) as r:
            return r.status == 200
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


def in_tcc_folder(path: str, home: str) -> bool:
    """True when path is inside <home>/Desktop, <home>/Documents or <home>/Downloads."""
    p, h = Path(path).resolve(), Path(home).resolve()
    return any(p == h / f or h / f in p.parents for f in TCC_FOLDERS)


def horus_exe(a: argparse.Namespace) -> str:
    return (a.server_cmd or ["horus"])[0]


def real_needs_mac() -> str | None:
    """None on macOS, else the detail of a real part that needs the Mac."""
    return None if this_os() == "darwin" else f"the real part needs macOS (this machine: {this_os()})"


def doctor_run(a: argparse.Namespace, *args: str) -> tuple[int, str, int]:
    """horus doctor with args: (exit code, output, FAIL lines). On macOS a ComfyUI that does not
    answer is started first with horus start-comfyui, because the doctor checks the server."""
    exe = horus_exe(a)
    if this_os() == "darwin" and not answers(comfy_url()):
        run([exe, "start-comfyui"], timeout=900)
    code, out = run([exe, "doctor", *args], timeout=300)
    return code, out, len(re.findall(r"^FAIL\b", out, re.M))


def with_real(a: argparse.Namespace, ok: bool, detail: str, real) -> tuple[bool, str]:
    """Join the part without --real and the real part. Without --real the row stands on its part
    without --real, as a row stands on its local part with --no-ci, and the detail says so."""
    if not ok:
        return False, detail
    if not a.real:
        return True, detail + "; real part not run (needs --real on macOS)"
    ok_r, detail_r = real(a)
    return ok_r, detail + "; real: " + detail_r


def real_mac_backend(a: argparse.Namespace) -> tuple[bool, str]:
    """Start and stop through horus, then a ComfyUI that a person starts with comfy, which
    horus stop-comfyui must refuse. The hand-started ComfyUI is stopped with comfy stop."""
    why = real_needs_mac()
    if why:
        return False, why
    exe, url = horus_exe(a), comfy_url()
    if answers(url):
        run([exe, "stop-comfyui"], timeout=300)
        if not wait_for(False, url, 60):
            return False, f"a ComfyUI that horus did not start answers at {url}; stop it first"
    code, out = run([exe, "start-comfyui"], timeout=900)
    if code != 0 or not answers(url):
        return False, f"horus start-comfyui: exit {code}, the server does not answer: {last_line(out)}"
    code, out = run([exe, "stop-comfyui"], timeout=300)
    if code != 0 or not wait_for(False, url, 60):
        return False, f"horus stop-comfyui: exit {code}, the server still answers: {last_line(out)}"
    comfy = shutil.which("comfy") or str(Path.home() / ".venvs" / "comfy-mcp" / "bin" / "comfy")
    workspace = str(Path(setting("HORUS_COMFYUI_DIR", str(Path.home() / "horus"))) / "ComfyUI")
    port = url.rsplit(":", 1)[-1]
    run([comfy, "--workspace", workspace, "launch", "--background", "--", "--listen", "127.0.0.1", "--port", port],
        timeout=600)
    if not wait_for(True, url, 300):
        return False, f"a ComfyUI started by hand with {comfy} does not answer at {url}"
    code, out = run([exe, "stop-comfyui"], timeout=300)
    still = answers(url)
    run([comfy, "--workspace", workspace, "stop"], timeout=300)
    wait_for(False, url, 60)
    if code == 0 or not still:
        return False, f"horus stop-comfyui did not refuse a ComfyUI that horus did not start (exit {code})"
    return True, "start, stop, and a refusal of a ComfyUI started by hand"


def row3_mac_backend(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail = pinned_tests(PINNED3["mac-backend"])
    if ok:
        ok_ci, ci = ci_part3(a, MAC_BACKEND_JOBS)
        ok, detail = ok_ci, f"{detail}; {ci}"
    return with_real(a, ok, detail, real_mac_backend)


def real_mac_setup(a: argparse.Namespace) -> tuple[bool, str]:
    why = real_needs_mac()
    if why:
        return False, why
    if os.environ.get("HORUS_COMFYUI_DIR") or setting("HORUS_COMFYUI_DIR", ""):
        return False, "HORUS_COMFYUI_DIR is set (environment or .env); the row needs the default folder"
    code, out, fails = doctor_run(a, "-workflow", "it-illustration")
    if code != 0 or fails:
        return False, f"horus doctor -workflow it-illustration: exit {code}, {fails} FAIL lines"
    code, out = run([horus_exe(a), "setup", "it-illustration"], timeout=300)
    m = STEPS_LINE.search(out)
    if code != 0 or not m or m.group(1) != "0":
        return False, f"horus setup it-illustration: exit {code}, {m.group(1) if m else 'no'} steps in the plan"
    return True, "horus doctor: 0 FAIL lines; Plan: 0 steps; no HORUS_COMFYUI_DIR"


def row3_mac_setup(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail = pinned_tests(PINNED3["mac-setup"])
    return with_real(a, ok, detail, real_mac_setup)


def row3_mac_image(a: argparse.Namespace) -> tuple[bool, str]:
    if not a.real:
        return False, "needs --real on macOS"
    why = real_needs_mac()
    if why:
        return False, "needs --real on macOS; " + why
    url = comfy_url()
    if answers(url):  # the row needs a cold start: stop the ComfyUI that horus started
        run([horus_exe(a), "stop-comfyui"], timeout=300)
        if not wait_for(False, url, 60):
            return False, f"a ComfyUI that horus did not start answers at {url}; stop it first"
    ok, detail, env = real_image(a)
    if ok and (env.get("result") or {}).get("started_backend") is not True:
        return False, f"started_backend is not true ({detail})"
    return ok, detail + ("; started_backend: true" if ok else "")


def real_tcc(a: argparse.Namespace) -> tuple[bool, str]:
    why = real_needs_mac()
    if why:
        return False, why
    if not in_tcc_folder(str(ROOT), str(Path.home())):
        return False, f"the repo {ROOT} is not inside a TCC folder; run from the clone in ~/Desktop"
    code, out, fails = doctor_run(a)
    warn = [ln for ln in out.splitlines() if ln.startswith("WARN") and str(ROOT) in ln]
    if code != 0 or fails or not warn:
        return False, f"horus doctor: exit {code}, {fails} FAIL lines, {len(warn)} WARN lines that name {ROOT}"
    return True, f"horus doctor: exit 0, 0 FAIL lines, a WARN line names {ROOT}"


def row3_tcc(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail = pinned_tests(PINNED3["tcc-folders"])
    return with_real(a, ok, detail, real_tcc)


def row3_oom(a: argparse.Namespace) -> tuple[bool, str]:
    return pinned_tests(PINNED3["out-of-memory"])


def windows_only_paths(cfg) -> int:
    """The string values of .mcp.json that only Windows understands (USERPROFILE, a backslash, .exe)."""
    if isinstance(cfg, dict):
        return sum(windows_only_paths(v) for v in cfg.values())
    if isinstance(cfg, list):
        return sum(windows_only_paths(v) for v in cfg)
    return int(isinstance(cfg, str) and ("USERPROFILE" in cfg or "\\" in cfg or ".exe" in cfg.lower()))


def mcp_json_problems(text: str) -> list[str]:
    """What stops .mcp.json from serving both systems (spec, "One .mcp.json")."""
    try:
        servers = json.loads(text)["mcpServers"]
    except (ValueError, KeyError, TypeError):
        return [".mcp.json is not JSON with mcpServers"]
    problems = []
    n = windows_only_paths(servers)
    if n:
        problems.append(f"{n} Windows-only paths")
    for name, args in (("comfy-mcp", ["comfy-mcp"]), ("horus", ["mcp"])):
        entry = servers.get(name) if isinstance(servers, dict) else None
        if not isinstance(entry, dict) or entry.get("command") != "horus" or entry.get("args") != args:
            problems.append(f"the entry {name} is not `horus {' '.join(args)}`")
    return problems


def real_one_config(a: argparse.Namespace) -> tuple[bool, str]:
    code, _, fails = doctor_run(a)
    if code != 0 or fails:
        return False, f"horus doctor on {this_os()}: exit {code}, {fails} FAIL lines"
    return True, f"horus doctor on {this_os()}: 0 FAIL lines"


def row3_one_config(a: argparse.Namespace) -> tuple[bool, str]:
    try:
        problems = mcp_json_problems((ROOT / ".mcp.json").read_text(encoding="utf-8"))
    except OSError:
        problems = ["missing: .mcp.json"]
    ok, detail = pinned_tests(PINNED3["one-config"])
    if problems:
        return False, ".mcp.json: " + ", ".join(problems) + "; " + detail
    if not ok:
        return False, ".mcp.json ok; " + detail
    if not a.real:
        return True, f".mcp.json ok; {detail}; real part not run (needs --real on Windows and on macOS)"
    ok_r, detail_r = real_one_config(a)
    return ok_r, f".mcp.json ok; {detail}; real: {detail_r}"


def setup_doc_problems(setup_text: str, help_files: dict[str, str]) -> list[str]:
    """The lines that the Red searches of #91 find, and a missing macOS section of docs/setup.md."""
    problems = [f"docs/setup.md:{i}" for i, ln in enumerate(setup_text.splitlines(), 1) if SETUP_DOC_RED.search(ln)]
    for name, text in sorted(help_files.items()):
        problems += [f"{name}:{i}" for i, ln in enumerate(text.splitlines(), 1) if HELP_RED in ln]
    if not MACOS_HEADING.search(setup_text):
        problems.append("docs/setup.md has no macOS section")
    return problems


def row3_setup_doc(a: argparse.Namespace) -> tuple[bool, str]:
    try:
        setup_text = (ROOT / "docs" / "setup.md").read_text(encoding="utf-8")
    except OSError:
        return False, "missing: docs/setup.md"
    help_files = {}
    for f in sorted((ROOT / "internal" / "cli").rglob("*")):
        if f.is_file():
            help_files[f.relative_to(ROOT).as_posix()] = f.read_text(encoding="utf-8", errors="replace")
    problems = setup_doc_problems(setup_text, help_files)
    lines = sum(1 for p in problems if ":" in p)
    if problems:
        return False, f"{lines} lines that the commands contradict; " + ", ".join(problems)
    return True, "0 lines that the commands contradict; docs/setup.md has a macOS section"


def real_same(a: argparse.Namespace) -> tuple[bool, str]:
    """The PC part: the Phase 2 rows backend and commands with --real. On macOS there is no PC part."""
    if this_os() != "win32":
        return True, "the PC part (Phase 2 rows backend and commands) runs with --real on Windows"
    parts, ok = [], True
    for name, check in (("backend", row2_backend), ("commands", row2_commands)):
        ok_r, detail_r = check(a)
        ok &= ok_r
        parts.append(f"{name}: {'PASS' if ok_r else 'FAIL'} {detail_r}")
    return ok, "; ".join(parts)


def row3_same(a: argparse.Namespace) -> tuple[bool, str]:
    parts, ok = [], True
    local = argparse.Namespace(**{**vars(a), "no_ci": True})
    ok_c, detail_c = row_contract(local)
    ok &= ok_c
    parts.append("contract: " + detail_c.replace("; CI part skipped (--no-ci)", ""))
    code, out = run([sys.executable, "tests/workflow_parity.py"], timeout=900)
    parity = code == 0 and re.search(r"parity: 17 of 17 cases equal", out) is not None
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
    lists = allow_files()
    ok_a, detail_a, _ = go_test_ran([f"./{ARCHTEST_DIR}"], ARCHTEST_TESTS)
    ok &= ok_a and not lists
    parts.append(f"archtest: {'ok' if ok_a else detail_a}, {len(lists)} allow list files")
    n, err = go_test_count()
    ok &= n >= BASELINE_TESTS3
    parts.append(err if n < 0 else f"{n} Go test functions (baseline {BASELINE_TESTS3})")
    changed = changed_contract_files(BASELINE_COMMIT3)
    if isinstance(changed, str):
        ok = False
        parts.append(changed)
    else:
        ok &= not changed
        parts.append(f"{len(changed)} changed contract files" + (": " + ", ".join(changed[:3]) if changed else ""))
    ok_ci, ci = ci_part3(a, SAME_BEHAVIOUR_JOBS)
    ok &= ok_ci
    parts.append(ci)
    detail = "; ".join(parts)
    if not ok:
        return ok, detail
    if not a.real:
        return ok, detail + "; real part not run (needs --real on Windows)"
    ok_r, detail_r = real_same(a)
    return ok_r, detail + "; real: " + detail_r


CHECKS3 = {"mac-backend": row3_mac_backend, "mac-setup": row3_mac_setup, "mac-image": row3_mac_image,
           "tcc-folders": row3_tcc, "out-of-memory": row3_oom, "one-config": row3_one_config,
           "one-setup-doc": row3_setup_doc, "same-behaviour": row3_same}


CHECKS = {"contract": row_contract, "graph": row_graph, "workflow-cli": row_workflow_cli,
          "downloader": row_downloader, "setup-commands": row_setup, "builds": row_builds,
          "real-image": row_real, "no-python": row_no_python}


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--server-cmd", nargs="+", help="server command of the contract row (default: horus mcp)")
    ap.add_argument("--no-ci", action="store_true", help="leave out the CI part of the rows contract, same-behaviour and process-tree")
    ap.add_argument("--real", action="store_true", help="run the real-image row (GPU machine only)")
    ap.add_argument("--real-out", help="folder for the PNG of the real-image row (default: .scratch/real-image)")
    ap.add_argument("--phase", type=int, choices=[1, 2, 3], default=1,
                    help="goal rows to check: 1 = the Go port (default), 2 = the three layers, 3 = macOS")
    ap.add_argument("--only", nargs="+", metavar="ROW",
                    help="check only these rows, for example --only backend (the summary counts these rows)")
    a = ap.parse_args()
    rows, checks = {1: (ROWS, CHECKS), 2: (ROWS2, CHECKS2), 3: (ROWS3, CHECKS3)}[a.phase]
    if a.only:
        unknown = [r for r in a.only if r not in rows]
        if unknown:
            ap.error("unknown row(s) for this phase: " + ", ".join(unknown))
        rows = [r for r in rows if r in a.only]
    passed, not_real = 0, []
    for row in rows:
        try:
            ok, detail = checks[row](a)
        except Exception as exc:  # a row never raises
            ok, detail = False, f"check failed to run: {type(exc).__name__}: {exc}"
        passed += ok
        if "real part not run" in detail:
            not_real.append(row)
        print(f"{'PASS' if ok else 'FAIL'}  {row} - {detail}", flush=True)
    print(f"goal: {passed} of {len(rows)} rows passed")
    if not_real:
        print("note: the real part did not run for " + ", ".join(not_real) + " (run with --real on the Mac and on the PC)")
    return 0 if passed == len(rows) else 1


if __name__ == "__main__":
    sys.exit(main())

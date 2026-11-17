#!/usr/bin/env python3
"""The runner of the goal check. It runs the rows of one suite and prints one line for each row.

    python tools/goal_check.py --suite <name> [--no-ci] [--real] [--real-out <folder>] [--server-cmd <words...>] [--only <row...>]
    python tools/goal_check.py --phase 1|2|3|4 ...    the same as --suite phase<N> (no option: phase1)
    python tools/goal_check.py --list                 the suites, with the spec of each

A suite is a module tools/suites/<name>.py. It holds the goal rows of one spec and the check of each row:

    TITLE       one line: what the suite checks
    SPEC        the spec file that holds the goal rows
    ROWS        the row names, in the order of the spec
    CHECKS      {row: check}. A check takes the options (argparse.Namespace) and returns (ok, detail).
    REAL_WHERE  where --real must run, for the note line (optional)

This runner gives the parts that many rows use: run a command, run Go tests, run pinned Go tests,
judge the CI jobs of HEAD, join the parts of a row, join a real part. tools/suites/common.py gives the
checks of this repo that more than one suite uses (the contract test, the parity test, a real image).
A new feature gets a new suite file. It does not change the runner.

Output: one line for each row, "PASS  <row> - <detail>" or "FAIL  <row> - <detail>", then
"goal: N of M rows passed". The exit code is 0 only when N is M. A row never raises: an exception
becomes a FAIL line. A row that the suite lists but has no check fails. Standard library only.

Rules for a suite, so that the unit tests can replace a part: use a part of another module through
its module (r.run, common.real_image), never with "from ... import".
"""

from __future__ import annotations

import argparse
import importlib
import json
import os
import re
import shutil
import subprocess
import sys
import time
from pathlib import Path

TOOLS = Path(__file__).resolve().parent
ROOT = TOOLS.parent
SUITES_DIR = TOOLS / "suites"
NOT_SUITES = {"__init__", "common"}  # modules of tools/suites that are no suite
GO_DIRS = [r"C:\Program Files\Go\bin", "/usr/local/go/bin"]
ASSERTS = re.compile(r"\bt\.(Errorf?|Fatalf?|Fail|FailNow)\(|[(,]\s*t\s*[,)]")  # an assertion, or t given to a helper

if str(TOOLS) not in sys.path:
    sys.path.insert(0, str(TOOLS))


# ---------------------------------------------------------------------------------------------
# Commands and settings.
# ---------------------------------------------------------------------------------------------

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


def last_line(text: str) -> str:
    lines = [ln.strip() for ln in text.splitlines() if ln.strip()]
    return lines[-1][:200] if lines else "no output"


def this_os() -> str:
    """sys.platform: "darwin" on macOS, "win32" on Windows. The tests replace it."""
    return sys.platform


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


def file_problems(files: list[str]) -> list[str]:
    """The files of the list that do not exist in the repo root."""
    return [f for f in files if not (ROOT / f).is_file()]


def sha256_of(path: Path) -> str:
    import hashlib
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


# ---------------------------------------------------------------------------------------------
# Parts of a row.
# ---------------------------------------------------------------------------------------------

def join_parts(parts: list[tuple[bool, str]]) -> tuple[bool, str]:
    """One row from several parts: it passes when every part passes, and the detail holds every part."""
    return all(ok for ok, _ in parts), "; ".join(d for _, d in parts)


def join_real(a: argparse.Namespace, ok: bool, detail: str, real, needs: str) -> tuple[bool, str]:
    """Join the part without --real and the real part. Without --real the row stands on its part
    without --real, as a row stands on its local part with --no-ci, and the detail says so.
    needs says where --real must run, for example "on macOS"."""
    if not ok:
        return False, detail
    if not a.real:
        return True, detail + f"; real part not run (needs --real {needs})"
    ok_r, detail_r = real(a)
    return ok_r, detail + "; real: " + detail_r


# ---------------------------------------------------------------------------------------------
# Go tests.
# ---------------------------------------------------------------------------------------------

def go_exe() -> str | None:
    found = shutil.which("go")
    if found:
        return found
    for d in GO_DIRS:
        for name in ("go.exe", "go"):
            if (Path(d) / name).is_file():
                return str(Path(d) / name)
    return None


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
    nopass = [ln for ln in out.splitlines() if "[no test files]" in ln]
    if nopass:
        return False, "no test files: " + "; ".join(ln.split()[1] for ln in nopass if len(ln.split()) > 1), out
    if not re.search(r"^--- PASS", out, re.M):
        return False, "no test passed", out
    return True, "tests pass", out


def go_test_count() -> tuple[int, str]:
    """The number of Go test functions of the repo (go test -list . ./...), or (-1, why not)."""
    go = go_exe()
    if not go:
        return -1, "go not found"
    code, out = run([go, "test", "-list", ".", "./..."])
    if code != 0:
        return -1, f"go test -list failed: {last_line(out)}"
    return len(re.findall(r"^Test\w+$", out, re.M)), ""


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


def pinned_tests(pins: list[tuple[str, str, str]], texts: dict[str, str] | None = None) -> tuple[bool, str]:
    """Each pinned test (package folder, test name, the task that writes it) must exist, assert
    something, hold its text of texts (when texts names it), and report "--- PASS" in go test -v -run
    of its package. A missing package, a package with no test files, a missing name, a skipped test
    and an empty body fail, and the detail names the test and its task."""
    texts = texts or {}
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
            elif texts.get(name) and texts[name] not in body:
                bad.append(f"{name} (has no text '{texts[name]}', {task})")
            elif not re.search(rf"^--- PASS: {name} ", out, re.M):
                bad.append(f"{name} (did not pass in {pkg}: {last_line(out)[:80]}, {task})")
    if bad:
        return False, "no passing test: " + "; ".join(bad)
    return True, f"{len(pins)} pinned tests pass"


# ---------------------------------------------------------------------------------------------
# CI: the jobs of the newest run of ci.yml on a commit.
# ---------------------------------------------------------------------------------------------

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


_CI_CACHE: dict[str, tuple[list[dict] | None, str]] = {}


def head_jobs() -> tuple[str, list[dict] | None, str]:
    """(HEAD, its CI jobs or None, why not). One gh lookup for each commit and each run of the check."""
    _, sha = run(["git", "rev-parse", "HEAD"], timeout=30)
    sha = sha.strip()
    if sha not in _CI_CACHE:
        _CI_CACHE[sha] = ci_jobs(sha)
    jobs, why = _CI_CACHE[sha]
    return sha, jobs, why


def ci_part(a: argparse.Namespace, required: list[str]) -> tuple[bool, str]:
    """The CI part of a row (strict): every required job of HEAD is green and no other job is red.
    With --no-ci the part passes and says that it was skipped."""
    if a.no_ci:
        return True, "CI part skipped (--no-ci)"
    _, jobs, why = head_jobs()
    if jobs is None:
        return False, why
    return judge_jobs_strict(jobs, required)


def ci_part_lenient(required: list[str]) -> tuple[bool, str]:
    """The CI part of the older rows (Phases 1 and 2): every required job of HEAD is green.
    Another job that is red does not fail the row. The caller skips it with --no-ci."""
    _, jobs, why = head_jobs()
    if jobs is None:
        return False, why
    return judge_jobs(jobs, required)


# ---------------------------------------------------------------------------------------------
# Suites and the command line.
# ---------------------------------------------------------------------------------------------

def suite_names() -> list[str]:
    """The names of the suites: each module of tools/suites, except the modules that are no suite."""
    return sorted(p.stem for p in SUITES_DIR.glob("*.py") if p.stem not in NOT_SUITES)


def load_suite(name: str):
    """The module of a suite. Raises ValueError when the module is not a suite."""
    if name not in suite_names():
        raise ValueError(f"unknown suite: {name} (known: {', '.join(suite_names())})")
    mod = importlib.import_module(f"suites.{name}")
    missing = [k for k in ("TITLE", "SPEC", "ROWS", "CHECKS") if not hasattr(mod, k)]
    if missing:
        raise ValueError(f"tools/suites/{name}.py is not a suite: it has no " + ", ".join(missing))
    return mod


def run_rows(suite, rows: list[str], a: argparse.Namespace) -> tuple[int, list[str]]:
    """Run the check of each row and print its line. Returns (rows passed, rows whose real part did not run)."""
    passed, not_real = 0, []
    for row in rows:
        check = suite.CHECKS.get(row)
        try:
            ok, detail = check(a) if check else (False, "the suite has no check for this row")
        except Exception as exc:  # a row never raises
            ok, detail = False, f"check failed to run: {type(exc).__name__}: {exc}"
        passed += ok
        if "real part not run" in detail:
            not_real.append(row)
        print(f"{'PASS' if ok else 'FAIL'}  {row} - {detail}", flush=True)
    return passed, not_real


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--suite", help="the suite to check: a module of tools/suites (see --list)")
    ap.add_argument("--phase", type=int, choices=[1, 2, 3, 4],
                    help="the same as --suite phase<N>: 1 = the Go port, 2 = the three layers, 3 = macOS, 4 = release and install")
    ap.add_argument("--list", action="store_true", help="print the suites and the spec of each, then stop")
    ap.add_argument("--server-cmd", nargs="+", help="server command of the contract rows (default: horus mcp)")
    ap.add_argument("--no-ci", action="store_true", help="leave out the CI part of each row")
    ap.add_argument("--real", action="store_true", help="also run the real part of each row (the GPU machine, the Mac or the release)")
    ap.add_argument("--real-out", help="folder for the files of the real parts (default: .scratch/<part>)")
    ap.add_argument("--only", nargs="+", metavar="ROW",
                    help="check only these rows, for example --only backend (the summary counts these rows)")
    a = ap.parse_args(argv)
    if a.list:
        for name in suite_names():
            try:
                s = load_suite(name)
                print(f"{name}: {s.TITLE} ({s.SPEC}, {len(s.ROWS)} rows)")
            except Exception as exc:  # a broken suite does not hide the others
                print(f"{name}: cannot load: {type(exc).__name__}: {exc}")
        return 0
    if a.suite and a.phase and a.suite != f"phase{a.phase}":
        ap.error(f"--suite {a.suite} and --phase {a.phase} name different suites")
    a.suite = a.suite or f"phase{a.phase or 1}"
    a.phase = a.phase or (int(a.suite[5:]) if re.fullmatch(r"phase\d+", a.suite) else None)
    try:
        suite = load_suite(a.suite)
    except ValueError as exc:
        ap.error(str(exc))
    rows = list(suite.ROWS)
    if a.only:
        unknown = [r for r in a.only if r not in rows]
        if unknown:
            ap.error("unknown row(s) for this suite: " + ", ".join(unknown))
        rows = [r for r in rows if r in a.only]
    passed, not_real = run_rows(suite, rows, a)
    print(f"goal: {passed} of {len(rows)} rows passed")
    if not_real:
        where = getattr(suite, "REAL_WHERE", "")
        print("note: the real part did not run for " + ", ".join(not_real) + f" (run with --real{' ' + where if where else ''})")
    return 0 if passed == len(rows) else 1


if __name__ == "__main__":
    sys.exit(main())

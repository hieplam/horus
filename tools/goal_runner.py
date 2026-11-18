#!/usr/bin/env python3
"""The runner of the goal check. It reads one suite file, runs its rows and prints one line for each row.

    python tools/goal_check.py --list                     the suites, with the spec of each
    python tools/goal_check.py --suite <name> [options]   the rows of tools/suites/<name>.toml
    python tools/goal_check.py --phase <N> [options]      the same as --suite phase<N> (no option: phase1)

A suite is a TOML file in tools/suites/. tools/suites/README.md gives its format and the check types.
In short, each row has a name and a list of parts. Each part names a check type and its parameters:
a short name of tools/checks/parts.py ("go_pinned_tests", "ci_green", "command", ...) or
"<module>.<function>" of a module of tools/checks. A row can also have a real part, which runs only
with --real. A new feature gets a new suite file. It changes the runner only for a new kind of row.

A row passes when every part passes. Its detail joins the details of its parts with "; ". With
stop_at_fail, the row stops at the first part that fails. Without --real, a row with a real part stands
on its other parts, and its detail says "real part not run".

Output: one line for each row, "PASS  <row> - <detail>" or "FAIL  <row> - <detail>", then
"goal: N of M rows passed". The exit code is 0 only when N is M. A row never raises: an exception
becomes a FAIL line. A suite file with an unknown check type or a wrong parameter does not run
(exit code 2), and the message names the row and the part. Standard library only (Python 3.11+).
"""

from __future__ import annotations

import argparse
import importlib
import inspect
import re
import sys
import tomllib
from pathlib import Path

RUNNER_DIR = Path(__file__).resolve().parent
SUITES_DIR = RUNNER_DIR / "suites"

if str(RUNNER_DIR) not in sys.path:
    sys.path.insert(0, str(RUNNER_DIR))

from checks import base  # noqa: E402


class SuiteError(ValueError):
    """A suite file that the runner cannot run. The message names the file, the row and the part."""


# ---------------------------------------------------------------------------------------------
# Check types.
# ---------------------------------------------------------------------------------------------

def resolve(name: str):
    """The function of a check type: a short name of checks/parts.py, or "<module>.<function>" of
    tools/checks/<module>.py. The lookup runs at each call, so a test that replaces a function sees it."""
    module, _, func = name.rpartition(".")
    mod = importlib.import_module(f"checks.{module or 'parts'}")
    fn = getattr(mod, func, None)
    if not callable(fn) or func.startswith("_"):
        raise SuiteError(f"unknown check type: {name}")
    return fn


def takes_options(fn) -> bool:
    """True when the first parameter of fn is the options of the command line (a check of a row).
    A part with no parameter, for example release_files_part(), gets no options."""
    params = list(inspect.signature(fn).parameters.values())
    return bool(params) and params[0].kind in (inspect.Parameter.POSITIONAL_ONLY, inspect.Parameter.POSITIONAL_OR_KEYWORD) \
        and params[0].default is inspect.Parameter.empty


def call(part: dict, a: argparse.Namespace) -> tuple[bool, str]:
    """Run one part: its check type with the options and its parameters."""
    fn = resolve(part["check"])
    params = {k: v for k, v in part.items() if k not in ("check", "needs")}
    ok, detail = fn(a, **params) if takes_options(fn) else fn(**params)
    return bool(ok), detail


def check_part(part, where: str) -> None:
    """Raise SuiteError when a part has no check type, an unknown one, or parameters that its function does not take."""
    if not isinstance(part, dict) or not isinstance(part.get("check"), str):
        raise SuiteError(f"{where}: a part needs check = \"<type>\"")
    try:
        fn = resolve(part["check"])
    except (ImportError, SuiteError):
        raise SuiteError(f"{where}: unknown check type: {part['check']}") from None
    params = {k: v for k, v in part.items() if k not in ("check", "needs")}
    try:
        if takes_options(fn):
            inspect.signature(fn).bind(None, **params)
        else:
            inspect.signature(fn).bind(**params)
    except TypeError as exc:
        raise SuiteError(f"{where}: check {part['check']}: {exc}") from None


# ---------------------------------------------------------------------------------------------
# Suites.
# ---------------------------------------------------------------------------------------------

def suite_names() -> list[str]:
    """The names of the suites: the TOML files of tools/suites."""
    return sorted(p.stem for p in SUITES_DIR.glob("*.toml"))


def load_suite(name: str) -> dict:
    """The suite tools/suites/<name>.toml, checked. Raises SuiteError when it cannot run."""
    if name not in suite_names():
        raise SuiteError(f"unknown suite: {name} (known: {', '.join(suite_names())})")
    path = SUITES_DIR / f"{name}.toml"
    try:
        suite = tomllib.loads(path.read_text(encoding="utf-8"))
    except tomllib.TOMLDecodeError as exc:
        raise SuiteError(f"tools/suites/{name}.toml is not TOML: {exc}") from None
    missing = [k for k in ("title", "spec", "rows") if k not in suite]
    if missing:
        raise SuiteError(f"tools/suites/{name}.toml has no " + ", ".join(missing))
    seen = set()
    for i, row in enumerate(suite["rows"], 1):
        where = f"tools/suites/{name}.toml, row {i}"
        if not isinstance(row.get("name"), str) or not row["name"]:
            raise SuiteError(f"{where}: a row needs name = \"<row>\"")
        where = f"tools/suites/{name}.toml, row {row['name']}"
        if row["name"] in seen:
            raise SuiteError(f"{where}: the name is used two times")
        seen.add(row["name"])
        if not row.get("parts"):
            raise SuiteError(f"{where}: a row needs at least one [[rows.parts]]")
        for j, part in enumerate(row["parts"], 1):
            check_part(part, f"{where}, part {j}")
        if "real" in row:
            check_part(row["real"], f"{where}, real part")
            if not (row["real"].get("needs") or suite.get("real_needs")):
                raise SuiteError(f"{where}, real part: needs = \"<where --real runs>\" (or real_needs of the suite)")
    suite["name"] = name
    return suite


def row_of(suite: dict, name: str) -> dict:
    for row in suite["rows"]:
        if row["name"] == name:
            return row
    raise KeyError(f"suite {suite['name']} has no row {name}")


def check_row(suite: dict, row: dict, a: argparse.Namespace) -> tuple[bool, str]:
    """Run the parts of a row in order and join them, then the real part. Never raises."""
    try:
        results = []
        for part in row["parts"]:
            results.append(call(part, a))
            if row.get("stop_at_fail") and not results[-1][0]:
                break
        ok, detail = base.join_parts(results)
        real = row.get("real")
        if real:
            needs = real.get("needs") or suite.get("real_needs", "")
            ok, detail = base.join_real(a, ok, detail, lambda opts: call(real, opts), needs)
        return ok, detail
    except Exception as exc:  # a row never raises
        return False, f"check failed to run: {type(exc).__name__}: {exc}"


def run_rows(suite: dict, rows: list[str], a: argparse.Namespace) -> tuple[int, list[str]]:
    """Run each row and print its line. Returns (rows passed, rows whose real part did not run)."""
    passed, not_real = 0, []
    for name in rows:
        ok, detail = check_row(suite, row_of(suite, name), a)
        passed += ok
        if "real part not run" in detail:
            not_real.append(name)
        print(f"{'PASS' if ok else 'FAIL'}  {name} - {detail}", flush=True)
    return passed, not_real


# ---------------------------------------------------------------------------------------------
# The command line.
# ---------------------------------------------------------------------------------------------

def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--suite", help="the suite to check: a file tools/suites/<name>.toml (see --list)")
    ap.add_argument("--phase", type=int, help="the same as --suite phase<N>")
    ap.add_argument("--list", action="store_true", help="print the suites and the spec of each, then stop")
    ap.add_argument("--server-cmd", nargs="+", help="server command of the contract checks (default: horus mcp)")
    ap.add_argument("--no-ci", action="store_true", help="leave out the CI part of each row")
    ap.add_argument("--real", action="store_true", help="also run the real part of each row (the GPU machine, the Mac or the release)")
    ap.add_argument("--real-out", help="folder for the files of the real parts (default: .scratch/<part>)")
    ap.add_argument("--only", nargs="+", metavar="ROW",
                    help="check only these rows, for example --only backend (the summary counts these rows)")
    a = ap.parse_args(argv)
    if a.list:
        code = 0
        for name in suite_names():
            try:
                s = load_suite(name)
                print(f"{name}: {s['title']} ({s['spec']}, {len(s['rows'])} rows)")
            except SuiteError as exc:  # a broken suite does not hide the others
                print(f"{name}: cannot run: {exc}")
                code = 2
        return code
    if a.suite and a.phase is not None and a.suite != f"phase{a.phase}":
        ap.error(f"--suite {a.suite} and --phase {a.phase} name different suites")
    a.suite = a.suite or f"phase{1 if a.phase is None else a.phase}"
    if a.phase is None and re.fullmatch(r"phase\d+", a.suite):
        a.phase = int(a.suite[5:])
    try:
        suite = load_suite(a.suite)
    except SuiteError as exc:
        ap.error(str(exc))
    rows = [r["name"] for r in suite["rows"]]
    if a.only:
        unknown = [r for r in a.only if r not in rows]
        if unknown:
            ap.error("unknown row(s) for this suite: " + ", ".join(unknown))
        rows = [r for r in rows if r in a.only]
    passed, not_real = run_rows(suite, rows, a)
    print(f"goal: {passed} of {len(rows)} rows passed")
    if not_real:
        where = suite.get("real_where", "")
        print("note: the real part did not run for " + ", ".join(not_real) + f" (run with --real{' ' + where if where else ''})")
    return 0 if passed == len(rows) else 1


if __name__ == "__main__":
    sys.exit(main())

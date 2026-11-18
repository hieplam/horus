"""The general check types that a suite file names with a short name, for example check = "go_pinned_tests".

Each function takes the options of the command line (a) and the parameters of its part in the
suite file, and returns (ok, detail). tools/suites/README.md lists them with an example.
A check type that only one spec needs lives in its own module (tools/checks/<module>.py), and the
suite file names it as "<module>.<function>".
"""

from __future__ import annotations

import argparse
import re
import sys

from checks import base, horus


def go_pinned_tests(a: argparse.Namespace, pins: list[list[str]], texts: dict[str, str] | None = None) -> tuple[bool, str]:
    """Each pin [package folder, test name, task] is a Go test that exists, asserts something, holds its
    text of texts (when texts names it), and passes in go test -v -run of its package."""
    return base.pinned_tests([tuple(p) for p in pins], texts)


def go_tests(a: argparse.Namespace, packages: list[str], names: list[str] | None = None) -> tuple[bool, str]:
    """go test -v of the packages passes and tests ran. With names, each named test reports --- PASS."""
    ok, detail, _ = base.go_test_ran(packages, names)
    return ok, detail


def ci_green(a: argparse.Namespace, jobs: list[str]) -> tuple[bool, str]:
    """Each named job of the newest ci.yml run on HEAD is green, and no other job is red. --no-ci skips it."""
    return base.ci_part(a, jobs)


def files(a: argparse.Namespace, paths: list[str]) -> tuple[bool, str]:
    """Each file exists in the repo root."""
    missing = base.file_problems(paths)
    if missing:
        return False, "missing: " + ", ".join(missing)
    return True, f"{len(paths)} files present"


def command(a: argparse.Namespace, cmd: list[str], exit: int = 0, expect: str | None = None,
            timeout: int = 900) -> tuple[bool, str]:
    """Run a command in the repo root. It passes when its exit code is exit and, with expect, when its
    output matches the regular expression expect. In cmd, {python} is this Python and {horus} is the
    horus program of --server-cmd (default: horus)."""
    if isinstance(cmd, str) or not cmd:
        raise TypeError("cmd must be a list of words, for example [\"{horus}\", \"version\"]")
    words =[w.replace("{python}", sys.executable).replace("{horus}", horus.horus_exe(a)) for w in cmd]
    code, out = base.run(words, timeout=timeout)
    ok = code == exit and (expect is None or re.search(expect, out, re.M) is not None)
    detail = f"{' '.join(cmd)}: exit {code}"
    if expect is not None and re.search(expect, out, re.M) is None:
        detail += f", no line matches '{expect}'"
    return ok, detail + ("" if ok else f" ({base.last_line(out)})")


def contract(a: argparse.Namespace, ci: list[str] | bool | None = None, label: str = "") -> tuple[bool, str]:
    """All checks of tests/mcp_contract.py pass. ci: the CI jobs to need (default: the contract jobs),
    or false for the local part only. label starts the detail."""
    return horus.contract_check(a, ci, label)


def parity(a: argparse.Namespace) -> tuple[bool, str]:
    """tests/workflow_parity.py passes with all cases equal."""
    return horus.parity_part()


def go_test_all(a: argparse.Namespace) -> tuple[bool, str]:
    """go test ./... passes."""
    return horus.go_test_all_part()


def archtest(a: argparse.Namespace) -> tuple[bool, str]:
    """The three tests of internal/archtest pass, and no allow list file exists."""
    return horus.archtest_part()


def test_count(a: argparse.Namespace, baseline: int) -> tuple[bool, str]:
    """The number of Go test functions is not below baseline."""
    return horus.test_count_part(baseline)


def no_changes(a: argparse.Namespace, commit: str, paths: list[str], what: str) -> tuple[bool, str]:
    """No file of paths differs from the commit (a rename with the same content is no change).
    what names the files in the detail."""
    return horus.changed_part(horus.changed_files(commit, paths), what)


def mcp_image(a: argparse.Namespace) -> tuple[bool, str]:
    """One real image through the horus MCP server (only with --real, on the GPU machine)."""
    ok, detail, _ = horus.real_image(a)
    return ok, detail

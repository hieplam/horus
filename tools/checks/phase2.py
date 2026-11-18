"""The checks that only the suite phase2 needs (tools/suites/phase2.toml): the three layers,
docs/specs/2026-11-11-restructure.md. The rows backend and commands pass only with --real, because
their oracle needs the owner's PC (the real image, horus doctor, horus setup). With --real, the row
commands runs horus doctor and horus setup it-illustration (no -yes) of the --server-cmd program. It
fails on a FAIL line of doctor, on a step in the plan, or when a file that the plan names changed."""

from __future__ import annotations

import argparse
import re
from pathlib import Path

from checks import base as r
from checks import horus

PROCESS_TREE_JOBS = horus.GO_JOBS
TREE_TEST = "TestTreeOutlivesItsStarterAndEndsFromAnotherProcess"  # check 1 and check 2 of the spec
PACKAGE_LIST = r.TOOLS / "phase2_packages.txt"
SETUP_LINE = re.compile(r"^(PRESENT|MISSING)\s+(.*)$")  # a fact of the plan of horus setup


def go_files() -> list[str]:
    _, out = r.run(["git", "ls-files", "*.go"], timeout=60)
    return [f for f in out.splitlines() if f.endswith(".go")]


def count_taskkill() -> int:
    """Lines of Go files that name taskkill (the Windows program that ends a process tree)."""
    n = 0
    for f in go_files():
        try:
            n += len(re.findall(r"taskkill", (r.ROOT / f).read_text(encoding="utf-8", errors="replace"), re.I))
        except OSError:
            pass
    return n


def packages_off_target() -> tuple[list[str], list[str]] | str:
    """(packages that exist but are not on the list, packages on the list that do not exist), or an error text."""
    go = r.go_exe()
    if not go:
        return "go not found"
    if not PACKAGE_LIST.is_file():
        return f"missing: {PACKAGE_LIST.relative_to(r.ROOT).as_posix()}"
    code, out = r.run([go, "list", "./..."])
    if code != 0:
        return f"go list failed: {r.last_line(out)}"
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


def row2_archtest(a: argparse.Namespace) -> tuple[bool, str]:
    """The rows os-boundary, direction and token: the three tests of internal/archtest report
    "--- PASS", and no allow list file internal/archtest/allow_<rule>.txt exists."""
    if not (r.ROOT / horus.ARCHTEST_DIR).is_dir():
        return False, f"missing: {horus.ARCHTEST_DIR} (task T1)"
    lists = horus.allow_files()
    ok, detail, _ = r.go_test_ran([f"./{horus.ARCHTEST_DIR}"], horus.ARCHTEST_TESTS)
    text = f"{len(lists)} allow list files" + (" (" + ", ".join(lists) + ")" if lists else "")
    return ok and not lists, f"{text}; archtest: {'ok' if ok else detail}"


def row2_process_tree(a: argparse.Namespace) -> tuple[bool, str]:
    kills = count_taskkill()
    if not (r.ROOT / "internal" / "platform").is_dir():
        return False, f"missing: internal/platform (task T2); {kills} taskkill references"
    ok, detail, out = r.go_test(["./internal/platform"], ["-count=1", "-v", "-run", "Tree"])
    # The test with check 1 and check 2 must pass. Another test with Tree in its name is not enough.
    if ok and not re.search(rf"^--- PASS: {TREE_TEST} ", out, re.M):
        return False, f"{TREE_TEST} did not pass in internal/platform; {kills} taskkill references"
    detail = f"tree test: {'ok' if ok else detail}; {kills} taskkill references"
    if ok and kills == 0 and not a.no_ci:
        ci_ok, ci = r.ci_part_lenient(PROCESS_TREE_JOBS)
        return ci_ok, f"{detail}; {ci}"
    if ok and kills == 0:
        detail += "; CI part skipped (--no-ci)"
    return ok and kills == 0, detail


def row2_backend(a: argparse.Namespace) -> tuple[bool, str]:
    if not (r.ROOT / "internal" / "backend").is_dir():
        return False, "missing: internal/backend (task T7)"
    ok, detail, _ = r.go_test_ran(["./internal/backend"])
    if not ok:
        return False, detail
    if not a.real:
        return False, "Go tests ok; needs --real on the owner's PC for the image through the backend"
    ok_r, detail_r, env = horus.real_image(a)
    if ok_r and (env.get("result") or {}).get("started_backend") is not True:
        # The row proves a start through the backend: a ComfyUI that already answered proves nothing.
        return False, f"started_backend is not true: ComfyUI already answered; stop it first ({detail_r})"
    return ok_r, detail_r + ("; started through the backend" if ok_r else "")


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
    missing = [p for p in ("internal/cli", "internal/doctor") if not (r.ROOT / p).is_dir()]
    if missing:
        return False, "missing: " + ", ".join(missing) + " (tasks T8 and T9)"
    ok, detail, _ = r.go_test_ran(["./internal/cli", "./internal/doctor", "./internal/provision"])
    if not ok:
        return False, detail
    if not a.real:
        return False, "Go tests ok; needs --real on the owner's PC for horus doctor and horus setup"
    exe = (a.server_cmd or ["horus"])[0]
    # The plan first: it names the files that doctor and setup must not change.
    code, first = r.run([exe, "setup", "it-illustration"], timeout=300)
    paths = setup_paths(first)
    if code != 0 or not paths:
        return False, f"horus setup it-illustration: exit {code}, {len(paths)} paths in the plan: {r.last_line(first)}"
    before = file_states(paths)
    code, out = r.run([exe, "doctor"], timeout=300)
    fails = len(re.findall(r"^FAIL\b", out, re.M))
    if code != 0 or fails:
        return False, f"horus doctor: exit {code}, {fails} FAIL lines"
    code, second = r.run([exe, "setup", "it-illustration"], timeout=300)
    m = horus.STEPS_LINE.search(second)
    steps = int(m.group(1)) if m else -1
    if code != 0 or steps != 0 or not re.search(r"nothing (is )?missing", second, re.I):
        return False, f"horus setup it-illustration: exit {code}, {steps} steps in the plan"
    changed = [b[0] for b, c in zip(before, file_states(paths)) if b != c]
    if changed or second != first:
        return False, "a file changed: " + (", ".join(changed) or "the plan text differs between two runs")
    present = sum(1 for _, size, _ in before if size is not None)
    return True, (f"horus doctor: 0 FAIL lines; horus setup it-illustration: 0 steps, nothing is missing; "
                  f"{present} of {len(paths)} named files present and unchanged")



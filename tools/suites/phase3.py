"""Phase 3: horus on macOS through comfy-cli (docs/specs/2026-11-12-macos-backend.md).

The 8 goal rows: mac-backend, mac-setup, mac-image, tcc-folders, out-of-memory, one-config,
one-setup-doc, same-behaviour. The rows that need Go code run the Go tests that PINNED3 pins by
name. Each pinned test must report "--- PASS" in go test -v -run of its package, and its body must
assert something. A missing package, a package with no test files, a missing name, a skipped test
and an empty body fail the row. Unless --no-ci, mac-backend needs the job "go darwin/arm64" and
same-behaviour the 6 jobs of Phase 2, and no other job of the run may be red. Without --real, a row
stands on its part without --real (mac-image fails: it has no such part), its detail says "real part
not run", and a note line follows the summary. With --real on macOS, mac-backend, mac-setup,
mac-image, tcc-folders and one-config run horus and comfy on the Mac. On Windows, a row of the Mac
fails and says so, one-config runs horus doctor, and same-behaviour runs the Phase 2 rows backend and
commands with --real.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
from pathlib import Path

import goal_runner as r
from suites import common, phase2

TITLE = "horus on macOS through comfy-cli"
SPEC = "docs/specs/2026-11-12-macos-backend.md"
REAL_WHERE = "on the Mac and on the PC"
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
SETUP_DOC_RED = re.compile(r"Two commands are|does steps 1 and 4")  # the first Red search of #91 (docs/setup.md)
HELP_RED = "then ~/ComfyUI_windows_portable"  # the second Red search of #91 (internal/cli)
MACOS_HEADING = re.compile(r"^#{1,6} .*\bmacOS\b", re.M)


def pinned(row: str) -> tuple[bool, str]:
    """The pinned tests of a row of Phase 3, with the texts of PIN_TEXT."""
    return r.pinned_tests(PINNED3[row], PIN_TEXT)


def with_real(a: argparse.Namespace, ok: bool, detail: str, real) -> tuple[bool, str]:
    return r.join_real(a, ok, detail, real, "on macOS")


def in_tcc_folder(path: str, home: str) -> bool:
    """True when path is inside <home>/Desktop, <home>/Documents or <home>/Downloads."""
    p, h = Path(path).resolve(), Path(home).resolve()
    return any(p == h / f or h / f in p.parents for f in TCC_FOLDERS)


def real_needs_mac() -> str | None:
    """None on macOS, else the detail of a real part that needs the Mac."""
    return None if r.this_os() == "darwin" else f"the real part needs macOS (this machine: {r.this_os()})"


def real_mac_backend(a: argparse.Namespace) -> tuple[bool, str]:
    """Start and stop through horus, then a ComfyUI that a person starts with comfy, which
    horus stop-comfyui must refuse. The hand-started ComfyUI is stopped with comfy stop."""
    why = real_needs_mac()
    if why:
        return False, why
    exe, url = common.horus_exe(a), common.comfy_url()
    if common.answers(url):
        r.run([exe, "stop-comfyui"], timeout=300)
        if not common.wait_for(False, url, 60):
            return False, f"a ComfyUI that horus did not start answers at {url}; stop it first"
    code, out = r.run([exe, "start-comfyui"], timeout=900)
    if code != 0 or not common.answers(url):
        return False, f"horus start-comfyui: exit {code}, the server does not answer: {r.last_line(out)}"
    code, out = r.run([exe, "stop-comfyui"], timeout=300)
    if code != 0 or not common.wait_for(False, url, 60):
        return False, f"horus stop-comfyui: exit {code}, the server still answers: {r.last_line(out)}"
    comfy = shutil.which("comfy") or str(Path.home() / ".venvs" / "comfy-mcp" / "bin" / "comfy")
    workspace = str(Path(r.setting("HORUS_COMFYUI_DIR", str(Path.home() / "horus"))) / "ComfyUI")
    port = url.rsplit(":", 1)[-1]
    r.run([comfy, "--workspace", workspace, "launch", "--background", "--", "--listen", "127.0.0.1", "--port", port],
          timeout=600)
    if not common.wait_for(True, url, 300):
        return False, f"a ComfyUI started by hand with {comfy} does not answer at {url}"
    code, out = r.run([exe, "stop-comfyui"], timeout=300)
    still = common.answers(url)
    r.run([comfy, "--workspace", workspace, "stop"], timeout=300)
    common.wait_for(False, url, 60)
    if code == 0 or not still:
        return False, f"horus stop-comfyui did not refuse a ComfyUI that horus did not start (exit {code})"
    return True, "start, stop, and a refusal of a ComfyUI started by hand"


def row3_mac_backend(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail = pinned("mac-backend")
    if ok:
        ok_ci, ci = r.ci_part(a, MAC_BACKEND_JOBS)
        ok, detail = ok_ci, f"{detail}; {ci}"
    return with_real(a, ok, detail, real_mac_backend)


def real_mac_setup(a: argparse.Namespace) -> tuple[bool, str]:
    why = real_needs_mac()
    if why:
        return False, why
    if os.environ.get("HORUS_COMFYUI_DIR") or r.setting("HORUS_COMFYUI_DIR", ""):
        return False, "HORUS_COMFYUI_DIR is set (environment or .env); the row needs the default folder"
    code, out, fails = common.doctor_run(a, "-workflow", "it-illustration")
    if code != 0 or fails:
        return False, f"horus doctor -workflow it-illustration: exit {code}, {fails} FAIL lines"
    code, out = r.run([common.horus_exe(a), "setup", "it-illustration"], timeout=300)
    m = common.STEPS_LINE.search(out)
    if code != 0 or not m or m.group(1) != "0":
        return False, f"horus setup it-illustration: exit {code}, {m.group(1) if m else 'no'} steps in the plan"
    return True, "horus doctor: 0 FAIL lines; Plan: 0 steps; no HORUS_COMFYUI_DIR"


def row3_mac_setup(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail = pinned("mac-setup")
    return with_real(a, ok, detail, real_mac_setup)


def row3_mac_image(a: argparse.Namespace) -> tuple[bool, str]:
    if not a.real:
        return False, "needs --real on macOS"
    why = real_needs_mac()
    if why:
        return False, "needs --real on macOS; " + why
    url = common.comfy_url()
    if common.answers(url):  # the row needs a cold start: stop the ComfyUI that horus started
        r.run([common.horus_exe(a), "stop-comfyui"], timeout=300)
        if not common.wait_for(False, url, 60):
            return False, f"a ComfyUI that horus did not start answers at {url}; stop it first"
    ok, detail, env = common.real_image(a)
    if ok and (env.get("result") or {}).get("started_backend") is not True:
        return False, f"started_backend is not true ({detail})"
    return ok, detail + ("; started_backend: true" if ok else "")


def real_tcc(a: argparse.Namespace) -> tuple[bool, str]:
    why = real_needs_mac()
    if why:
        return False, why
    if not in_tcc_folder(str(r.ROOT), str(Path.home())):
        return False, f"the repo {r.ROOT} is not inside a TCC folder; run from the clone in ~/Desktop"
    code, out, fails = common.doctor_run(a)
    warn = [ln for ln in out.splitlines() if ln.startswith("WARN") and str(r.ROOT) in ln]
    if code != 0 or fails or not warn:
        return False, f"horus doctor: exit {code}, {fails} FAIL lines, {len(warn)} WARN lines that name {r.ROOT}"
    return True, f"horus doctor: exit 0, 0 FAIL lines, a WARN line names {r.ROOT}"


def row3_tcc(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail = pinned("tcc-folders")
    return with_real(a, ok, detail, real_tcc)


def row3_oom(a: argparse.Namespace) -> tuple[bool, str]:
    return pinned("out-of-memory")


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
    code, _, fails = common.doctor_run(a)
    if code != 0 or fails:
        return False, f"horus doctor on {r.this_os()}: exit {code}, {fails} FAIL lines"
    return True, f"horus doctor on {r.this_os()}: 0 FAIL lines"


def row3_one_config(a: argparse.Namespace) -> tuple[bool, str]:
    try:
        problems = mcp_json_problems((r.ROOT / ".mcp.json").read_text(encoding="utf-8"))
    except OSError:
        problems = ["missing: .mcp.json"]
    ok, detail = pinned("one-config")
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
        setup_text = (r.ROOT / "docs" / "setup.md").read_text(encoding="utf-8")
    except OSError:
        return False, "missing: docs/setup.md"
    help_files = {}
    for f in sorted((r.ROOT / "internal" / "cli").rglob("*")):
        if f.is_file():
            help_files[f.relative_to(r.ROOT).as_posix()] = f.read_text(encoding="utf-8", errors="replace")
    problems = setup_doc_problems(setup_text, help_files)
    lines = sum(1 for p in problems if ":" in p)
    if problems:
        return False, f"{lines} lines that the commands contradict; " + ", ".join(problems)
    return True, "0 lines that the commands contradict; docs/setup.md has a macOS section"


def real_same(a: argparse.Namespace) -> tuple[bool, str]:
    """The PC part: the Phase 2 rows backend and commands with --real. On macOS there is no PC part."""
    if r.this_os() != "win32":
        return True, "the PC part (Phase 2 rows backend and commands) runs with --real on Windows"
    parts, ok = [], True
    for name, check in (("backend", phase2.row2_backend), ("commands", phase2.row2_commands)):
        ok_r, detail_r = check(a)
        ok &= ok_r
        parts.append(f"{name}: {'PASS' if ok_r else 'FAIL'} {detail_r}")
    return ok, "; ".join(parts)


def row3_same(a: argparse.Namespace) -> tuple[bool, str]:
    local = argparse.Namespace(**{**vars(a), "no_ci": True})
    ok_c, detail_c = common.row_contract(local)
    parts = [(ok_c, "contract: " + detail_c.replace("; CI part skipped (--no-ci)", "")),
             common.parity_part(), common.go_test_all_part(), common.archtest_part(),
             common.test_count_part(BASELINE_TESTS3),
             common.changed_part(common.changed_contract_files(BASELINE_COMMIT3), "contract files"),
             r.ci_part(a, common.SAME_BEHAVIOUR_JOBS)]
    ok, detail = r.join_parts(parts)
    if not ok:
        return ok, detail
    if not a.real:
        return ok, detail + "; real part not run (needs --real on Windows)"
    ok_r, detail_r = real_same(a)
    return ok_r, detail + "; real: " + detail_r


ROWS = ["mac-backend", "mac-setup", "mac-image", "tcc-folders", "out-of-memory", "one-config",
        "one-setup-doc", "same-behaviour"]
CHECKS = {"mac-backend": row3_mac_backend, "mac-setup": row3_mac_setup, "mac-image": row3_mac_image,
          "tcc-folders": row3_tcc, "out-of-memory": row3_oom, "one-config": row3_one_config,
          "one-setup-doc": row3_setup_doc, "same-behaviour": row3_same}

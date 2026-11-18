"""Phase 4: release, install line, full setup and uninstall (docs/specs/2026-11-15-release-install.md).

The 8 goal rows: release, install-line, binary-only, full-setup, uninstall, register, readme,
same-behaviour. The Go rows run the tests that PINNED4 pins by name, as Phase 3 does. A row whose
code does not exist yet fails and its detail names what is missing (a file, a test name, a CI job, a
code count). Unless --no-ci, release needs the CI job "release-build" and install-line the jobs
"install <os>". Without --real, a row stands on its part without --real and its detail says "real
part not run". With --real (on the PC, after the release), release downloads the 3 assets of the
newest release and checks each SHA-256 and "horus version". install-line, full-setup and uninstall
need the newest run of fresh-machine.yml with source=release (run-name "fresh-machine source=release
tag=<tag>") green on both runners. register runs the real codex with CODEX_HOME in a scratch folder.
same-behaviour makes one real image from the downloaded Windows asset (HORUS_COMFYUI_URL must name
port 8189).
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import sys
import time
from pathlib import Path

import goal_runner as r
from suites import common

TITLE = "release, install line, full setup and uninstall"
SPEC = "docs/specs/2026-11-15-release-install.md"
REAL_WHERE = "on the PC after the release"
BASELINE_COMMIT4 = "38c1838"  # main before the first task of Phase 4; the parity test and the golden files are compared with it
BASELINE_TESTS4 = 250  # Go test functions at BASELINE_COMMIT4 (go test -list . ./...)
CONTRACT_NAMES4 = r.TOOLS / "phase4_contract_checks.txt"  # the 30 contract checks at BASELINE_COMMIT4, one name per line
ERROR_CODES4 = 22  # horus errors -json after T5 (21 at the baseline)
REAL_PORT4 = "8189"  # a real image of Phase 4 uses this port only: the ComfyUI of the owner on 8188 gets no job
RELEASE_ASSETS = ["horus-windows-amd64.exe", "horus-darwin-arm64", "checksums.txt"]
RELEASE_FILES = ["release-please-config.json", ".release-please-manifest.json", ".github/workflows/release.yml"]
RELEASE_SETTINGS = ["bump-minor-pre-major", "bump-patch-for-minor-pre-major"]  # the two 0.x settings of release-please
RELEASE_JOBS = ["release-build"]
INSTALL_JOBS = [f"install {o}" for o in common.CI_OSES]
INSTALL_SCRIPTS = ["install.sh", "install.ps1"]
INSTALL_LINES = ["curl -fsSL https://raw.githubusercontent.com/hieplam/horus/main/install.sh | sh",
                 "irm https://raw.githubusercontent.com/hieplam/horus/main/install.ps1 | iex"]
FRESH_WORKFLOW = ".github/workflows/fresh-machine.yml"
# The Go tests that the rows of Phase 4 pin: (package folder, test name, the task that writes it).
# Tasks T1 to T7 write tests with exactly these names (the list is in the evidence log, section T0).
# A task may rename a pin only by changing this table in its own pull request.
PINNED4 = {
    "release": [
        ("cmd/horus", "TestVersionPrintsTheVersionOfTheLinkerFlag", "T1"),
        ("internal/mcpserver", "TestServerInfoVersionIsTheVersionOfTheBuild", "T1"),
    ],
    "binary-only": [
        ("cmd/horus", "TestWorkflowListWithoutAClonePrintsTheBuiltInWorkflows", "T2"),
        ("cmd/horus", "TestListWorkflowsOfTheMCPServerWithoutAClonePrintsTheBuiltInWorkflows", "T2"),
        ("internal/settings", "TestEnvFileOfTheUserFolderGivesASetting", "T2"),
        ("internal/settings", "TestOutputDirWithoutACloneIsTheOutputsFolderOfTheUserFolder", "T2"),
        ("internal/settings", "TestHorusWorkflowsDirReplacesTheBuiltInWorkflows", "T2"),
        ("internal/settings", "TestAFolderWithOnlyAWorkflowsFolderIsNotAClone", "T2"),
    ],
    "full-setup": [
        ("internal/provision", "TestPlanOfAnEmptyMachineListsTheStepsInTheOrderOfTheSpec", "T4"),
        ("internal/provision", "TestPlanWithoutAPackageManagerHasAStepForAPerson", "T4"),
        ("cmd/horus", "TestSetupYesWithoutWithPackagesRunsNoProgramAndExitsOne", "T4"),
        ("cmd/horus", "TestSetupYesWithPackagesRunsTheFakePackageManagerThenAPlanHasZeroSteps", "T4"),
        ("cmd/horus", "TestSetupYesWithoutAPackageManagerRunsNothingAndExitsOne", "T4"),
        ("internal/mcpserver", "TestInstallWorkflowWithoutAcceptSetupAnswersSetupConfirmationRequired", "T5"),
        ("internal/mcpserver", "TestInstallWorkflowWithAcceptSetupRunsTheSetupSteps", "T5"),
    ],
    "uninstall": [
        ("internal/provision", "TestUninstallPlanWithoutYesChangesNothing", "T7"),
        ("internal/provision", "TestUninstallYesRemovesEachRemoveItem", "T7"),
        ("internal/provision", "TestUninstallKeepsAFolderWithoutAnInstallRecord", "T7"),
        ("internal/provision", "TestUninstallKeepsTheEnvFileAndTheOutputsFolder", "T7"),
        ("internal/provision", "TestUninstallModelsLeavesComfyUIAndTheRegistration", "T7"),
        ("internal/provision", "TestUninstallMCPLeavesTheFiles", "T7"),
        ("internal/provision", "TestUninstallKeepsAComfyUIThatAnswersAndHorusDidNotStart", "T7"),
        ("internal/provision", "TestUninstallRunsMCPRemoveOfBothClients", "T7"),
        ("internal/backend", "TestInstallWritesTheInstallRecord", "T7"),
        ("internal/platform", "TestPathEntryOfTheInstallerIsRemoved", "T7"),
        ("internal/platform", "TestTheProgramFileRemovesItself", "T7"),
        ("cmd/horus", "TestSetupThenUninstallLeavesOnlyTheKeptItems", "T7"),
    ],
    "register": [
        ("internal/provision", "TestRegisterClaudeRunsMCPAddWithTheArgumentsOfTheSpec", "T6"),
        ("internal/provision", "TestRegisterClaudeSkipsOnASecondRun", "T6"),
        ("internal/provision", "TestRegisterCodexRunsMCPAddWithTheArgumentsOfTheSpec", "T6"),
        ("internal/provision", "TestRegisterCodexSkipsWhenMCPGetExitsZero", "T6"),
        ("internal/provision", "TestRegisterPassesNoHorusRootWithoutARepoRoot", "T6"),
        ("internal/cli", "TestRegisterAnotherClientNameExitsTwo", "T6"),
    ],
}
_RELEASE_CACHE: dict[str, tuple] = {}


def with_real4(a: argparse.Namespace, ok: bool, detail: str, real) -> tuple[bool, str]:
    return r.join_real(a, ok, detail, real, "on the PC after the release")


def find_key(node, key: str) -> list:
    """Every value of the key anywhere in a JSON tree."""
    found = []
    if isinstance(node, dict):
        for k, v in node.items():
            if k == key:
                found.append(v)
            found += find_key(v, key)
    elif isinstance(node, list):
        for v in node:
            found += find_key(v, key)
    return found


def release_config_problems(text: str) -> list[str]:
    """What stops release-please-config.json from keeping every version at 0.x (spec, "Version and release")."""
    try:
        cfg = json.loads(text)
    except ValueError:
        return ["release-please-config.json is not JSON"]
    problems = [f"release-please-config.json has no {k}: true" for k in RELEASE_SETTINGS if True not in find_key(cfg, k)]
    # A pinned version (release-as) makes release-please ignore the commits. After the first release it
    # proposes the version of the pin again, and the tag exists (issue #150). So the key must not be there.
    for pin in find_key(cfg, "release-as"):
        problems.append(f"release-please-config.json pins the version with release-as: {pin!r}; remove the key after the first release")
    return problems


def release_files_part() -> tuple[bool, str]:
    missing = r.file_problems(RELEASE_FILES)
    problems = [f"missing: {f}" for f in missing]
    if RELEASE_FILES[0] not in missing:
        problems += release_config_problems((r.ROOT / RELEASE_FILES[0]).read_text(encoding="utf-8", errors="replace"))
    return not problems, ("files: " + ", ".join(problems)) if problems else "release files ok"


def install_script_problems(name: str, text: str) -> list[str]:
    """A script that is empty or that never names the checksum file cannot check a SHA-256."""
    low = text.lower()
    return [f"{name} does not mention {w}" for w in ("sha256", "checksums.txt") if w not in low]


def install_files_part() -> tuple[bool, str]:
    problems = [f"missing: {f}" for f in r.file_problems(INSTALL_SCRIPTS)]
    for f in INSTALL_SCRIPTS:
        if (r.ROOT / f).is_file():
            problems += install_script_problems(f, (r.ROOT / f).read_text(encoding="utf-8", errors="replace"))
    return not problems, ("files: " + ", ".join(problems)) if problems else "install scripts ok"


def parse_checksums(text: str) -> dict[str, str]:
    """checksums.txt in the format of sha256sum: '<sha256>  <name>' or '<sha256> *<name>'."""
    out = {}
    for ln in text.splitlines():
        m = re.fullmatch(r"([0-9a-fA-F]{64}) [ *](\S+)", ln.strip())
        if m:
            out[m.group(2)] = m.group(1).lower()
    return out


def checksum_problems(folder: Path) -> list[str]:
    """The program files of the folder whose SHA-256 is not equal to their line of checksums.txt."""
    try:
        lines = parse_checksums((folder / "checksums.txt").read_text(encoding="utf-8"))
    except OSError:
        return ["checksums.txt is missing"]
    problems = []
    for name in RELEASE_ASSETS[:2]:
        if name not in lines:
            problems.append(f"{name} has no line in checksums.txt")
        elif not (folder / name).is_file():
            problems.append(f"{name} was not downloaded")
        elif r.sha256_of(folder / name) != lines[name]:
            problems.append(f"{name} has a SHA-256 that differs from checksums.txt")
    return problems


def release_assets(a: argparse.Namespace) -> tuple[Path | None, str, str]:
    """Download the 3 assets of the newest release into a scratch folder. Returns (folder, tag, why not)."""
    if "r" not in _RELEASE_CACHE:
        _RELEASE_CACHE["r"] = _release_assets(a)
    return _RELEASE_CACHE["r"]


def _release_assets(a: argparse.Namespace) -> tuple[Path | None, str, str]:
    if not shutil.which("gh"):
        return None, "", "gh not found"
    code, out = r.run(["gh", "release", "view", "--json", "tagName,assets"], timeout=120)
    try:
        info = json.loads(out) if code == 0 else None
    except ValueError:
        info = None
    if not info:
        return None, "", "no release"
    tag = info.get("tagName", "")
    names = sorted(x.get("name", "") for x in info.get("assets", []))
    if names != sorted(RELEASE_ASSETS):
        return None, tag, f"release {tag} has {len(names)} assets ({', '.join(names) or 'none'}), needs " + ", ".join(RELEASE_ASSETS)
    base = Path(a.real_out).resolve() if a.real_out else r.ROOT / ".scratch" / "release-assets"
    folder = base / tag
    folder.mkdir(parents=True, exist_ok=True)
    code, out = r.run(["gh", "release", "download", tag, "--dir", str(folder), "--clobber"], timeout=900)
    if code != 0:
        return None, tag, f"download of {tag} failed: {r.last_line(out)}"
    return folder, tag, ""


def real_release(a: argparse.Namespace) -> tuple[bool, str]:
    folder, tag, why = release_assets(a)
    if folder is None:
        return False, why
    bad = checksum_problems(folder)
    if bad:
        return False, f"release {tag}: " + ", ".join(bad)
    if r.this_os() != "win32":
        return False, f"release {tag}: 3 assets, SHA-256 ok; running the Windows asset needs Windows (this machine: {r.this_os()})"
    code, out = r.run([str(folder / RELEASE_ASSETS[0]), "version"], timeout=60)
    want = f"horus {tag.lstrip('v')}"
    if code != 0 or out.strip() != want:
        return False, f"{RELEASE_ASSETS[0]} version: exit {code}, printed '{r.last_line(out)}', needs '{want}'"
    return True, f"release {tag}: 3 assets, each SHA-256 equal to its line, '{want}'"


def row4_release(a: argparse.Namespace) -> tuple[bool, str]:
    parts = [release_files_part(), r.pinned_tests(PINNED4["release"]), r.ci_part(a, RELEASE_JOBS)]
    ok, detail = r.join_parts(parts)
    return with_real4(a, ok, detail, real_release)


def pick_fresh_run(runs: list[dict]) -> dict | None:
    """The newest run of fresh-machine.yml with the real release as its source. The run-name of the
    workflow is 'fresh-machine source=<build|release> tag=<tag>', and gh lists the newest run first."""
    for run in runs:
        if "source=release" in (run.get("displayTitle") or ""):
            return run
    return None


def judge_fresh(runs: list[dict], jobs_of, tag: str) -> tuple[bool, str]:
    """Pure judgement: the newest release run is for the newest tag, it is green, and both runners are green."""
    run = pick_fresh_run(runs)
    if run is None:
        return False, "no run of fresh-machine.yml with source=release"
    title = run.get("displayTitle") or ""
    if f"tag={tag}" not in title:
        return False, f"the newest release run '{title}' is not for the newest tag {tag}"
    if run.get("conclusion") != "success":
        return False, f"the run '{title}' is {run.get('conclusion') or run.get('status') or 'unknown'}"
    jobs = jobs_of(run["databaseId"])
    bad = []
    for os_name in common.CI_OSES:
        hit = [j for j in jobs if os_name in j.get("name", "")]
        if not hit or any(j.get("conclusion") != "success" for j in hit):
            bad.append(os_name)
    if bad:
        return False, f"the run '{title}' has no green job on " + ", ".join(bad)
    return True, f"fresh-machine run '{title}' is green on " + " and ".join(common.CI_OSES)


def real_fresh(a: argparse.Namespace) -> tuple[bool, str]:
    if not shutil.which("gh"):
        return False, "gh not found"
    code, out = r.run(["gh", "release", "view", "--json", "tagName"], timeout=120)
    try:
        tag = json.loads(out)["tagName"] if code == 0 else ""
    except (ValueError, KeyError, TypeError):
        tag = ""
    if not tag:
        return False, "no release"
    code, out = r.run(["gh", "run", "list", "--workflow", "fresh-machine.yml", "--limit", "30",
                       "--json", "databaseId,displayTitle,conclusion,status"], timeout=120)
    try:
        runs = json.loads(out) if code == 0 else []
    except ValueError:
        runs = []

    def jobs_of(run_id: int) -> list[dict]:
        c, o = r.run(["gh", "run", "view", str(run_id), "--json", "jobs"], timeout=120)
        try:
            return json.loads(o).get("jobs", []) if c == 0 else []
        except ValueError:
            return []
    return judge_fresh(runs, jobs_of, tag)


def row4_install_line(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail = r.join_parts([install_files_part(), r.ci_part(a, INSTALL_JOBS)])
    return with_real4(a, ok, detail, real_fresh)


def row4_binary_only(a: argparse.Namespace) -> tuple[bool, str]:
    return r.pinned_tests(PINNED4["binary-only"])


def error_codes_part(a: argparse.Namespace) -> tuple[bool, str]:
    """horus errors -json lists ERROR_CODES4 codes (the new code SETUP_CONFIRMATION_REQUIRED, task T5)."""
    exe = common.horus_exe(a)
    if not shutil.which(exe) and not Path(exe).is_file():
        return False, f"horus errors -json: program not found: {exe}"
    code, out = r.run([exe, "errors", "-json"], timeout=60)
    try:
        n = len(json.loads(out)) if code == 0 else -1
    except ValueError:
        n = -1
    if n < 0:
        return False, f"horus errors -json: exit {code}, no JSON list ({r.last_line(out)})"
    return n == ERROR_CODES4, f"horus errors -json lists {n} codes" + ("" if n == ERROR_CODES4 else f", needs {ERROR_CODES4}")


def row4_full_setup(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail = r.join_parts([r.pinned_tests(PINNED4["full-setup"]), error_codes_part(a)])
    return with_real4(a, ok, detail, real_fresh)


def row4_uninstall(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail = r.pinned_tests(PINNED4["uninstall"])
    return with_real4(a, ok, detail, real_fresh)


def real_register(a: argparse.Namespace) -> tuple[bool, str]:
    """horus register -client codex with CODEX_HOME in a scratch folder, then codex mcp get horus.
    The real config of the owner is never touched."""
    codex = shutil.which("codex")
    if not codex:
        return False, "codex not found"
    exe = common.horus_exe(a)
    scratch = r.ROOT / ".scratch" / f"codex-home-{time.strftime('%Y%m%d-%H%M%S')}"
    scratch.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, CODEX_HOME=str(scratch))
    code, out = r.run([exe, "register", "-client", "codex"], env=env, timeout=120)
    if code != 0:
        return False, f"horus register -client codex: exit {code}: {r.last_line(out)}"
    code, out = r.run([codex, "mcp", "get", "horus"], env=env, timeout=120)
    if code != 0:
        return False, f"codex mcp get horus: exit {code}: {r.last_line(out)}"
    return True, f"horus register -client codex, then codex mcp get horus exits 0 (CODEX_HOME {scratch})"


def row4_register(a: argparse.Namespace) -> tuple[bool, str]:
    ok, detail = r.pinned_tests(PINNED4["register"])
    return with_real4(a, ok, detail, real_register)


def readme_sections(text: str) -> list[str]:
    """The text of each '## ' section of the README, in order. The text before the first one is no section."""
    return re.split(r"(?m)^## ", text)[1:]


def fence_commands(text: str) -> list[str]:
    """The command lines of the shell code blocks of a text: blocks with no language, sh, bash, shell or
    powershell. A comment line, a blank line and a block of another language (mermaid) are no command."""
    cmds, lang, inside = [], "", False
    for ln in text.splitlines():
        m = re.match(r"^```\s*(\w*)", ln)
        if m:
            inside = not inside
            lang = m.group(1).lower() if inside else ""
            continue
        if not inside or lang not in ("", "sh", "bash", "shell", "powershell", "pwsh", "console"):
            continue
        cmd = re.sub(r"^(\$|PS>|>)\s+", "", ln.strip())
        if cmd and not cmd.startswith("#"):
            cmds.append(cmd)
    return cmds


def squash(text: str) -> str:
    return re.sub(r"\s+", " ", text)


def readme_problems(readme: str, fresh: str | None) -> list[str]:
    """Both install lines are in section 1 or 2 of the README, and each command line of those sections is
    in fresh-machine.yml (None when the file is missing)."""
    first = "\n".join(readme_sections(readme)[:2])
    problems = [f"the install line of {ln.rsplit('/', 1)[-1].split()[0]} is not in section 1 or 2"
                for ln in INSTALL_LINES if squash(ln) not in squash(first)]
    if fresh is None:
        problems.append(f"missing: {FRESH_WORKFLOW}")
    else:
        body = squash(fresh)
        problems += [f"the README command '{c[:60]}' is not in fresh-machine.yml"
                     for c in fence_commands(first) if squash(c) not in body]
    return problems


def row4_readme(a: argparse.Namespace) -> tuple[bool, str]:
    try:
        readme = (r.ROOT / "README.md").read_text(encoding="utf-8")
    except OSError:
        return False, "missing: README.md"
    try:
        fresh = (r.ROOT / FRESH_WORKFLOW).read_text(encoding="utf-8")
    except OSError:
        fresh = None
    problems = readme_problems(readme, fresh)
    if problems:
        return False, f"{len(problems)} problems: " + "; ".join(problems)
    return True, "both install lines are in section 1 or 2; each README command is in fresh-machine.yml"


def contract_names_part(a: argparse.Namespace) -> tuple[bool, str]:
    """The contract test passes with at least CONTRACT_CHECKS checks, and each of the 30 baseline checks is in
    its output with PASS. The count may grow; the names may not go away."""
    cmd = a.server_cmd or ["horus", "mcp"]
    if common.server_missing(cmd):
        return False, f"contract: server command not found: {cmd[0]}"
    try:
        names = [ln.strip() for ln in CONTRACT_NAMES4.read_text(encoding="utf-8").splitlines() if ln.strip()]
    except OSError:
        return False, f"missing: {CONTRACT_NAMES4.relative_to(r.ROOT).as_posix()}"
    code, out = r.run([sys.executable, "tests/mcp_contract.py", "--server-cmd", *cmd], timeout=600)
    m = re.search(r"contract: (\d+) of (\d+) passed", out)
    if not m:
        return False, f"contract: no result line ({r.last_line(out)})"
    passed = {ln[len("PASS  "):].strip() for ln in out.splitlines() if ln.startswith("PASS  ")}
    lacking = [n for n in names if n not in passed]
    checks = common.CONTRACT_CHECKS
    ok = code == 0 and m.group(1) == m.group(2) and int(m.group(2)) >= max(len(names), checks) and not lacking
    detail = f"contract: {m.group(1)} of {m.group(2)}, {len(names) - len(lacking)} of {len(names)} baseline checks pass"
    if int(m.group(2)) < checks:
        detail += f", needs {checks} or more checks"
    if lacking:
        detail += ", missing or failed: " + ", ".join(lacking[:3]) + (" ..." if len(lacking) > 3 else "")
    return ok, detail


def frozen_files4() -> list[str] | str:
    """Files of the parity test (and its fixtures and golden files) that differ from the baseline.
    tests/mcp_contract.py is left out: task T5 adds checks there, and contract_names_part guards the old ones."""
    return common.changed_files(BASELINE_COMMIT4, ["tests/workflow_parity.py", "tests/fixtures", "*golden*"])


def real_same4(a: argparse.Namespace) -> tuple[bool, str]:
    """One real image through MCP from the downloaded Windows asset, on port 8189 only."""
    if r.this_os() != "win32":
        return False, f"the real image needs the PC (this machine: {r.this_os()})"
    url = common.comfy_url()
    if not url.endswith(":" + REAL_PORT4):
        return False, f"HORUS_COMFYUI_URL is {url}; set it to a ComfyUI on port {REAL_PORT4} (the ComfyUI of the owner on 8188 gets no job)"
    folder, tag, why = release_assets(a)
    if folder is None:
        return False, why
    bad = checksum_problems(folder)
    if bad:
        return False, f"release {tag}: " + ", ".join(bad)
    img = argparse.Namespace(**{**vars(a), "server_cmd": [str(folder / RELEASE_ASSETS[0]), "mcp"]})
    ok, detail, _ = common.real_image(img)
    return ok, f"{detail} (program of release {tag})" if ok else detail


def row4_same(a: argparse.Namespace) -> tuple[bool, str]:
    parts = [contract_names_part(a), common.parity_part(), common.go_test_all_part(), common.archtest_part(),
             common.test_count_part(BASELINE_TESTS4),
             common.changed_part(frozen_files4(), "parity or golden files"),
             r.ci_part(a, common.SAME_BEHAVIOUR_JOBS)]
    ok, detail = r.join_parts(parts)
    return with_real4(a, ok, detail, real_same4)


ROWS = ["release", "install-line", "binary-only", "full-setup", "uninstall", "register", "readme", "same-behaviour"]
CHECKS = {"release": row4_release, "install-line": row4_install_line, "binary-only": row4_binary_only,
          "full-setup": row4_full_setup, "uninstall": row4_uninstall, "register": row4_register,
          "readme": row4_readme, "same-behaviour": row4_same}

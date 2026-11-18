#!/usr/bin/env python3
"""Refuse a change to a frozen goal suite.

A goal suite is the folder goals/<spec stem>/. It is frozen when the spec
docs/specs/<spec stem>.md has the block "Record of a finished phase"
(AGENTS.md, section "Goal suites"). A change to a finished feature gets a new
spec and a new suite.

    python scripts/frozen-suites.py --hook            the Claude Code hook
    python scripts/frozen-suites.py --base <commit>   the CI step

--hook   .claude/settings.json runs this before each Write or Edit. Claude Code
         gives the tool call as JSON on standard input. When the file is in a
         suite whose spec has the block in the working tree, this prints the
         reason on standard error and exits 2, and Claude Code refuses the call.
--base   Checks each file of goals/ that differs between the commit and HEAD.
         It reads the spec on the commit, not in the working tree, so the pull
         request that finishes a spec can still change its suite. It prints one
         line for each finding and exits 1 when there is a finding.
--root   The repo root. Default: the folder with go.mod above the file of the
         tool call (--hook), or the repo of this script (--base).
"""
from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
MARK = "> **Record of a finished phase.**"
NOT_SUITES = {"testdata"}  # folders of goals/ that are no suite


def suite_of(rel: str) -> str | None:
    """The spec stem of a file path from the repo root, or None when the file is in no suite."""
    parts = rel.replace("\\", "/").split("/")
    if len(parts) >= 3 and parts[0] == "goals" and parts[1] not in NOT_SUITES:
        return parts[1]
    return None


def finished(spec_text: str) -> bool:
    return any(line.startswith(MARK) for line in spec_text.splitlines())


def repo_root_of(path: Path) -> Path | None:
    """The first folder above the path that has a go.mod file."""
    for folder in path.parents:
        if (folder / "go.mod").is_file():
            return folder
    return None


def hook(root: Path | None) -> int:
    try:
        call = json.load(sys.stdin)
    except ValueError:
        return 0
    name = (call.get("tool_input") or {}).get("file_path") or ""
    if not name:
        return 0
    path = Path(name)
    if not path.is_absolute():
        path = Path(call.get("cwd") or ROOT) / path
    path = path.resolve()
    root = root.resolve() if root else repo_root_of(path)
    if root is None:
        return 0
    try:
        rel = path.relative_to(root).as_posix()
    except ValueError:
        return 0
    stem = suite_of(rel)
    if stem is None:
        return 0
    spec = root / "docs" / "specs" / f"{stem}.md"
    if not spec.is_file() or not finished(spec.read_text(encoding="utf-8", errors="replace")):
        return 0
    sys.stderr.write(
        f"Refused: {rel} is a file of a frozen goal suite. The spec docs/specs/{stem}.md has the block "
        '"Record of a finished phase", so its suite is a record and nobody edits it. '
        "For a change to this feature, write a new spec and a new suite (AGENTS.md, section \"Goal suites\").\n"
    )
    return 2


def git(root: Path, *args: str) -> subprocess.CompletedProcess:
    return subprocess.run(["git", "-C", str(root), *args], capture_output=True, text=True, encoding="utf-8", errors="replace")


def base(root: Path, commit: str) -> int:
    diff = git(root, "diff", "--name-only", "--no-renames", commit, "HEAD", "--", "goals")
    if diff.returncode != 0:
        print(f"frozen-suites: git diff against {commit} failed: {diff.stderr.strip()}")
        return 1
    findings = []
    for rel in diff.stdout.splitlines():
        stem = suite_of(rel)
        if stem is None:
            continue
        spec = git(root, "show", f"{commit}:docs/specs/{stem}.md")
        if spec.returncode == 0 and finished(spec.stdout):
            findings.append(f"{rel}: frozen-suite: docs/specs/{stem}.md is finished on {commit}, so nobody edits its suite")
    for f in findings:
        print(f)
    print(f"frozen-suites: {len(findings)} finding(s)")
    return 0 if not findings else 1


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    mode = ap.add_mutually_exclusive_group(required=True)
    mode.add_argument("--hook", action="store_true", help="the Claude Code hook: read the tool call from standard input")
    mode.add_argument("--base", metavar="COMMIT", help="the CI step: check the files that differ between COMMIT and HEAD")
    ap.add_argument("--root", type=Path, help="the repo root")
    args = ap.parse_args()
    if args.hook:
        return hook(args.root)
    return base(args.root or ROOT, args.base)


if __name__ == "__main__":
    sys.exit(main())

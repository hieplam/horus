#!/usr/bin/env python3
"""Check that the repo is safe to publish.

Usage: python scripts/lint-public.py [--extra-words FILE]
       python scripts/lint-public.py --adr      (only the decision records, fast)

Checks every tracked or untracked (not ignored) text file for:
  - absolute user paths (C:\\Users\\<name>, /Users/<name>, /home/<name>)
  - email addresses
  - access tokens (Hugging Face hf_..., GitHub ghp_/gho_/github_pat_, Civitai keys in URLs)
  - relative Markdown links that point to a missing file
  - words from --extra-words (one per line, case-insensitive). Keep that file
    OUTSIDE the repo: it lists the private words you do not want to publish.

It also checks the decision records in docs/adr (docs/adr/README.md):
  - each file NNNN-<name>.md has a number that no other file has
  - each record has a Status line, a Date line and the sections Context,
    Decision and Consequences
  - the index in docs/adr/README.md links each record

Prints one line per finding and a total. Exits 0 when there are no findings.
"""
from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
TEXT_EXT = {".md", ".ps1", ".py", ".json", ".yaml", ".yml", ".txt", ".sh", ".example", ".go", ".mod", ".toml", ""}

# Placeholders that docs may show on purpose.
ALLOWED_USERS = {"<you>", "<name>", "<user>", "you", "public", "default"}

PATTERNS = [
    ("user-path", re.compile(r"[A-Za-z]:\\+Users\\+([^\\\s\"'`<>]+)", re.I)),
    ("user-path", re.compile(r"/(?:Users|home)/([A-Za-z0-9._-]+)")),
    ("email", re.compile(r"[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}")),
    ("token", re.compile(r"\bhf_[A-Za-z0-9]{20,}")),
    ("token", re.compile(r"\b(?:ghp|gho|ghs)_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}")),
    ("token", re.compile(r"[?&]token=[A-Za-z0-9]{16,}")),
]
ALLOWED_EMAILS = {"noreply@anthropic.com"}
LINK = re.compile(r"\]\(([^)\s#]+)(?:#[^)]*)?\)")


def repo_files() -> list[Path]:
    out = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard"],
        cwd=ROOT, capture_output=True, text=True, check=True,
    ).stdout.splitlines()
    return [ROOT / p for p in out if p]


ADR_NAME = re.compile(r"^(\d{4})-[a-z0-9-]+\.md$")
ADR_PARTS = ["- **Status:**", "- **Date:**", "## Context", "## Decision", "## Consequences"]


def adr_findings(adr: Path) -> list[str]:
    """The findings of the decision records in the folder adr."""
    rel = adr.relative_to(ROOT).as_posix()
    index = adr / "README.md"
    if not index.is_file():
        return [f"{rel}/README.md: adr: the index of the decision records is missing"]
    index_text = index.read_text(encoding="utf-8")
    findings: list[str] = []
    seen: dict[str, str] = {}
    for path in sorted(adr.glob("*.md")):
        if path.name == "README.md":
            continue
        m = ADR_NAME.match(path.name)
        if not m:
            findings.append(f"{rel}/{path.name}: adr: the name is not NNNN-<name>.md")
            continue
        number = m.group(1)
        if number in seen:
            findings.append(f"{rel}/{path.name}: adr: number {number} is also the number of {seen[number]}")
        seen[number] = path.name
        text = path.read_text(encoding="utf-8")
        for part in ADR_PARTS:
            if not any(line.startswith(part) for line in text.splitlines()):
                findings.append(f"{rel}/{path.name}: adr: no line starts with '{part}'")
        if number != "0000" and f"]({path.name})" not in index_text:
            findings.append(f"{rel}/README.md: adr: the index has no link to {path.name}")
    return findings


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--extra-words", type=Path)
    ap.add_argument("--adr", action="store_true", help="check only the decision records in docs/adr")
    args = ap.parse_args()
    if args.adr:
        findings = adr_findings(ROOT / "docs" / "adr")
        for f in findings:
            print(f)
        print(f"lint-public --adr: {len(findings)} finding(s)")
        return 0 if not findings else 1
    words: list[str] = []
    if args.extra_words:
        words = [w.strip().lower() for w in args.extra_words.read_text(encoding="utf-8").splitlines() if w.strip()]

    findings: list[str] = []
    for path in repo_files():
        if not path.is_file():
            continue
        if path.suffix.lower() not in TEXT_EXT and path.name not in {".gitignore", ".env.example"}:
            continue
        rel = path.relative_to(ROOT).as_posix()
        if rel == "scripts/lint-public.py":
            continue
        text = path.read_text(encoding="utf-8", errors="replace")
        for n, line in enumerate(text.splitlines(), 1):
            for kind, rx in PATTERNS:
                for m in rx.finditer(line):
                    if kind == "user-path" and m.group(1).strip("$%{}").lower() in ALLOWED_USERS | {"userprofile", "env:userprofile"}:
                        continue
                    if kind == "user-path" and m.group(1).startswith(("$", "%", "<", "{")):
                        continue
                    if kind == "email" and m.group(0).lower() in ALLOWED_EMAILS:
                        continue
                    findings.append(f"{rel}:{n}: {kind}: {m.group(0)}")
            low = line.lower()
            for w in words:
                if w in low:
                    findings.append(f"{rel}:{n}: private-word: {w}")
            if path.suffix.lower() == ".md":
                for m in LINK.finditer(line):
                    target = m.group(1)
                    if re.match(r"^[a-z]+:", target):
                        continue
                    if not (path.parent / target).exists():
                        findings.append(f"{rel}:{n}: broken-link: {target}")

    findings.extend(adr_findings(ROOT / "docs" / "adr"))

    for f in findings:
        print(f)
    print(f"lint-public: {len(findings)} finding(s)")
    return 0 if not findings else 1


if __name__ == "__main__":
    sys.exit(main())

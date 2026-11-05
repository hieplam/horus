#!/usr/bin/env python3
"""Check that the repo is safe to publish.

Usage: python scripts/lint-public.py [--extra-words FILE]

Checks every tracked or untracked (not ignored) text file for:
  - absolute user paths (C:\\Users\\<name>, /Users/<name>, /home/<name>)
  - email addresses
  - access tokens (Hugging Face hf_..., GitHub ghp_/gho_/github_pat_, Civitai keys in URLs)
  - relative Markdown links that point to a missing file
  - words from --extra-words (one per line, case-insensitive). Keep that file
    OUTSIDE the repo: it lists the private words you do not want to publish.

Prints one line per finding and a total. Exits 0 when there are no findings.
"""
from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
TEXT_EXT = {".md", ".ps1", ".py", ".json", ".yaml", ".yml", ".txt", ".sh", ".example", ""}

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


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--extra-words", type=Path)
    args = ap.parse_args()
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

    for f in findings:
        print(f)
    print(f"lint-public: {len(findings)} finding(s)")
    return 0 if not findings else 1


if __name__ == "__main__":
    sys.exit(main())
